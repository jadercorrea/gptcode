package sessionstore

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStoreActive(t *testing.T) {
	now := time.Date(2026, time.July, 25, 18, 0, 0, 0, time.UTC)
	store := NewStore()

	store.Put(Session{Token: "active", ExpiresAt: now.Add(time.Minute)})
	store.Put(Session{Token: "expired", ExpiresAt: now})

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "active session", token: "active", want: true},
		{name: "expiration boundary is exclusive", token: "expired", want: false},
		{name: "unknown session", token: "missing", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := store.Active(tt.token, now); got != tt.want {
				t.Fatalf("Active(%q, now) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestStoreSupportsConcurrentAccess(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, time.July, 25, 18, 0, 0, 0, time.UTC)

	var workers sync.WaitGroup
	for i := range 64 {
		workers.Add(2)
		token := fmt.Sprintf("session-%d", i)

		go func() {
			defer workers.Done()
			store.Put(Session{Token: token, ExpiresAt: now.Add(time.Minute)})
		}()

		go func() {
			defer workers.Done()
			_ = store.Active(token, now)
		}()
	}

	workers.Wait()
}

func TestStoreZeroValueIsReadyToUse(t *testing.T) {
	var store Store
	now := time.Date(2026, time.July, 25, 18, 0, 0, 0, time.UTC)

	store.Put(Session{Token: "zero-value", ExpiresAt: now.Add(time.Minute)})

	if !store.Active("zero-value", now) {
		t.Fatal("zero-value Store did not retain an active session")
	}
}
