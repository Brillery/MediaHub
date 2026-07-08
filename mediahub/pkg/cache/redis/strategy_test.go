package redis

import (
	"context"
	"testing"
	"time"
)

func TestBreakdownStrategyDoesNotDeleteLockOwnedByNextWorker(t *testing.T) {
	ctx := context.Background()
	cache := newMemoryCache()
	strategy := NewBreakdownStrategy(cache)

	_, err := strategy.Get(ctx, "hot-key", func() (interface{}, error) {
		// 模拟当前 worker 业务查询较慢，锁过期后已被下一轮 worker 接管。
		cache.values["hot-key:lock"] = "next-worker-token"
		return "fresh-data", nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("strategy.Get returned error: %v", err)
	}

	if got := cache.values["hot-key:lock"]; got != "next-worker-token" {
		t.Fatalf("lock owned by next worker was deleted or changed, got %q", got)
	}
}

func TestAllProtectionsStrategyDoesNotDeleteLockOwnedByNextWorkerWhenDataMissing(t *testing.T) {
	ctx := context.Background()
	cache := newMemoryCache()
	strategy := NewAllProtectionsStrategy(cache)

	_, err := strategy.Get(ctx, "missing-key", func() (interface{}, error) {
		// 数据源确认为空时也会走释放锁分支，必须同样保护后续 worker 的锁。
		cache.values["missing-key:lock"] = "next-worker-token"
		return nil, nil
	}, time.Minute)
	if err != nil {
		t.Fatalf("strategy.Get returned error: %v", err)
	}

	if got := cache.values["missing-key:lock"]; got != "next-worker-token" {
		t.Fatalf("lock owned by next worker was deleted or changed, got %q", got)
	}
}

type memoryCache struct {
	values map[string]interface{}
}

func newMemoryCache() *memoryCache {
	return &memoryCache{values: make(map[string]interface{})}
}

func (c *memoryCache) Get(_ context.Context, key string) (interface{}, error) {
	return c.values[key], nil
}

func (c *memoryCache) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	c.values[key] = value
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	delete(c.values, key)
	return nil
}

func (c *memoryCache) CompareAndDelete(_ context.Context, key string, expected string) (bool, error) {
	if c.values[key] != expected {
		return false, nil
	}

	delete(c.values, key)
	return true, nil
}

func (c *memoryCache) Exists(_ context.Context, key string) (bool, error) {
	_, ok := c.values[key]
	return ok, nil
}

func (c *memoryCache) SetNX(_ context.Context, key string, value interface{}, _ time.Duration) (bool, error) {
	if _, ok := c.values[key]; ok {
		return false, nil
	}

	c.values[key] = value
	return true, nil
}

func (c *memoryCache) GetOrSet(_ context.Context, key string, value interface{}, _ time.Duration) (interface{}, error) {
	if cached, ok := c.values[key]; ok {
		return cached, nil
	}

	c.values[key] = value
	return value, nil
}

func (c *memoryCache) Close() error {
	return nil
}
