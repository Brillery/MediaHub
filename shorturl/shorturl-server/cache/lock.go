package cache

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"math/rand"
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

// RedisDistributedLock 基于Redis的分布式锁实现
type RedisDistributedLock struct {
	redisClient *redis.Client
	destroy     func()
}

// NewRedisDistributedLock 创建一个新的Redis分布式锁
func NewRedisDistributedLock(client *redis.Client, destroy func()) DistributedLock {
	return &RedisDistributedLock{
		redisClient: client,
		destroy:     destroy,
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
		// 将锁的标识值存储在客户端，用于解锁时验证
		l.redisClient.Set(context.Background(), key+"_owner", value, ttl)
	}

	return success, nil
}

// Unlock 释放锁
func (l *RedisDistributedLock) Unlock(key string) error {
	// 获取锁的标识值
	ownerValue, err := l.redisClient.Get(context.Background(), key+"_owner").Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 锁已经不存在，直接返回
			return nil
		}
		return err
	}

	// 获取当前锁的值
	currentValue, err := l.redisClient.Get(context.Background(), key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 锁已经不存在，直接返回
			return nil
		}
		return err
	}

	// 验证当前锁是否由当前客户端持有
	if currentValue != ownerValue {
		return errors.New("lock is not owned by this client")
	}

	// 使用Lua脚本原子性地删除锁
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	_, err = l.redisClient.Eval(context.Background(), script, []string{key}, []interface{}{ownerValue}).Result()
	if err != nil {
		return err
	}

	// 删除锁的标识值
	l.redisClient.Del(context.Background(), key+"_owner")

	return nil
}

// generateRandomValue 生成随机值
func generateRandomValue() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
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
