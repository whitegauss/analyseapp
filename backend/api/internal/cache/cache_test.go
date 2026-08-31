package cache

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAnalysisKey(t *testing.T) {
	id := uuid.New()

	key, err := AnalysisKey(id, "linear_regression", map[string]any{"weighted": true})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}

	wantPrefix := "analysis:" + id.String() + ":linear_regression:"
	if !strings.HasPrefix(key, wantPrefix) {
		t.Errorf("key = %q, want prefix %q", key, wantPrefix)
	}
}

func TestAnalysisKey_DeterministicRegardlessOfParamOrder(t *testing.T) {
	id := uuid.New()

	key1, err := AnalysisKey(id, "linear_regression", map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}
	key2, err := AnalysisKey(id, "linear_regression", map[string]any{"b": 2, "a": 1})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}

	if key1 != key2 {
		t.Errorf("key1 = %q, key2 = %q, want equal (map iteration order shouldn't matter)", key1, key2)
	}
}

func TestAnalysisKey_DifferentParamsProduceDifferentKeys(t *testing.T) {
	id := uuid.New()

	key1, err := AnalysisKey(id, "linear_regression", map[string]any{"weighted": true})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}
	key2, err := AnalysisKey(id, "linear_regression", map[string]any{"weighted": false})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}

	if key1 == key2 {
		t.Errorf("key1 == key2 == %q, want different keys for different params", key1)
	}
}

func TestAnalysisKey_DifferentTypesProduceDifferentKeys(t *testing.T) {
	id := uuid.New()

	key1, err := AnalysisKey(id, "linear_regression", map[string]any{})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}
	key2, err := AnalysisKey(id, "some_other_type", map[string]any{})
	if err != nil {
		t.Fatalf("AnalysisKey: %v", err)
	}

	if key1 == key2 {
		t.Errorf("key1 == key2 == %q, want different keys for different analysis types", key1)
	}
}

func TestAnalysisKeyPrefix(t *testing.T) {
	id, other := uuid.New(), uuid.New()
	prefix := AnalysisKeyPrefix(id)

	// AnalysisKey and AnalysisKeyPrefix are written independently but have to
	// agree: if the prefix stopped matching the keys, DeleteByPrefix would
	// delete nothing and stale results would survive a raw_data edit silently.
	for _, analysisType := range []string{"linear_regression", "correlation"} {
		key, err := AnalysisKey(id, analysisType, map[string]any{"weighted": true})
		if err != nil {
			t.Fatalf("AnalysisKey: %v", err)
		}
		if !strings.HasPrefix(key, prefix) {
			t.Errorf("AnalysisKey(%s) = %q, want it to start with the prefix %q", analysisType, key, prefix)
		}
	}
	// DeleteByPrefix hands the prefix to SCAN MATCH unescaped, so a glob
	// metacharacter in it would match keys it does not name. A UUID has none.
	if AnalysisKeyPrefix(other) == prefix || strings.ContainsAny(prefix, `*?[]\`) {
		t.Errorf("prefix = %q, want it distinct from %s's and free of glob metacharacters", prefix, other)
	}
}
