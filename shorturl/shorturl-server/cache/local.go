// 本模块提供短链进程内缓存和 Redis 缓存组合。输入为 key/value/TTL，输出为缓存值。
// 本地 map 由互斥锁保护，跨实例事实归 Redis/MySQL 维护；本模块不负责鉴权或持久化。
// 只有显式 Set 才知道 TTL，Redis Get 没有剩余寿命信息时不回填本地，避免延长负缓存。
package cache

import (
	"sync"
	"time"
)

// 这里的本地缓存是指运行在服务器端的内存缓存
// 作用是将数据存储在服务端的内存中，以加速数据访问并减少对分布式缓存（如 Redis）或数据库的依赖。

// 什么是本地缓存？
// 本地缓存是指将数据存储在应用程序的内存中，而不是存储在远程的分布式缓存
// 本地缓存是一个典型的二级缓存架构
// 1.本地低延迟
// 2.本地并发高吞吐量，不受网络带宽影响
// 3.减少远程调用，如redis

// 缺点：
// 1.单机限制：本地缓存通常只存在于单个实例中，无法在多个实例之间共享。
// 2.内存占用：如果缓存的数据量较大，可能会占用较多的内存资源。
// 3.数据一致性问题：如果使用了分布式系统，本地缓存可能与远程缓存或数据库中的数据不一致。

// 为了什么设置本地缓存？
// 1.本地缓存可以在一定程度上缓解**缓存雪崩**的问题。
// 2.

// 二级缓存是指在一个系统中同时使用两种不同类型的缓存：
// 1.本地缓存：存储在应用进程的内存中，用于快速访问和减少对远程缓存的依赖。
// 2.分布式缓存（如 Redis）：用于在多个实例之间共享数据，并支持更大的数据量和更高的可用性。

// LocalCache 本地缓存接口
type LocalCache interface {
	// Get 从本地缓存中获取值
	Get(key string) (string, bool)
	// Set 将值存储到本地缓存中
	Set(key string, value string, ttl time.Duration)
	// Delete 从本地缓存中删除值
	Delete(key string)
	// Clear 清空本地缓存
	Clear()
}

// MemoryCache 基于内存的本地缓存实现
type MemoryCache struct {
	cache map[string]cacheItem
	mutex sync.RWMutex
}

// 未来可拓展：基于文件、数据库实现的本地缓存

// cacheItem 本地缓存的缓存项，包含值和过期时间
type cacheItem struct {
	value      string
	expiration time.Time
}

// NewMemoryCache 创建一个新的内存缓存
func NewMemoryCache() LocalCache {
	cache := &MemoryCache{
		cache: make(map[string]cacheItem),
	}

	// 启动清理过期项的goroutine
	go cache.cleanExpiredItems()

	return cache
	// 这里返回 LocalCache 而不是 MemoryCache 的原因如下：
	// 1.接口抽象：通过返回接口类型，可以隐藏具体的实现细节，只暴露公共的行为。
	// 2.解耦与灵活性：返回接口，而不是具体类型，专注于使用缓存的功能，而不需要关心具体实现类，提高代码灵活性，可维护性。
	// 3.支持多态：使用接口返回，可以在未来替换为其他实现了该接口的缓存实现，例如文件/数据库缓存，无需修改代码。
	// 4.go设计哲学：返回接口，遵循“编程到接口而非实现”的原则
}

// Get 返回尚未过期的值；过期项由定时清理统一删除，避免异步删除覆盖并发 Set。
func (c *MemoryCache) Get(key string) (string, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.cache[key]
	if !exists {
		return "", false
	}

	// 检查是否过期
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		// 持读锁时只报告过期，不启动延迟删除；否则可能删掉随后写入的新值。
		return "", false
	}

	return item.value, true
}

// Set 将值存储到本地缓存中
func (c *MemoryCache) Set(key string, value string, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	c.cache[key] = cacheItem{
		value:      value,
		expiration: expiration,
	}
}

// Delete 从本地缓存中删除值
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.cache, key)
}

// Clear 清空本地缓存
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[string]cacheItem)
}

// cleanExpiredItems 定期清理过期的缓存项
func (c *MemoryCache) cleanExpiredItems() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C { // ticker.C 传递即时消息的通道
		c.mutex.Lock()
		now := time.Now()

		for key, item := range c.cache {
			if !item.expiration.IsZero() && now.After(item.expiration) {
				delete(c.cache, key)
			}
		}

		c.mutex.Unlock()
	}
}

// TwoLevelCache 两级缓存实现，结合本地缓存和分布式缓存
type TwoLevelCache struct {
	localCache  LocalCache
	distributed KVCache
	lockFactory DistributedLockFactory
}

// NewTwoLevelCache 创建一个新的两级缓存
func NewTwoLevelCache(localCache LocalCache, distributed KVCache, lockFactory DistributedLockFactory) KVCache {
	return &TwoLevelCache{
		localCache:  localCache,
		distributed: distributed,
		lockFactory: lockFactory,
	}
}

// Get 从两级缓存中获取值
func (c *TwoLevelCache) Get(key string) (string, error) {
	// 先从本地缓存中获取
	if value, exists := c.localCache.Get(key); exists {
		return value, nil
	}

	// 本地缓存未命中，从分布式缓存中获取
	value, err := c.distributed.Get(key)
	if err != nil {
		return "", err
	}

	// KVCache.Get 不携带剩余 TTL，不能把即将失效的值重新保存 30 天。
	// 本实例写入的热点仍走 L1；跨实例读取以 Redis 的有效期为准。

	return value, nil
}

// Set 将值存储到两级缓存中
func (c *TwoLevelCache) Set(key, value string, ttl int) error {
	// 先存储到分布式缓存
	err := c.distributed.Set(key, value, ttl)
	if err != nil {
		return err
	}

	// TTL 已由业务层添加抖动；本地不能再次放大，也不能对 1 秒 TTL 调用 Intn(0)。
	if ttl > 0 {
		c.localCache.Set(key, value, time.Duration(ttl)*time.Second)
	} else {
		c.localCache.Delete(key)
	}

	return nil
}

// Destroy 释放资源
func (c *TwoLevelCache) Destroy() {
	c.distributed.Destroy()
}

// TwoLevelCacheFactory 两级缓存工厂
type TwoLevelCacheFactory struct {
	localCache  LocalCache
	distributed CacheFactory
	lockFactory DistributedLockFactory
}

// NewTwoLevelCacheFactory 创建一个新的两级缓存工厂
func NewTwoLevelCacheFactory(localCache LocalCache, distributed CacheFactory, lockFactory DistributedLockFactory) CacheFactory {
	return &TwoLevelCacheFactory{
		localCache:  localCache,
		distributed: distributed,
		lockFactory: lockFactory,
	}
}

// NewKVCache 创建一个新的两级缓存实例
func (f *TwoLevelCacheFactory) NewKVCache() KVCache {
	distributed := f.distributed.NewKVCache()
	return NewTwoLevelCache(f.localCache, distributed, f.lockFactory)
}
