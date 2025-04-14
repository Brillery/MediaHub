package cache

import (
	"context"
	"errors"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/redis/go-redis/v9"
	pkgredis "shorturl/pkg/db/redis"
)

// BloomFilter 布隆过滤器接口
type BloomFilter interface {
	// Add 添加元素到布隆过滤器
	Add(key string, value string) error
	// Exists 检查元素是否可能存在于布隆过滤器中
	Exists(key string, value string) (bool, error)
}

// RedisBloomFilter 基于Redis的布隆过滤器实现
type RedisBloomFilter struct {
	redisClient *redis.Client
	destroy     func()
	// 布隆过滤器参数
	filter *bloom.BloomFilter
	key    string // 布隆过滤器在Redis中的键名
}

// NewRedisBloomFilter 创建一个新的Redis布隆过滤器
func NewRedisBloomFilter(client *redis.Client, key string, expectedItems int, errorRate float64, destroy func()) BloomFilter {
	// 使用bits-and-blooms库创建布隆过滤器
	filter := bloom.NewWithEstimates(uint(expectedItems), errorRate)

	return &RedisBloomFilter{
		redisClient: client,
		destroy:     destroy,
		filter:      filter,
		key:         key,
	}
}

// Add 添加元素到布隆过滤器
func (bf *RedisBloomFilter) Add(key string, value string) error {
	// 添加到内存中的布隆过滤器
	bf.filter.AddString(value)

	// 将布隆过滤器的位数组序列化并存储到Redis
	bits, err := bf.filter.MarshalBinary()
	if err != nil {
		return err
	}

	// 存储到Redis
	return bf.redisClient.Set(context.Background(), bf.key, bits, 0).Err()
}

// Exists 检查元素是否可能存在于布隆过滤器中
func (bf *RedisBloomFilter) Exists(key string, value string) (bool, error) {
	// 从Redis获取布隆过滤器的位数组
	bits, err := bf.redisClient.Get(context.Background(), bf.key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 如果布隆过滤器不存在，则元素一定不存在
			return false, nil
		}
		return false, err
	}

	// 反序列化布隆过滤器
	if err := bf.filter.UnmarshalBinary(bits); err != nil {
		return false, err
	}

	// 检查元素是否可能存在
	return bf.filter.TestString(value), nil
}

// BloomFilterFactory 布隆过滤器工厂接口
type BloomFilterFactory interface {
	// NewBloomFilter 创建一个新的布隆过滤器实例
	NewBloomFilter(key string, expectedItems int, errorRate float64) BloomFilter
}

// RedisBloomFilterFactory 基于Redis的布隆过滤器工厂
type RedisBloomFilterFactory struct {
	redisPool pkgredis.RedisPool
}

// NewRedisBloomFilterFactory 创建一个新的Redis布隆过滤器工厂
func NewRedisBloomFilterFactory(redisPool pkgredis.RedisPool) BloomFilterFactory {
	return &RedisBloomFilterFactory{
		redisPool: redisPool,
	}
}

// NewBloomFilter 创建一个新的布隆过滤器实例
func (f *RedisBloomFilterFactory) NewBloomFilter(key string, expectedItems int, errorRate float64) BloomFilter {
	client := f.redisPool.Get()
	return NewRedisBloomFilter(client, key, expectedItems, errorRate, func() {
		f.redisPool.Put(client)
	})
}
