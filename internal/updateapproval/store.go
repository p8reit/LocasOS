package updateapproval

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type entry struct {
	generation uint64
	selection  Selection
	expires    time.Time
}

// Store is deliberately volatile: restarting core invalidates all approvals.
// At most one pending discovery/selection exists per authenticated session.
type Store struct {
	mu      sync.Mutex
	next    uint64
	entries map[string]entry
	now     func() time.Time
}

func NewStore() *Store { return &Store{entries: make(map[string]entry), now: time.Now} }

func (s *Store) Begin(session string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, e := range s.entries {
		if !now.Before(e.expires) {
			delete(s.entries, key)
		}
	}
	if session == "" {
		return 0, errors.New("session required")
	}
	if _, exists := s.entries[session]; !exists && len(s.entries) >= 256 {
		return 0, errors.New("too many pending approvals")
	}
	s.next++
	s.entries[session] = entry{generation: s.next, expires: now.Add(Lifetime)}
	return s.next, nil
}

func (s *Store) Issue(session string, generation uint64, version, channel, digest string) (Selection, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, exists := s.entries[session]
	if !exists || e.generation != generation || !s.now().Before(e.expires) {
		return Selection{}, time.Time{}, errors.New("discovery invalidated; check again")
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return Selection{}, time.Time{}, err
	}
	selection := Selection{Version: version, Channel: channel, SHA256: digest, Token: hex.EncodeToString(token[:])}
	if err := selection.Validate(); err != nil {
		return Selection{}, time.Time{}, err
	}
	e.selection = selection
	e.expires = s.now().Add(Lifetime)
	s.entries[session] = e
	return selection, e.expires, nil
}

func (s *Store) Consume(session string, selection Selection) (Job, error) {
	if err := selection.Validate(); err != nil {
		return Job{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, exists := s.entries[session]
	if !exists || e.selection != selection || !s.now().Before(e.expires) {
		return Job{}, errors.New("approval unavailable, expired or changed; check again")
	}
	delete(s.entries, session) // burn before enqueue, even if the IPC result is uncertain
	return Job{Selection: selection, ExpiresAt: e.expires}, nil
}

func (s *Store) Invalidate(session string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, session)
}
func (s *Store) InvalidateAll() { s.mu.Lock(); defer s.mu.Unlock(); clear(s.entries) }
