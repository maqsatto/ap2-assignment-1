package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"order-service/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisOrderCache struct {
	client *redis.Client
}

func NewRedisOrderCache(client *redis.Client) *RedisOrderCache {
	return &RedisOrderCache{client: client}
}

func (c *RedisOrderCache) Get(id string) (*domain.Order, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data, err := c.client.Get(ctx, orderCacheKey(id)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get order cache: %w", err)
	}

	var order domain.Order
	if err := json.Unmarshal(data, &order); err != nil {
		_ = c.Delete(id)
		return nil, false, fmt.Errorf("decode order cache: %w", err)
	}

	return &order, true, nil
}

func (c *RedisOrderCache) Set(order *domain.Order, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("encode order cache: %w", err)
	}

	if err := c.client.Set(ctx, orderCacheKey(order.ID), data, ttl).Err(); err != nil {
		return fmt.Errorf("set order cache: %w", err)
	}
	return nil
}

func (c *RedisOrderCache) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return c.client.Del(ctx, orderCacheKey(id)).Err()
}

func orderCacheKey(id string) string {
	return "order:" + id
}
