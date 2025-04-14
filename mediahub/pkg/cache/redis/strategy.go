package redis

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// CacheStrategy 定义缓存策略接口
type CacheStrategy interface {
	// Get 获取数据，如果缓存不存在则调用fetchFunc获取
	Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error)
}

// SimpleStrategy 简单缓存策略
type SimpleStrategy struct {
	cache Cache
}

// NewSimpleStrategy 创建简单缓存策略
func NewSimpleStrategy(cache Cache) CacheStrategy {
	return &SimpleStrategy{
		cache: cache,
	}
}

// Get 获取数据，如果缓存不存在则调用fetchFunc获取
func (s *SimpleStrategy) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// 尝试从缓存获取
	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// 缓存不存在，从数据源获取
	data, err := fetchFunc()
	if err != nil {
		return nil, err
	}

	// 设置缓存
	if data != nil {
		err = s.cache.Set(ctx, key, data, ttl)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// PenetrationStrategy 防止缓存穿透的策略
type PenetrationStrategy struct {
	cache Cache
}

// NewPenetrationStrategy 创建防止缓存穿透的策略
func NewPenetrationStrategy(cache Cache) CacheStrategy {
	return &PenetrationStrategy{
		cache: cache,
	}
}

// Get 获取数据，如果缓存不存在则调用fetchFunc获取
func (s *PenetrationStrategy) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// 尝试从缓存获取
	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// 检查是否是空值缓存
	emptyKey := key + ":empty"
	exists, err := s.cache.Exists(ctx, emptyKey)
	if err != nil {
		return nil, err
	}
	if exists {
		// 空值缓存存在，说明之前查询过且数据不存在
		return nil, nil
	}

	// 从数据源获取
	data, err := fetchFunc()
	if err != nil {
		return nil, err
	}

	// 如果数据不存在，设置空值缓存
	if data == nil {
		// 设置空值标记，过期时间比正常数据短
		err = s.cache.Set(ctx, emptyKey, "", ttl/2)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	// 设置缓存
	err = s.cache.Set(ctx, key, data, ttl)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// BreakdownStrategy 防止缓存击穿的策略
type BreakdownStrategy struct {
	cache Cache
}

// NewBreakdownStrategy 创建防止缓存击穿的策略
func NewBreakdownStrategy(cache Cache) CacheStrategy {
	return &BreakdownStrategy{
		cache: cache,
	}
}

// Get 获取数据，如果缓存不存在则调用fetchFunc获取
func (s *BreakdownStrategy) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// 尝试从缓存获取
	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// 尝试获取锁
	lockKey := key + ":lock"
	locked, err := s.cache.SetNX(ctx, lockKey, "1", 10*time.Second)
	if err != nil {
		return nil, err
	}

	if !locked {
		// 获取锁失败，说明其他协程正在更新缓存
		// 等待一段时间后重试
		time.Sleep(100 * time.Millisecond)
		return s.Get(ctx, key, fetchFunc, ttl)
	}

	// 获取锁成功，再次检查缓存（双重检查）
	val, err = s.cache.Get(ctx, key)
	if err != nil {
		// 释放锁
		s.cache.Delete(ctx, lockKey)
		return nil, err
	}
	if val != nil {
		// 缓存已更新，释放锁并返回
		s.cache.Delete(ctx, lockKey)
		return val, nil
	}

	// 从数据源获取
	data, err := fetchFunc()
	if err != nil {
		// 释放锁
		s.cache.Delete(ctx, lockKey)
		return nil, err
	}

	// 设置缓存
	if data != nil {
		err = s.cache.Set(ctx, key, data, ttl)
		if err != nil {
			// 释放锁
			s.cache.Delete(ctx, lockKey)
			return nil, err
		}
	}

	// 释放锁
	s.cache.Delete(ctx, lockKey)

	return data, nil
}

// AvalancheStrategy 防止缓存雪崩的策略
type AvalancheStrategy struct {
	cache Cache
}

// NewAvalancheStrategy 创建防止缓存雪崩的策略
func NewAvalancheStrategy(cache Cache) CacheStrategy {
	return &AvalancheStrategy{
		cache: cache,
	}
}

// Get 获取数据，如果缓存不存在则调用fetchFunc获取
func (s *AvalancheStrategy) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// 尝试从缓存获取
	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// 从数据源获取
	data, err := fetchFunc()
	if err != nil {
		return nil, err
	}

	// 设置缓存，使用随机过期时间
	if data != nil {
		// 在基础过期时间的基础上，随机增加或减少最多10%的时间
		randomFactor := 0.9 + 0.2*rand.Float64() // 0.9到1.1之间的随机数
		randomTTL := time.Duration(float64(ttl) * randomFactor)

		err = s.cache.Set(ctx, key, data, randomTTL)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// AllProtectionsStrategy 综合防止缓存穿透、击穿和雪崩的策略
type AllProtectionsStrategy struct {
	cache Cache
}

// NewAllProtectionsStrategy 创建综合防止缓存穿透、击穿和雪崩的策略
func NewAllProtectionsStrategy(cache Cache) CacheStrategy {
	return &AllProtectionsStrategy{
		cache: cache,
	}
}

// Get 获取数据，如果缓存不存在则调用fetchFunc获取
func (s *AllProtectionsStrategy) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// 尝试从缓存获取
	val, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// 检查是否是空值缓存
	emptyKey := key + ":empty"
	exists, err := s.cache.Exists(ctx, emptyKey)
	if err != nil {
		return nil, err
	}
	if exists {
		// 空值缓存存在，说明之前查询过且数据不存在
		return nil, nil
	}

	// 尝试获取锁
	lockKey := key + ":lock"
	locked, err := s.cache.SetNX(ctx, lockKey, "1", 10*time.Second)
	if err != nil {
		return nil, err
	}

	if !locked {
		// 获取锁失败，说明其他协程正在更新缓存
		// 等待一段时间后重试
		time.Sleep(100 * time.Millisecond)
		return s.Get(ctx, key, fetchFunc, ttl)
	}

	// 获取锁成功，再次检查缓存（双重检查）
	val, err = s.cache.Get(ctx, key)
	if err != nil {
		// 释放锁
		s.cache.Delete(ctx, lockKey)
		return nil, err
	}
	if val != nil {
		// 缓存已更新，释放锁并返回
		s.cache.Delete(ctx, lockKey)
		return val, nil
	}

	// 从数据源获取
	data, err := fetchFunc()
	if err != nil {
		// 释放锁
		s.cache.Delete(ctx, lockKey)
		return nil, err
	}

	// 如果数据不存在，设置空值缓存
	if data == nil {
		// 设置空值标记，过期时间比正常数据短
		err = s.cache.Set(ctx, emptyKey, "", ttl/2)
		if err != nil {
			// 释放锁
			s.cache.Delete(ctx, lockKey)
			return nil, err
		}
		// 释放锁
		s.cache.Delete(ctx, lockKey)
		return nil, nil
	}

	// 设置缓存，使用随机过期时间
	randomFactor := 0.9 + 0.2*rand.Float64() // 0.9到1.1之间的随机数
	randomTTL := time.Duration(float64(ttl) * randomFactor)

	err = s.cache.Set(ctx, key, data, randomTTL)
	if err != nil {
		// 释放锁
		s.cache.Delete(ctx, lockKey)
		return nil, err
	}

	// 释放锁
	s.cache.Delete(ctx, lockKey)

	return data, nil
} 