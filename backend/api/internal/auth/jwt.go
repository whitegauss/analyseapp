// Package auth verifies Supabase-issued JWTs on incoming requests.
package auth

import (
	"context"
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

// NewJWKS fetches and keeps in sync the Supabase project's JSON Web Key Set,
// used to verify the asymmetric (ES256) JWTs Supabase Auth issues by
// default. Call once at startup; the returned Keyfunc auto-refreshes keys.
func NewJWKS(ctx context.Context, supabaseURL string) (keyfunc.Keyfunc, error) {
	jwksURL := strings.TrimRight(supabaseURL, "/") + "/auth/v1/.well-known/jwks.json"
	k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, wrapJWKSErr(supabaseURL, err)
	}
	return k, nil
}

// Middleware verifies the Authorization: Bearer <token> header against the
// Supabase project's JWKS.
func Middleware(jwks keyfunc.Keyfunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenString, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || tokenString == "" {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}

			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(tokenString, claims, jwks.Keyfunc)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid token: "+err.Error())
				return
			}

			role, _ := claims["role"].(string)
			if role != "authenticated" {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "token is not an authenticated-user token")
				return
			}

			sub, _ := claims["sub"].(string)
			userID, err := uuid.Parse(sub)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid sub claim")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func wrapJWKSErr(supabaseURL string, err error) error {
	return fmt.Errorf("fetch JWKS from %s: %w", supabaseURL, err)
}
