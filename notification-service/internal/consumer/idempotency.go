package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type IdempotencyStore interface {
	AlreadyProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string) error
}

type InMemoryIdempotencyStore struct {
	mu        sync.Mutex
	processed map[string]struct{}
}

func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{processed: make(map[string]struct{})}
}

func (s *InMemoryIdempotencyStore) AlreadyProcessed(ctx context.Context, eventID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.processed[eventID]
	return ok, nil
}

func (s *InMemoryIdempotencyStore) MarkProcessed(ctx context.Context, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[eventID] = struct{}{}
	return nil
}

type RedisIdempotencyStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisIdempotencyStore(client *redis.Client, ttl time.Duration) *RedisIdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RedisIdempotencyStore{client: client, ttl: ttl}
}

func (s *RedisIdempotencyStore) AlreadyProcessed(ctx context.Context, eventID string) (bool, error) {
	value, err := s.client.Get(ctx, idempotencyKey(eventID)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check idempotency: %w", err)
	}
	return value == "sent", nil
}

func (s *RedisIdempotencyStore) MarkProcessed(ctx context.Context, eventID string) error {
	if err := s.client.Set(ctx, idempotencyKey(eventID), "sent", s.ttl).Err(); err != nil {
		return fmt.Errorf("mark idempotency: %w", err)
	}
	return nil
}

func idempotencyKey(eventID string) string {
	return "notification:sent:" + eventID
}
