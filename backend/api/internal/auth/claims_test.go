package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestParseBearer(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantErr   error
	}{
		{name: "well-formed bearer header", header: "Bearer abc.def.ghi", wantToken: "abc.def.ghi"},
		{name: "token may contain spaces", header: "Bearer a b", wantToken: "a b"},
		{name: "absent header", header: "", wantErr: errMissingBearer},
		{name: "another scheme", header: "Basic YWxhZGRpbjpvcGVuc2VzYW1l", wantErr: errMissingBearer},
		{name: "scheme with no token", header: "Bearer ", wantErr: errMissingBearer},
		{name: "scheme without separating space", header: "Bearerabc", wantErr: errMissingBearer},
		// CutPrefix is case-sensitive, so the lowercase spelling some clients
		// send is rejected. Pinning it here so a change is deliberate.
		{name: "lowercase scheme", header: "bearer abc", wantErr: errMissingBearer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBearer(tt.header)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.wantToken {
				t.Errorf("token = %q, want %q", got, tt.wantToken)
			}
		})
	}
}

func TestUserIDFromClaims(t *testing.T) {
	const sub = "6f1d2c3b-4a59-4e8f-9b0d-1e2f3a4b5c6d"

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		wantID  string
		wantErr error
	}{
		{
			name:   "authenticated user token",
			claims: jwt.MapClaims{"role": "authenticated", "sub": sub},
			wantID: sub,
		},
		{
			name:    "anon key is not a user token",
			claims:  jwt.MapClaims{"role": "anon", "sub": sub},
			wantErr: errNotUserToken,
		},
		{
			name:    "service_role key is not a user token",
			claims:  jwt.MapClaims{"role": "service_role", "sub": sub},
			wantErr: errNotUserToken,
		},
		{
			name:    "role claim absent",
			claims:  jwt.MapClaims{"sub": sub},
			wantErr: errNotUserToken,
		},
		{
			name:    "role claim is not a string",
			claims:  jwt.MapClaims{"role": 1, "sub": sub},
			wantErr: errNotUserToken,
		},
		{
			name:    "sub claim absent",
			claims:  jwt.MapClaims{"role": "authenticated"},
			wantErr: errInvalidSub,
		},
		{
			name:    "sub claim is not a uuid",
			claims:  jwt.MapClaims{"role": "authenticated", "sub": "not-a-uuid"},
			wantErr: errInvalidSub,
		},
		{
			name:    "sub claim is not a string",
			claims:  jwt.MapClaims{"role": "authenticated", "sub": 42},
			wantErr: errInvalidSub,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := userIDFromClaims(tt.claims)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != uuid.Nil {
					t.Errorf("id = %v, want uuid.Nil on error", got)
				}
				return
			}
			if got.String() != tt.wantID {
				t.Errorf("id = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestUserIDRoundTrip(t *testing.T) {
	want := uuid.MustParse("6f1d2c3b-4a59-4e8f-9b0d-1e2f3a4b5c6d")

	got, ok := UserID(WithUserID(context.Background(), want))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got != want {
		t.Errorf("id = %v, want %v", got, want)
	}
}

func TestUserIDAbsent(t *testing.T) {
	if _, ok := UserID(context.Background()); ok {
		t.Error("ok = true for a context that never passed Middleware, want false")
	}
}

func TestUserIDWrongType(t *testing.T) {
	// A value stored under the same key but of another type must not be
	// mistaken for an authenticated user.
	ctx := context.WithValue(context.Background(), userIDKey, "6f1d2c3b-4a59-4e8f-9b0d-1e2f3a4b5c6d")

	if _, ok := UserID(ctx); ok {
		t.Error("ok = true for a non-uuid value, want false")
	}
}
