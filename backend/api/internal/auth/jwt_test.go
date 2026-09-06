package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

// The 401 body used to carry golang-jwt's own wording. Every rejection now
// answers with the same fixed phrase, so the response cannot drift with a
// dependency bump and cannot separate "expired" from "bad signature"
// (KAN-54).
func TestMiddlewareDoesNotLeakParseErrorDetail(t *testing.T) {
	key, srv := newSigningKey(t)
	jwks := newJWKS(t, srv)
	otherKey, _ := newSigningKey(t)

	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()

	// One case per shape of parse failure the library words differently.
	tests := []struct {
		name, authHeader string
	}{
		{name: "not a JWT at all", authHeader: "Bearer garbage"},
		{name: "expired", authHeader: "Bearer " + sign(t, key, expired)},
		{name: "signed by an unknown key", authHeader: "Bearer " + sign(t, otherKey, validClaims())},
	}

	// Fragments of the library's phrasing for exactly those three.
	leaks := []string{
		"invalid number of segments",
		"token is expired",
		"signature is invalid",
		"crypto/",
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
			if body.Error.Message != "invalid token" {
				t.Errorf("message = %q, want exactly %q", body.Error.Message, "invalid token")
			}
			for _, leak := range leaks {
				if strings.Contains(body.Error.Message, leak) {
					t.Errorf("message = %q, want it free of the library's wording %q", body.Error.Message, leak)
				}
			}
		})
	}
}

// The detail is not discarded, only moved: without it in the log there is no
// way to tell an expired token from a forged one when a user reports a 401.
func TestMiddlewareLogsWhyTheTokenWasRejected(t *testing.T) {
	_, srv := newSigningKey(t)
	jwks := newJWKS(t, srv)

	var buf bytes.Buffer
	saved, savedLevel := log.Logger, zerolog.GlobalLevel()
	log.Logger = zerolog.New(&buf)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() {
		log.Logger = saved
		zerolog.SetGlobalLevel(savedLevel)
	})

	serve(t, jwks, "Bearer garbage")

	logged := buf.String()
	if !strings.Contains(logged, "invalid number of segments") {
		t.Errorf("log = %q, want it to carry the parse error the body no longer does", logged)
	}
	// Empty here because no logging.Middleware ran in front of this test;
	// what matters is that the field is emitted, so the real chain has
	// somewhere to put the id that ties this line to the request.
	if !strings.Contains(logged, `"trace_id"`) {
		t.Errorf("log = %q, want a trace_id field to tie it back to the request", logged)
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

// A wrong SUPABASE_URL has to stop the process. cmd/api/main.go calls
// log.Fatal on this error, which only means "refuse to start without a
// working JWKS" if the error actually arrives -- it used to be swallowed for
// every failure except an unparseable URL, so the server started, mounted
// /api/v1, and 401d every request (KAN-53).
func TestNewJWKSReportsAnUnusableEndpoint(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer failing.Close()

	// A 200 with a well-formed but empty key set: not an HTTP failure, so
	// only reading the keys back catches it.
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer empty.Close()

	tests := []struct {
		name, url string
	}{
		{name: "endpoint returns 500", url: failing.URL},
		{name: "connection refused", url: "http://127.0.0.1:1"},
		{name: "host does not resolve", url: "http://not-a-real-host.invalid"},
		{name: "endpoint answers with no keys", url: empty.URL},
		{name: "malformed url", url: "://malformed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJWKS(context.Background(), tt.url)
			if err == nil {
				t.Fatal("err = nil, want startup to fail on an unusable JWKS endpoint")
			}
			// The operator has to be able to tell which value is wrong.
			if !strings.Contains(err.Error(), tt.url) {
				t.Errorf("err = %v, want it to name the URL it tried (%q)", err, tt.url)
			}
		})
	}
}

func TestNewJWKSEmptyKeySetIsDistinguishable(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer empty.Close()

	// "reachable but empty" is a different fix from "unreachable", so it is
	// worth a sentinel rather than only a message.
	if _, err := NewJWKS(context.Background(), empty.URL); !errors.Is(err, ErrNoJWKSKeys) {
		t.Errorf("err = %v, want it to wrap ErrNoJWKSKeys", err)
	}
}
