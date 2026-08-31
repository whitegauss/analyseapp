package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"

	"analyseapp/api/internal/auth"
)

func TestParseID(t *testing.T) {
	const canonical = "6f1d2c3b-4a59-4e8f-9b0d-1e2f3a4b5c6d"

	tests := []struct {
		name, raw, want string
	}{
		{name: "canonical dashed form", raw: canonical, want: canonical},
		{name: "uppercase is normalized to lowercase", raw: strings.ToUpper(canonical), want: canonical},
		{name: "the nil uuid is a valid id", raw: "00000000-0000-0000-0000-000000000000", want: "00000000-0000-0000-0000-000000000000"},
		// uuid.Parse also takes three non-canonical spellings. Nothing sends
		// them, but a URL carrying one reaches the store as a real id, so pin
		// that they are accepted rather than 400.
		{name: "braced form is accepted", raw: "{" + canonical + "}", want: canonical},
		{name: "urn:uuid: form is accepted", raw: "urn:uuid:" + canonical, want: canonical},
		{name: "undashed 32 hex digits are accepted", raw: strings.ReplaceAll(canonical, "-", ""), want: canonical},
		{name: "empty id", raw: ""},
		{name: "not a uuid at all", raw: "not-a-uuid"},
		// Surrounding whitespace is rejected, not trimmed -- a %20-padded id
		// in a URL is a 400.
		{name: "padded with spaces", raw: "  " + canonical + "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseID(tt.raw)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("parseID(%q) = %v, want an error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseID(%q): %v", tt.raw, err)
			}
			if got.String() != tt.want {
				t.Errorf("parseID(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Title   string         `json:"title"`
		RawData map[string]any `json:"raw_data"`
	}

	tests := []struct {
		name, body, wantTitle string
		wantErr               bool
	}{
		{name: "well-formed object", body: `{"title":"t"}`, wantTitle: "t"},
		// DisallowUnknownFields is not set, so a client may send fields this
		// version doesn't know without being rejected.
		{name: "unknown fields are ignored", body: `{"title":"t","nope":2}`, wantTitle: "t"},
		// Decode reads one value and stops, so nothing after it is examined.
		{name: "bytes after the first value are not read", body: `{"title":"t"} garbage`, wantTitle: "t"},
		// A literal null decodes into an untouched dst without an error, so
		// it is the handlers' own required-field checks that reject it.
		{name: "a null body is not an error", body: `null`},
		{name: "empty body", body: ``, wantErr: true},
		{name: "truncated object", body: `{"title":`, wantErr: true},
		{name: "wrong type for a field", body: `{"title":1}`, wantErr: true},
		{name: "array where an object is expected", body: `[]`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got payload
			err := decodeJSON(strings.NewReader(tt.body), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("decodeJSON(%q) = %+v, want an error", tt.body, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeJSON(%q): %v", tt.body, err)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", got.Title, tt.wantTitle)
			}
		})
	}
}

// requireUserID keeps reading from the request, so it stays an adapter; only
// its rejection response needs pinning here.
func TestRequireUserID(t *testing.T) {
	t.Run("no authenticated user", func(t *testing.T) {
		rec := httptest.NewRecorder()

		if _, ok := requireUserID(rec, httptest.NewRequest("GET", "/", nil)); ok {
			t.Fatal("ok = true for a request that never passed the auth middleware")
		}
		if rec.Code != 401 {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		body := decodeEnvelope(t, rec)
		if body.Error == nil || body.Error.Code != "unauthorized" {
			t.Errorf("error = %+v, want code unauthorized", body.Error)
		}
	})

	t.Run("authenticated user", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r = r.WithContext(auth.WithUserID(r.Context(), testUserID))

		got, ok := requireUserID(rec, r)
		if !ok || got != testUserID {
			t.Fatalf("requireUserID = (%v, %v), want (%v, true)", got, ok, testUserID)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("wrote %q on success, want nothing", rec.Body.String())
		}
	})
}
