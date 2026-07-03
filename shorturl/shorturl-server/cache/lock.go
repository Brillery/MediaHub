package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/redis/go-redis/v9"
	pkgredis "shorturl/pkg/db/redis"
	"time"
)

// DistributedLock 分布式锁接口
type DistributedLock interface {
	// Lock 尝试获取锁
	Lock(key string, ttl time.Duration) (bool, error)
	// Unlock 释放锁
	Unlock(key string) error
}

// redisLockClient 抽象 Redis 锁需要的最小命令集合，便于测试锁 token 归属语义。
type redisLockClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// RedisDistributedLock 基于Redis的分布式锁实现
type RedisDistributedLock struct {
	redisClient redisLockClient
	destroy     func()
	// ownerTokens 只保存在当前锁实例内，表示本实例成功获取到的 key/token。
	// 不能把 owner token 放到 Redis 的共享 key 中，否则锁过期后新实例重抢会覆盖 owner，
	// 旧实例释放时可能误删新实例的锁。
	ownerTokens map[string]string
}

// NewRedisDistributedLock 创建一个新的Redis分布式锁
func NewRedisDistributedLock(client *redis.Client, destroy func()) DistributedLock {
	return &RedisDistributedLock{
		redisClient: client,
		destroy:     destroy,
		ownerTokens: map[string]string{},
	}
}

// Lock 尝试获取锁
func (l *RedisDistributedLock) Lock(key string, ttl time.Duration) (bool, error) {
	// 生成随机值，用于标识锁的持有者
	value := generateRandomValue()

	// 使用SET命令的NX选项实现互斥锁
	// NX: 只有当key不存在时才设置
	// EX: 设置过期时间（秒）
	success, err := l.redisClient.SetNX(context.Background(), key, value, ttl).Result()
	if err != nil {
		return false, err
	}

	if success {
		// 锁归属必须保存在当前实例内，Unlock 时只允许释放自己拿到的 token。
		l.ownerTokens[key] = value
	}

	return success, nil
}

// Unlock 释放锁
func (l *RedisDistributedLock) Unlock(key string) error {
	ownerValue, ok := l.ownerTokens[key]
	if !ok {
		return nil
	}

	// 使用 Lua 脚本原子校验 token 后删除。
	// 如果锁已过期并被其他实例重新获取，Redis 中的值会变成新 token，这里不会删除新锁。
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	_, err := l.redisClient.Eval(context.Background(), script, []string{key}, ownerValue).Result()
	if err != nil {
		return err
	}
	delete(l.ownerTokens, key)

	return nil
}

// generateRandomValue 生成随机值
func generateRandomValue() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}

	// 随机源异常极少发生；兜底值只用于释放校验，不参与业务协议。
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// DistributedLockFactory 分布式锁工厂接口
type DistributedLockFactory interface {
	// NewDistributedLock 创建一个新的分布式锁实例
	NewDistributedLock() DistributedLock
}

// RedisDistributedLockFactory 基于Redis的分布式锁工厂
type RedisDistributedLockFactory struct {
	redisPool pkgredis.RedisPool
}

// NewRedisDistributedLockFactory 创建一个新的Redis分布式锁工厂
func NewRedisDistributedLockFactory(redisPool pkgredis.RedisPool) DistributedLockFactory {
	return &RedisDistributedLockFactory{
		redisPool: redisPool,
	}
}

// NewDistributedLock 创建一个新的分布式锁实例
func (f *RedisDistributedLockFactory) NewDistributedLock() DistributedLock {
	client := f.redisPool.Get()
	return NewRedisDistributedLock(client, func() {
		f.redisPool.Put(client)
	})
}
