package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache 定义缓存接口
type Cache interface {
	// Get 获取缓存
	Get(ctx context.Context, key string) (interface{}, error)
	// Set 设置缓存
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	// Delete 删除缓存
	Delete(ctx context.Context, key string) error
	// Exists 检查缓存是否存在
	Exists(ctx context.Context, key string) (bool, error)
	// SetNX 设置缓存，如果不存在
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	// GetOrSet 获取缓存，如果不存在则设置
	GetOrSet(ctx context.Context, key string, value interface{}, ttl time.Duration) (interface{}, error)
	// Close 关闭缓存连接
	Close() error
}

// RedisCache 是Cache接口的Redis实现
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 创建Redis缓存实例
func NewRedisCache(client *redis.Client) Cache {
	return &RedisCache{
		client: client,
	}
}

// Get 获取缓存
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Set 设置缓存
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return err
		}
		val = string(bytes)
	}
	return c.client.Set(ctx, key, val, ttl).Err()
}

// Delete 删除缓存
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists 检查缓存是否存在
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	val, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// SetNX 设置缓存，如果不存在
func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return false, err
		}
		val = string(bytes)
	}
	return c.client.SetNX(ctx, key, val, ttl).Result()
}

// GetOrSet 获取缓存，如果不存在则设置
func (c *RedisCache) GetOrSet(ctx context.Context, key string, value interface{}, ttl time.Duration) (interface{}, error) {
	// 尝试获取缓存
	val, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// 缓存不存在，设置缓存
	err = c.Set(ctx, key, value, ttl)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Close 关闭缓存连接
func (c *RedisCache) Close() error {
	return c.client.Close()
} 