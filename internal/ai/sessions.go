package ai

import (
	"fmt"
	"sync"
	"time"
)

type Session struct {
	ID        string
	CreatedAt time.Time
}

type SessionStore struct {
	mu       sync.Mutex
	sequence uint64
}

func NewSessionStore() *SessionStore {
	return &SessionStore{}
}

func (s *SessionStore) NewSession() Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sequence++
	return Session{
		ID:        fmt.Sprintf("session-%d", s.sequence),
		CreatedAt: time.Now().UTC(),
	}
}
