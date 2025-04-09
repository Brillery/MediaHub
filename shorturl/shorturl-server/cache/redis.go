package cache

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	pkgredis "shorturl/pkg/db/redis"
	"time"
)

// redisKVCache 是基于Redis的键值缓存实现。
type redisKVCache struct {
	redisClient *redis.Client // Redis客户端实例。
	destroy     func()        // 销毁时调用的资源释放函数。
}

// newRedisKVCache 创建并返回一个新的Redis键值缓存实例。
//
// 参数:
//
//	client *redis.Client - Redis客户端实例。
//	destroy func() - 销毁时调用的资源释放函数。
//
// 返回值:
//
//	KVCache接口的实现，即 *redisKVCache。
func newRedisKVCache(client *redis.Client, destroy func()) KVCache {
	return &redisKVCache{
		redisClient: client,
		destroy:     destroy,
	}
}

// getKey 生成用于Redis键的完整键名。
//
// 参数:
//
//	key string - 原始键名。
//
// 返回值:
//
//	完整的键名字符串（可能包含命名空间或其他前缀）。
func getKey(key string) string {
	return pkgredis.GetKey(key)
}

// Get 根据给定键从缓存中获取值。
//
// 参数:
//
//	key string - 要查询的键名。
//
// 返回值:
//
//	string - 缓存中的值，如果键不存在则返回空字符串。
//	error - 错误信息，当键存在但发生错误时返回，否则为nil。
func (c *redisKVCache) Get(key string) (string, error) {
	key = getKey(key)
	rs, err := c.redisClient.Get(context.Background(), key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // 键不存在时返回空字符串而非错误
	}
	return rs, err
}

// Set 将键值对存储到缓存中，并设置过期时间。
//
// 参数:
//
//	key string - 要存储的键名。
//	value string - 要存储的值。
//	ttl int - 键的生存时间（秒）。
//
// 返回值:
//
//	error - 存储操作的错误信息，若成功则为nil。
func (c *redisKVCache) Set(key, value string, ttl int) error {
	key = getKey(key)
	return c.redisClient.SetEx(context.Background(), key, value, time.Second*time.Duration(ttl)).Err()
}

// Destroy 释放与缓存相关的资源。
//
// 调用时会执行destroy函数（如将Redis客户端放回连接池）。
func (c *redisKVCache) Destroy() {
	if c.destroy != nil {
		c.destroy()
	}
}

// redisCacheFactory 是用于创建Redis键值缓存实例的工厂。
type redisCacheFactory struct {
	redisPool pkgredis.RedisPool // Redis连接池实例。
}

// NewRedisCacheFactory 创建一个新的Redis缓存工厂实例。
//
// 参数:
//
//	redisPool pkgredis.RedisPool - Redis连接池实例。
//
// 返回值:
//
//	CacheFactory接口的实现，即 *redisCacheFactory。
func NewRedisCacheFactory(redisPool pkgredis.RedisPool) CacheFactory {
	return &redisCacheFactory{
		redisPool: redisPool,
	}
}

// NewKVCache 创建并返回一个新的Redis键值缓存实例。
//
// 返回值:
//
//	KVCache接口的实现，基于从连接池获取的Redis客户端。
//	销毁时会将客户端放回连接池。
func (f *redisCacheFactory) NewKVCache() KVCache {
	client := f.redisPool.Get()
	return newRedisKVCache(client, func() {
		f.redisPool.Put(client)
	})
}
