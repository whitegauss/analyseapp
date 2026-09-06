// Package auth verifies Supabase-issued JWTs on incoming requests.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/response"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

// UserID returns the authenticated user's id, as extracted from the JWT's
// `sub` claim by Middleware. Only valid on requests that passed Middleware.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(userIDKey).(uuid.UUID)
	return v, ok
}

// WithUserID returns a context carrying userID, as if it had been set by
// Middleware. Exposed for handler tests that want to skip real JWT
// verification.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// ErrNoJWKSKeys is returned by NewJWKS when the endpoint answers but carries
// no keys. Nothing would fail at startup in that case and every authenticated
// request would 401, which reads as an auth bug rather than a bad
// SUPABASE_URL.
var ErrNoJWKSKeys = errors.New("no keys in the JWK Set")

// NewJWKS fetches and keeps in sync the Supabase project's JSON Web Key Set,
// used to verify the asymmetric (ES256) JWTs Supabase Auth issues by
// default. Call once at startup; the returned Keyfunc auto-refreshes keys.
//
// It reports a failure to reach or read the key set, so that a wrong
// SUPABASE_URL stops the process instead of producing a server that starts,
// mounts /api/v1, and 401s every request (KAN-53). Two things are needed for
// that, because keyfunc is built to survive a JWKS endpoint being down at any
// point *after* startup:
//
//   - NoErrorReturnFirstHTTPReq is overridden to false. It defaults to true,
//     which is what swallowed the connection-refused / DNS / 500 cases: the
//     first fetch's error was logged and a Keyfunc returned anyway.
//   - The key set is read back. A 200 carrying zero keys is not an HTTP
//     error, so the override alone would still let an empty JWKS through.
func NewJWKS(ctx context.Context, supabaseURL string) (keyfunc.Keyfunc, error) {
	jwksURL := strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"

	reportFirstFetch := false
	k, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, keyfunc.Override{
		NoErrorReturnFirstHTTPReq: &reportFirstFetch,
	})
	if err != nil {
		return nil, wrapJWKSErr(supabaseURL, err)
	}

	keys, err := k.Storage().KeyReadAll(ctx)
	if err != nil {
		return nil, wrapJWKSErr(supabaseURL, err)
	}
	if len(keys) == 0 {
		return nil, wrapJWKSErr(supabaseURL, ErrNoJWKSKeys)
	}
	return k, nil
}

// Middleware verifies the Authorization: Bearer <token> header against the
// Supabase project's JWKS.
func Middleware(jwks keyfunc.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, err := parseBearer(r.Header.Get("Authorization"))
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}

			claims := jwt.MapClaims{}
			if _, err := jwt.ParseWithClaims(tokenString, claims, jwks.Keyfunc); err != nil {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid token: "+err.Error())
				return
			}

			userID, err := userIDFromClaims(claims)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
		})
	}
}

// Rejection reasons. These are written verbatim into the 401 response body, so
// they must stay free of anything derived from the token itself.
var (
	errMissingBearer = errors.New("missing bearer token")
	errNotUserToken  = errors.New("token is not an authenticated-user token")
	errInvalidSub    = errors.New("invalid sub claim")
)

// parseBearer extracts the token from an Authorization header value. A header
// that is absent, uses another scheme, or carries an empty token is rejected
// the same way, so a caller cannot probe which of the three it hit.
func parseBearer(authHeader string) (string, error) {
	token, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || token == "" {
		return "", errMissingBearer
	}
	return token, nil
}

// userIDFromClaims applies the authorization rules to already-verified claims:
// the token must belong to a signed-in end user (not the anon or service-role
// key), and its subject must be a real user id.
func userIDFromClaims(claims jwt.MapClaims) (uuid.UUID, error) {
	role, _ := claims["role"].(string)
	if role != "authenticated" {
		return uuid.Nil, errNotUserToken
	}

	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, errInvalidSub
	}
	return userID, nil
}

func wrapJWKSErr(supabaseURL string, err error) error {
	return fmt.Errorf("fetch JWKS from %s: %w", supabaseURL, err)
}
