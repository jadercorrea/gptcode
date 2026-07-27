// Package sessionstore provides the small, concurrency-safe fixture used by
// GPTCode's public evidence-driven repair demonstration.
package sessionstore

import (
	"sync"
	"time"
)

// Session identifies a session and the instant at which it stops being active.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// Store keeps sessions in memory and is safe for concurrent use.
// Its zero value is ready to use.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

// NewStore returns an initialized Store.
func NewStore() *Store {
	return &Store{sessions: make(map[string]Session)}
}

// Put inserts or replaces a session.
func (s *Store) Put(session Session) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions == nil {
		s.sessions = make(map[string]Session)
	}
	s.sessions[session.Token] = session
}

// Active reports whether token identifies a session that expires after now.
func (s *Store) Active(token string, now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[token]
	return ok && now.Before(session.ExpiresAt)
}
