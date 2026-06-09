package httpapi

import (
	"testing"
	"time"
)

func TestRateLimiterPrunesExpiredBuckets(t *testing.T) {
	limiter := newRateLimiter(10, time.Minute)
	limiter.buckets["stale"] = bucket{count: 1, expiresAt: time.Now().Add(-time.Minute)}

	if ok, retryAfter := limiter.allow("fresh"); !ok || retryAfter != 0 {
		t.Fatal("expected fresh key to be allowed")
	}
	if _, ok := limiter.buckets["stale"]; ok {
		t.Fatal("expected stale bucket to be pruned")
	}
	if _, ok := limiter.buckets["fresh"]; !ok {
		t.Fatal("expected fresh bucket to be recorded")
	}
}

func TestRateLimiterReturnsRetryAfterWhenLimited(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)

	if ok, _ := limiter.allow("client"); !ok {
		t.Fatal("expected first request to be allowed")
	}
	ok, retryAfter := limiter.allow("client")
	if ok {
		t.Fatal("expected second request to be rate limited")
	}
	if retryAfter <= 0 {
		t.Fatalf("expected positive retry-after duration, got %s", retryAfter)
	}
}

func TestRetryAfterSecondsRoundsToAtLeastOneSecond(t *testing.T) {
	if got := retryAfterSeconds(100 * time.Millisecond); got != "1" {
		t.Fatalf("expected minimum retry-after of 1 second, got %q", got)
	}
}
