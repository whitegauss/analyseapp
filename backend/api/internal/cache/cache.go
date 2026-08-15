// Package cache stores analysis results in Redis (PDR.md section 7), keyed
// as analysis:{experiment_id}:{type}:{params_hash}. Caching is best-effort:
// callers treat a Redis error the same as a cache miss rather than failing
// the request, so an unreachable Redis degrades performance, not
// correctness.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Cache stores and retrieves opaque byte values (the worker's raw JSON
// response body) by key.
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// AnalysisTTL is how long a cached analysis result is kept (PDR.md section
// 7: "実験データが不変なら長め（24h目安）").
const AnalysisTTL = 24 * time.Hour

// AnalysisKey builds the cache key for a given experiment/analysis-type/
// params combination. params is hashed rather than embedded directly since
// it's an arbitrary, unordered map; encoding/json sorts object keys when
// marshaling a Go map, so the hash is stable regardless of map iteration
// order.
func AnalysisKey(experimentID uuid.UUID, analysisType string, params map[string]any) (string, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(paramsJSON)
	return fmt.Sprintf("analysis:%s:%s:%s", experimentID, analysisType, hex.EncodeToString(sum[:])), nil
}
