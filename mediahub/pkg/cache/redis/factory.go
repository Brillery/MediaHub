package redis

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config Redis配置
type Config struct {
	// 连接配置
	Host     string `json:"host"`     // Redis主机地址
	Port     int    `json:"port"`     // Redis端口
	Password string `json:"password"` // Redis密码
	DB       int    `json:"db"`       // Redis数据库编号

	// 连接池配置
	PoolSize     int           `json:"pool_size"`      // 连接池大小
	MinIdleConns int           `json:"min_idle_conns"` // 最小空闲连接数
	MaxConnAge   time.Duration `json:"max_conn_age"`   // 连接最大存活时间
	IdleTimeout  time.Duration `json:"idle_timeout"`   // 空闲连接超时时间

	// 缓存配置
	DefaultTTL time.Duration `json:"default_ttl"` // 默认过期时间
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxConnAge:   time.Hour,
		IdleTimeout:  time.Minute * 10,
		DefaultTTL:   time.Hour,
	}
}

// Factory 缓存工厂
type Factory struct {
	config *Config
	cache  Cache
}

// NewFactory 创建缓存工厂
func NewFactory(config *Config) *Factory {
	if config == nil {
		config = DefaultConfig()
	}
	return &Factory{
		config: config,
	}
}

// NewRedisClient 创建Redis客户端
func (f *Factory) NewRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", f.config.Host, f.config.Port),
		Password:     f.config.Password,
		DB:           f.config.DB,
		PoolSize:     f.config.PoolSize,
		MinIdleConns: f.config.MinIdleConns,
		MaxConnAge:   f.config.MaxConnAge,
		IdleTimeout:  f.config.IdleTimeout,
	})
}

// GetCache 获取缓存实例
func (f *Factory) GetCache() Cache {
	if f.cache == nil {
		client := f.NewRedisClient()
		f.cache = NewRedisCache(client)
	}
	return f.cache
}

// NewStrategy 创建缓存策略
func (f *Factory) NewStrategy(strategyType string) CacheStrategy {
	cache := f.GetCache()
	switch strategyType {
	case "simple":
		return NewSimpleStrategy(cache)
	case "penetration":
		return NewPenetrationStrategy(cache)
	case "breakdown":
		return NewBreakdownStrategy(cache)
	case "avalanche":
		return NewAvalancheStrategy(cache)
	case "all":
		return NewAllProtectionsStrategy(cache)
	default:
		return NewSimpleStrategy(cache)
	}
}

// Close 关闭缓存连接
func (f *Factory) Close() error {
	if f.cache != nil {
		return f.cache.Close()
	}
	return nil
} 