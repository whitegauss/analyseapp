package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/response"
)

const (
	testKID  = "test-key"
	testUser = "6f1d2c3b-4a59-4e8f-9b0d-1e2f3a4b5c6d"
)

// newSigningKey returns an ES256 key plus a Supabase-shaped JWKS endpoint that
// publishes it. Supabase signs with ES256 by default, so this mirrors what the
// middleware meets in production rather than the symmetric HS256 shortcut.
func newSigningKey(t *testing.T) (*ecdsa.PrivateKey, *httptest.Server) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	b64 := base64.RawURLEncoding
	jwks, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "EC",
		"crv": "P-256",
		"kid": testKID,
		"alg": "ES256",
		"use": "sig",
		"x":   b64.EncodeToString(key.X.FillBytes(make([]byte, 32))),
		"y":   b64.EncodeToString(key.Y.FillBytes(make([]byte, 32))),
	}}})
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	t.Cleanup(srv.Close)

	return key, srv
}

func sign(t *testing.T, key *ecdsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = testKID
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"role": "authenticated",
		"sub":  testUser,
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
}

// serve runs the middleware over a handler that records whether it ran and what
// user id it saw.
func serve(t *testing.T, jwks keyfunc.Keyfunc, authHeader string) (*httptest.ResponseRecorder, bool, uuid.UUID) {
	t.Helper()

	var reached bool
	var gotUser uuid.UUID
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		gotUser, _ = UserID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	Middleware(jwks)(next).ServeHTTP(rec, req)

	return rec, reached, gotUser
}

func newJWKS(t *testing.T, srv *httptest.Server) keyfunc.Keyfunc {
	t.Helper()

	jwks, err := NewJWKS(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("NewJWKS: %v", err)
	}
	return jwks
}

func TestMiddlewareAcceptsValidToken(t *testing.T) {
	key, srv := newSigningKey(t)
	jwks := newJWKS(t, srv)

	rec, reached, gotUser := serve(t, jwks, "Bearer "+sign(t, key, validClaims()))

	if !reached {
		t.Fatalf("next handler not reached; status = %d, body = %s", rec.Code, rec.Body)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if gotUser.String() != testUser {
		t.Errorf("UserID = %v, want %v", gotUser, testUser)
	}
}

func TestMiddlewareRejects(t *testing.T) {
	key, srv := newSigningKey(t)
	jwks := newJWKS(t, srv)

	otherKey, _ := newSigningKey(t)

	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()

	futureIssued := validClaims()
	futureIssued["nbf"] = time.Now().Add(time.Hour).Unix()

	anon := validClaims()
	anon["role"] = "anon"

	serviceRole := validClaims()
	serviceRole["role"] = "service_role"

	badSub := validClaims()
	badSub["sub"] = "not-a-uuid"

	noSub := jwt.MapClaims{"role": "authenticated", "exp": time.Now().Add(time.Hour).Unix()}

	tests := []struct {
		name        string
		authHeader  string
		wantMessage string
	}{
		{name: "no Authorization header", authHeader: "", wantMessage: "missing bearer token"},
		{name: "another scheme", authHeader: "Basic YWxhZGRpbjpvcGVuc2VzYW1l", wantMessage: "missing bearer token"},
		{name: "empty token", authHeader: "Bearer ", wantMessage: "missing bearer token"},
		{name: "not a JWT at all", authHeader: "Bearer garbage", wantMessage: "invalid token"},
		{name: "signed by an unknown key", authHeader: "Bearer " + sign(t, otherKey, validClaims()), wantMessage: "invalid token"},
		{name: "expired", authHeader: "Bearer " + sign(t, key, expired), wantMessage: "invalid token"},
		{name: "not yet valid", authHeader: "Bearer " + sign(t, key, futureIssued), wantMessage: "invalid token"},
		{name: "anon key", authHeader: "Bearer " + sign(t, key, anon), wantMessage: "not an authenticated-user token"},
		{name: "service_role key", authHeader: "Bearer " + sign(t, key, serviceRole), wantMessage: "not an authenticated-user token"},
		{name: "sub is not a uuid", authHeader: "Bearer " + sign(t, key, badSub), wantMessage: "invalid sub claim"},
		{name: "sub is absent", authHeader: "Bearer " + sign(t, key, noSub), wantMessage: "invalid sub claim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, reached, _ := serve(t, jwks, tt.authHeader)

			if reached {
				t.Error("next handler ran for a rejected request")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}

			var body response.Envelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Error == nil {
				t.Fatalf("Error = nil, want an error envelope; body = %s", rec.Body)
			}
			if body.Error.Code != "unauthorized" {
				t.Errorf("code = %q, want unauthorized", body.Error.Code)
			}
			if !strings.Contains(body.Error.Message, tt.wantMessage) {
				t.Errorf("message = %q, want it to contain %q", body.Error.Message, tt.wantMessage)
			}
		})
	}
}

// The middleware forwards the jwt library's parse error verbatim to the client.
// That is more than a caller needs to know; this test pins today's behaviour so
// that tightening it later is a deliberate, visible change rather than a
// silent one. Tightening it is KAN-54.
func TestMiddlewareLeaksParseErrorDetail(t *testing.T) {
	_, srv := newSigningKey(t)
	jwks := newJWKS(t, srv)

	rec, _, _ := serve(t, jwks, "Bearer garbage")

	var body response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !strings.Contains(body.Error.Message, "token contains an invalid number of segments") {
		t.Errorf("message = %q, want the underlying parse error to be echoed", body.Error.Message)
	}
}

func TestNewJWKSTrailingSlash(t *testing.T) {
	_, srv := newSigningKey(t)

	// The JWKS path is appended to SUPABASE_URL; both spellings of the base URL
	// have to land on the same endpoint.
	for _, base := range []string{srv.URL, srv.URL + "/"} {
		if _, err := NewJWKS(t.Context(), base); err != nil {
			t.Errorf("NewJWKS(%q) = %v, want nil", base, err)
		}
	}
}

// NewJWKS only reports an error for a URL it cannot parse. An endpoint that is
// refused, unresolvable, or answering 500 is NOT reported, because keyfunc logs
// the failure and hands back a Keyfunc that fails per-request instead.
//
// That matters: cmd/api/main.go:41-43 calls log.Fatal on this error, so it reads
// as "refuse to start without a working JWKS" — but with a wrong SUPABASE_URL
// the server starts, mounts /api/v1, and 401s every request. Pinned here as
// today's behaviour; fixing the startup check is KAN-53.
func TestNewJWKSReportsOnlyMalformedURLs(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer failing.Close()

	unreported := []struct {
		name string
		url  string
	}{
		{name: "endpoint returns 500", url: failing.URL},
		{name: "connection refused", url: "http://127.0.0.1:1"},
		{name: "host does not resolve", url: "http://not-a-real-host.invalid"},
	}
	for _, tt := range unreported {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewJWKS(context.Background(), tt.url); err != nil {
				t.Errorf("err = %v, want nil (startup does not detect this today)", err)
			}
		})
	}

	t.Run("malformed url", func(t *testing.T) {
		_, err := NewJWKS(context.Background(), "://malformed")
		if err == nil {
			t.Fatal("err = nil, want an error for an unparseable URL")
		}
		if !strings.Contains(err.Error(), "://malformed") {
			t.Errorf("err = %v, want it to name the URL it tried", err)
		}
	})
}
