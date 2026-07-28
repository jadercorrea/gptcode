package expiringcache

import (
	"sync"
	"testing"
	"time"
)

func TestGetReturnsLiveEntry(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	cache := newWithClock(func() time.Time { return now })
	cache.Set("session", "active", time.Minute)

	value, found := cache.Get("session")
	if !found || value != "active" {
		t.Fatalf("Get() = %q, %v, want active, true", value, found)
	}
}

func TestGetEvictsExpiredEntry(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	cache := newWithClock(func() time.Time { return now })
	cache.Set("session", "expired", time.Minute)
	now = now.Add(time.Minute)

	if value, found := cache.Get("session"); found || value != "" {
		t.Fatalf("Get() = %q, %v, want empty, false", value, found)
	}
	if length := cache.Len(); length != 0 {
		t.Fatalf("Len() = %d, want expired entry removed", length)
	}
}

func TestLenPurgesExpiredEntriesWithoutLookup(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	cache := newWithClock(func() time.Time { return now })
	cache.Set("expired", "stale", time.Minute)
	cache.Set("live", "current", 2*time.Minute)
	now = now.Add(time.Minute)

	if length := cache.Len(); length != 1 {
		t.Fatalf("Len() = %d, want only the live entry", length)
	}
	if value, found := cache.Get("live"); !found || value != "current" {
		t.Fatalf("Get(live) = %q, %v, want current, true", value, found)
	}
}

func TestCacheSupportsConcurrentReadersAndWriters(t *testing.T) {
	cache := New()
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for iteration := range 250 {
				key := string(rune('a' + worker))
				cache.Set(key, "value", time.Minute)
				cache.Get(key)
				if iteration%10 == 0 {
					cache.Len()
				}
			}
		}()
	}
	workers.Wait()
}
