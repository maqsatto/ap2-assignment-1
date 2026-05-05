package consumer

import "sync"

type IdempotencyStore interface {
	AlreadyProcessed(eventID string) bool
	MarkProcessed(eventID string)
}

type InMemoryIdempotencyStore struct {
	mu        sync.Mutex
	processed map[string]struct{}
}

func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{processed: make(map[string]struct{})}
}

func (s *InMemoryIdempotencyStore) AlreadyProcessed(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.processed[eventID]
	return ok
}

func (s *InMemoryIdempotencyStore) MarkProcessed(eventID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[eventID] = struct{}{}
}
