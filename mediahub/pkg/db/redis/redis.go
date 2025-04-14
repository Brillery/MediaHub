package redis

import (
	"context"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"fmt"
	"github.com/redis/go-redis/v9"
	"math/rand"
	"sync"
	"time"
)

// RedisPool 定义了Redis连接池的接口
type RedisPool interface {
	Get() *redis.Client
	Put(client *redis.Client)
}

// pool 是RedisPool接口的一个全局实例
var pool RedisPool

// redisPool 是RedisPool接口的具体实现
type redisPool struct {
	pool sync.Pool
}

// Get 从池中获取一个Redis客户端实例
// 如果获取的客户端实例不可用，则创建并返回一个新的客户端实例
func (p *redisPool) Get() *redis.Client {
	client := p.pool.Get().(*redis.Client)
	if client.Ping(context.Background()).Err() != nil {
		client = p.pool.New().(*redis.Client)
	}
	return client
}

// Put 将Redis客户端实例放回池中
// 如果客户端实例不可用，则不将其放回池中
func (p *redisPool) Put(client *redis.Client) {
	if client.Ping(context.Background()).Err() != nil {
		return
	}
	p.pool.Put(client)
}

// getPool 根据配置信息创建并返回一个Redis连接池实例
func getPool(cnf *config.Config) RedisPool {
	return &redisPool{
		pool: sync.Pool{
			New: func() any {
				rdb := redis.NewClient(&redis.Options{
					Addr:     fmt.Sprintf("%s:%d", cnf.Redis.Host, cnf.Redis.Port),
					Password: cnf.Redis.Pwd,
				})
				return rdb
			},
		},
	}
}

// InitRedisPool 初始化Redis连接池
func InitRedisPool(cnf *config.Config) {
	pool = getPool(cnf)
}

// GetPool 返回全局的RedisPool实例
func GetPool() RedisPool {
	return pool
}

// CacheService 提供缓存服务
type CacheService struct {
	client *redis.Client
}

// NewCacheService 创建一个新的缓存服务
func NewCacheService() *CacheService {
	client := GetPool().Get()
	return &CacheService{
		client: client,
	}
}

// GetWithCachePenetration 防止缓存穿透的获取方法
// key: 缓存键
// ttl: 缓存过期时间
// fetchFunc: 当缓存不存在时，从数据库获取数据的函数
func (cs *CacheService) GetWithCachePenetration(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// 1. 尝试从缓存获取
	val, err := cs.client.Get(ctx, key).Result()
	if err == nil {
		// 缓存命中
		return val, nil
	}
	
	// 2. 缓存未命中，检查是否是空值缓存
	if err == redis.Nil {
		// 检查是否存在空值标记
		emptyKey := key + ":empty"
		exists, _ := cs.client.Exists(ctx, emptyKey).Result()
		if exists > 0 {
			// 空值缓存存在，说明之前查询过且数据不存在
			return nil, nil
		}
	}
	
	// 3. 从数据库获取数据
	data, err := fetchFunc()
	if err != nil {
		return nil, err
	}
	
	// 4. 如果数据不存在，设置空值缓存
	if data == nil {
		// 设置空值标记，过期时间比正常数据短
		cs.client.Set(ctx, key+":empty", "", ttl/2)
		return nil, nil
	}
	
	// 5. 数据存在，设置缓存
	cs.client.Set(ctx, key, data, ttl)
	return data, nil
}

// GetWithCacheBreakdown 防止缓存击穿的获取方法
// key: 缓存键
// ttl: 缓存过期时间
// fetchFunc: 当缓存不存在时，从数据库获取数据的函数
func (cs *CacheService) GetWithCacheBreakdown(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// 1. 尝试从缓存获取
	val, err := cs.client.Get(ctx, key).Result()
	if err == nil {
		// 缓存命中
		return val, nil
	}
	
	// 2. 缓存未命中，尝试获取锁
	lockKey := key + ":lock"
	// 使用SETNX命令尝试获取锁，设置锁的过期时间为10秒
	locked, err := cs.client.SetNX(ctx, lockKey, "1", 10*time.Second).Result()
	if err != nil {
		return nil, err
	}
	
	if !locked {
		// 获取锁失败，说明其他协程正在更新缓存
		// 等待一段时间后重试
		time.Sleep(100 * time.Millisecond)
		return cs.GetWithCacheBreakdown(ctx, key, ttl, fetchFunc)
	}
	
	// 3. 获取锁成功，再次检查缓存（双重检查）
	val, err = cs.client.Get(ctx, key).Result()
	if err == nil {
		// 缓存已更新，释放锁并返回
		cs.client.Del(ctx, lockKey)
		return val, nil
	}
	
	// 4. 从数据库获取数据
	data, err := fetchFunc()
	if err != nil {
		// 释放锁
		cs.client.Del(ctx, lockKey)
		return nil, err
	}
	
	// 5. 设置缓存
	cs.client.Set(ctx, key, data, ttl)
	
	// 6. 释放锁
	cs.client.Del(ctx, lockKey)
	
	return data, nil
}

