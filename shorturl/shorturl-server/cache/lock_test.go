package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisDistributedLockUnlockOnlyDeletesOwnedToken(t *testing.T) {
	client := newFakeRedisLockClient()
	lock := newTestRedisDistributedLock(client)
	const key = "shorturl:create:test"

	locked, err := lock.Lock(key, time.Minute)
	if err != nil {
		t.Fatalf("Lock error = %v, want nil", err)
	}
	if !locked {
		t.Fatal("Lock locked = false, want true")
	}

	token := lock.ownerTokens[key]
	if token == "" {
		t.Fatal("owner token is empty after successful lock")
	}
	if got := client.values[key]; got != token {
		t.Fatalf("redis lock token = %q, want owner token %q", got, token)
	}

	if err := lock.Unlock(key); err != nil {
		t.Fatalf("Unlock error = %v, want nil", err)
	}
	if _, ok := client.values[key]; ok {
		t.Fatalf("redis lock key still exists after owner unlock: %#v", client.values)
	}
}

func TestRedisDistributedLockUnlockKeepsNewOwnerAfterTTLReacquire(t *testing.T) {
	client := newFakeRedisLockClient()
	oldLock := newTestRedisDistributedLock(client)
	const key = "shorturl:create:test"

	locked, err := oldLock.Lock(key, time.Minute)
	if err != nil {
		t.Fatalf("Lock error = %v, want nil", err)
	}
	if !locked {
		t.Fatal("Lock locked = false, want true")
	}

	// 模拟旧锁 TTL 到期后，新实例已经用不同 token 抢到同一个锁。
	// 旧实例随后执行 defer Unlock 时，不能删除新实例持有的锁。
	client.values[key] = "new-owner-token"

	if err := oldLock.Unlock(key); err != nil {
		t.Fatalf("Unlock error = %v, want nil", err)
	}
	if got := client.values[key]; got != "new-owner-token" {
		t.Fatalf("redis lock token after stale unlock = %q, want new owner token", got)
	}
}

func newTestRedisDistributedLock(client *fakeRedisLockClient) *RedisDistributedLock {
	return &RedisDistributedLock{
		redisClient: client,
		ownerTokens: map[string]string{},
	}
}

type fakeRedisLockClient struct {
	values map[string]string
}

func newFakeRedisLockClient() *fakeRedisLockClient {
	return &fakeRedisLockClient{values: map[string]string{}}
}

func (c *fakeRedisLockClient) SetNX(_ context.Context, key string, value interface{}, _ time.Duration) *redis.BoolCmd {
	if _, ok := c.values[key]; ok {
		return redis.NewBoolResult(false, nil)
	}
	c.values[key] = fmt.Sprint(value)
	return redis.NewBoolResult(true, nil)
}

func (c *fakeRedisLockClient) Eval(_ context.Context, _ string, keys []string, args ...interface{}) *redis.Cmd {
	if len(keys) == 0 || len(args) == 0 {
		return redis.NewCmdResult(int64(0), nil)
	}

	key := keys[0]
	expectedToken := fmt.Sprint(args[0])
	if c.values[key] == expectedToken {
		delete(c.values, key)
		return redis.NewCmdResult(int64(1), nil)
	}
	return redis.NewCmdResult(int64(0), nil)
}
