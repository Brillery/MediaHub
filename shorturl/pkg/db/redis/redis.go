package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"shorturl/pkg/config"
	"sync"
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
