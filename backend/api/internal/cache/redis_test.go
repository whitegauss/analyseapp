package cache

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestCache returns a RedisCache over an in-process miniredis holding keys,
// so the real go-redis client (SCAN cursors, TTLs, redis.Nil) runs with no
// Redis container and without leaving loopback.
func newTestCache(t *testing.T, keys ...string) (*miniredis.Miniredis, *RedisCache) {
	t.Helper()

	mr := miniredis.RunT(t)
	for _, k := range keys {
		if err := mr.Set(k, "v"); err != nil {
			t.Fatalf("seed %q: %v", k, err)
		}
	}
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, &RedisCache{Client: client}
}

func TestRedisCacheSetAndGet(t *testing.T) {
	ctx := context.Background()
	mr, c := newTestCache(t)

	if err := c.Set(ctx, "k", []byte("body"), AnalysisTTL); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, found, err := c.Get(ctx, "k")
	if err != nil || !found || string(value) != "body" {
		t.Fatalf(`Get after Set = %q, %v, %v, want "body", true, nil`, value, found, err)
	}
	if got := mr.TTL("k"); got != AnalysisTTL {
		t.Errorf("TTL = %v, want %v", got, AnalysisTTL)
	}
	mr.FastForward(AnalysisTTL + time.Hour)
	if _, found, _ = c.Get(ctx, "k"); found {
		t.Errorf("the key outlived its %v ttl", AnalysisTTL)
	}

	// redis.Nil is swallowed, so a miss is found=false with a nil error: an
	// error out of Get always means Redis failed, never "no entry".
	if value, found, err = c.Get(ctx, "absent"); err != nil || found || value != nil {
		t.Errorf(`Get("absent") = %q, %v, %v, want nil, false, nil`, value, found, err)
	}

	// go-redis reads a zero ttl as "no expiry", so a result cached with 0 would
	// outlive its experiment data forever. Callers always pass AnalysisTTL.
	if err = c.Set(ctx, "forever", []byte("body"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := mr.TTL("forever"); got != 0 {
		t.Errorf(`TTL("forever") = %v, want 0 (no expiry)`, got)
	}
}

func TestRedisCacheDeleteByPrefix(t *testing.T) {
	// Sorted, since miniredis.Keys() returns its keys sorted.
	keys := []string{"analysis:exp-1:corr:h2", "analysis:exp-1:lr:h1", "analysis:exp-2:lr:h3", "session:abc"}

	tests := []struct {
		name, prefix string
		wantRemain   []string
	}{
		// What matters most: invalidating one experiment must not evict another
		// experiment's, or another feature's, keys.
		{name: "deletes every match and keeps the rest", prefix: "analysis:exp-1:", wantRemain: []string{"analysis:exp-2:lr:h3", "session:abc"}},
		{name: "no match is not an error and deletes nothing", prefix: "analysis:exp-9:", wantRemain: keys},
		// The prefix gets "*" appended and goes to SCAN MATCH, so an empty one
		// matches everything: a whole-cache flush that nothing here rejects.
		{name: "an empty prefix deletes the entire database", prefix: "", wantRemain: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, c := newTestCache(t, keys...)
			if err := c.DeleteByPrefix(context.Background(), tt.prefix); err != nil {
				t.Fatalf("DeleteByPrefix(%q): %v", tt.prefix, err)
			}
			if got := mr.Keys(); !slices.Equal(got, tt.wantRemain) {
				t.Errorf("remaining keys = %v, want %v", got, tt.wantRemain)
			}
		})
	}
}

func TestRedisCacheDeleteByPrefixFollowsTheScanCursorAcrossPages(t *testing.T) {
	const pageSize, matching = 32, 500
	keys := make([]string, 0, matching+1)
	for i := 0; i < matching; i++ {
		keys = append(keys, fmt.Sprintf("analysis:exp-1:lr:%03d", i))
	}
	mr, c := newTestCache(t, append(keys, "session:abc")...)
	pager := &pagingScanHook{pageSize: pageSize}
	c.Client.AddHook(pager)

	if err := c.DeleteByPrefix(context.Background(), "analysis:exp-1:"); err != nil {
		t.Fatalf("DeleteByPrefix: %v", err)
	}
	if want := matching/pageSize + 1; pager.scans != want {
		t.Errorf("SCAN round trips = %d, want %d (the cursor must be followed, not read once)", pager.scans, want)
	}
	if got := mr.Keys(); !slices.Equal(got, []string{"session:abc"}) {
		t.Errorf("remaining keys = %v, want only the non-matching key", got)
	}
}

func TestRedisCacheUnreachableServer(t *testing.T) {
	ctx := context.Background()
	mr, c := newTestCache(t, "analysis:exp-1:lr:h1")
	mr.Close()

	// Every method surfaces the dial failure; what keeps an unreachable Redis
	// harmless is the caller (handleAnalyzeExperiment reads a Get error as a
	// miss, and discards the Set error).
	if _, found, err := c.Get(ctx, "analysis:exp-1:lr:h1"); err == nil || found {
		t.Errorf("Get = %v, %v, want found=false and a dial error", found, err)
	}
	if err := c.Set(ctx, "k", []byte("v"), AnalysisTTL); err == nil {
		t.Error("Set = nil, want a dial error")
	}
	// A SCAN failure reaches the caller only through iter.Err(): the loop just
	// stops, so a dropped Err() check would read as "nothing matched".
	if err := c.DeleteByPrefix(ctx, "analysis:exp-1:"); err == nil {
		t.Error("DeleteByPrefix = nil, want a dial error")
	}
}

// pagingScanHook makes SCAN replies paginate. miniredis answers every SCAN
// with the whole matching key set and cursor 0, so without this DeleteByPrefix
// is only exercised single-page and reading one page and stopping would pass.
type pagingScanHook struct {
	pageSize int
	scans    int
}

func (h *pagingScanHook) DialHook(n redis.DialHook) redis.DialHook { return n }

func (h *pagingScanHook) ProcessPipelineHook(n redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return n
}

func (h *pagingScanHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		scan, ok := cmd.(*redis.ScanCmd)
		if !ok {
			return next(ctx, cmd)
		}
		// args[1] is the cursor the iterator asked for; miniredis returns an
		// empty page for a non-zero one, so ask for everything and page here.
		args := cmd.Args()
		start, _ := args[1].(uint64)
		args[1] = uint64(0)
		h.scans++
		if err := next(ctx, cmd); err != nil {
			return err
		}

		all, _ := scan.Val()
		start = min(start, uint64(len(all)))
		if end := start + uint64(h.pageSize); end < uint64(len(all)) {
			scan.SetVal(all[start:end], end)
		} else {
			scan.SetVal(all[start:], 0)
		}
		return nil
	}
}