// GetWithCacheAvalanche 防止缓存雪崩的获取方法
// key: 缓存键
// baseTTL: 基础缓存过期时间
// fetchFunc: 当缓存不存在时，从数据库获取数据的函数
func (cs *CacheService) GetWithCacheAvalanche(ctx context.Context, key string, baseTTL time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// 1. 尝试从缓存获取
	val, err := cs.client.Get(ctx, key).Result()
	if err == nil {
		// 缓存命中
		return val, nil
	}
	
	// 2. 缓存未命中，从数据库获取数据
	data, err := fetchFunc()
	if err != nil {
		return nil, err
	}
	
	// 3. 设置缓存，使用随机过期时间
	// 在基础过期时间的基础上，随机增加或减少最多10%的时间
	// 这样可以避免大量缓存同时过期
	randomFactor := 0.9 + 0.2*rand.Float64() // 0.9到1.1之间的随机数
	randomTTL := time.Duration(float64(baseTTL) * randomFactor)
	
	cs.client.Set(ctx, key, data, randomTTL)
	return data, nil
}

// GetWithAllProtections 综合防止缓存穿透、击穿和雪崩的获取方法
// key: 缓存键
// baseTTL: 基础缓存过期时间
// fetchFunc: 当缓存不存在时，从数据库获取数据的函数
func (cs *CacheService) GetWithAllProtections(ctx context.Context, key string, baseTTL time.Duration, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// 1. 尝试从缓存获取
	val, err := cs.client.Get(ctx, key).Result()
	if err == nil {
		// 缓存命中
		return val, nil
	}
	
	// 2. 缓存未命中，检查是否是空值缓存
	if err == redis.Nil {
		// 检查是否存在空值标记
		emptyKey := key + ":empty"
		exists, _ := cs.client.Exists(ctx, emptyKey).Result()
		if exists > 0 {
			// 空值缓存存在，说明之前查询过且数据不存在
			return nil, nil
		}
	}
	
	// 3. 尝试获取锁
	lockKey := key + ":lock"
	// 使用SETNX命令尝试获取锁，设置锁的过期时间为10秒
	locked, err := cs.client.SetNX(ctx, lockKey, "1", 10*time.Second).Result()
	if err != nil {
		return nil, err
	}
	
	if !locked {
		// 获取锁失败，说明其他协程正在更新缓存
		// 等待一段时间后重试
		time.Sleep(100 * time.Millisecond)
		return cs.GetWithAllProtections(ctx, key, baseTTL, fetchFunc)
	}
	
	// 4. 获取锁成功，再次检查缓存（双重检查）
	val, err = cs.client.Get(ctx, key).Result()
	if err == nil {
		// 缓存已更新，释放锁并返回
		cs.client.Del(ctx, lockKey)
		return val, nil
	}
	
	// 5. 从数据库获取数据
	data, err := fetchFunc()
	if err != nil {
		// 释放锁
		cs.client.Del(ctx, lockKey)
		return nil, err
	}
	
	// 6. 如果数据不存在，设置空值缓存
	if data == nil {
		// 设置空值标记，过期时间比正常数据短
		cs.client.Set(ctx, key+":empty", "", baseTTL/2)
		// 释放锁
		cs.client.Del(ctx, lockKey)
		return nil, nil
	}
	
	// 7. 设置缓存，使用随机过期时间
	randomFactor := 0.9 + 0.2*rand.Float64() // 0.9到1.1之间的随机数
	randomTTL := time.Duration(float64(baseTTL) * randomFactor)
	
	cs.client.Set(ctx, key, data, randomTTL)
	
	// 8. 释放锁
	cs.client.Del(ctx, lockKey)
	
	return data, nil
}
