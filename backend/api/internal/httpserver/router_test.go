package httpserver

import (
	"net/http"
	"slices"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRegisterAPIRoutes pins the routing table itself. Every handler below
// has its own test, but one that was written and never wired up would pass
// all of those and 404 in production -- easy to miss for the nested paths,
// where /projects/{id} already exists and a missing
// /projects/{id}/experiments is a 404 either way.
//
// The stores are fakes that fail the test if any method is called: walking
// the table never invokes a handler, so reaching one would mean chi matched
// something unexpected.
func TestRegisterAPIRoutes(t *testing.T) {
	r := chi.NewRouter()
	registerAPIRoutes(r, &fakeStore{t: t}, &fakeProjectStore{t: t}, nil, nil)

	var got []string
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got = append(got, method+" "+route)
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	want := []string{
		"POST /experiments",
		"GET /experiments",
		"GET /experiments/{id}",
		"DELETE /experiments/{id}",
		"PATCH /experiments/{id}/config",
		"PATCH /experiments/{id}/raw_data",
		"POST /experiments/{id}/analyze",
		"POST /projects",
		"GET /projects",
		"GET /projects/{id}",
		"PATCH /projects/{id}",
		"DELETE /projects/{id}",
		"POST /projects/{id}/experiments",
		"GET /projects/{id}/experiments",
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("route %q is not registered (registered: %v)", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("registered %d routes, want %d: %v", len(got), len(want), got)
	}
}
