package httpapi

import (
	"testing"
	"time"
)

func TestRateLimiterPrunesExpiredBuckets(t *testing.T) {
	limiter := newRateLimiter(10, time.Minute)
	limiter.buckets["stale"] = bucket{count: 1, expiresAt: time.Now().Add(-time.Minute)}

	if !limiter.allow("fresh") {
		t.Fatal("expected fresh key to be allowed")
	}
	if _, ok := limiter.buckets["stale"]; ok {
		t.Fatal("expected stale bucket to be pruned")
	}
	if _, ok := limiter.buckets["fresh"]; !ok {
		t.Fatal("expected fresh bucket to be recorded")
	}
}
