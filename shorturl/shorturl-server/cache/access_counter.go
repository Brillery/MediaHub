// Package cache 中的访问计数器负责把短链访问次数聚合到 Redis。
//
// 输入是 shorturl-server 已经解析成功的短链表名和短链 ID；输出是 Redis Hash 中对应字段的原子自增结果。
// 状态边界：本模块只维护待落库的增量计数，不读取 MySQL、不决定何时 flush，也不影响短链解析返回。
// 外部依赖：Redis 连接池；Hash key 规则必须与 shorturl-crontab 保持一致。
// 并发边界：HINCRBY 在 Redis 内原子执行，多实例 shorturl-server 可以安全并发累加同一个短链 ID。
package cache

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
	pkgredis "shorturl/pkg/db/redis"
)

const (
	// accessCountRedisKeyBase 是短链访问计数 Redis Hash 的基础 key。
	// 实际 key 会经过 pkgredis.GetKey("access_count", tableName) 变成 shorturl_access_count_<table>；
	// shorturl-crontab 必须使用同一规则扫描和落库。
	accessCountRedisKeyBase = "access_count"
)

// AccessCounter 定义短链访问计数的聚合写入能力。
//
// 调用方只传内部表名和短链 ID，不传用户输入；失败由调用方记录日志并继续返回原始 URL。
type AccessCounter interface {
	// Increment 把指定表的短链 ID 访问次数累加 1。
	// tableName 必须来自内部表名常量，id 必须来自短链 key 解码结果，调用方不能传外部任意字段名。
	Increment(tableName string, id int64) error
	// Destroy 释放计数器持有的外部资源；Redis 实现会把客户端归还连接池。
	Destroy()
}

// AccessCounterFactory 为每次请求创建访问计数器。
//
// 当前生产实现从 Redis 连接池获取客户端，并在 Destroy 时归还。
type AccessCounterFactory interface {
	// NewAccessCounter 创建一次请求内使用的访问计数器实例。
	NewAccessCounter() AccessCounter
}

type redisAccessCounter struct {
	redisClient *redis.Client
	destroy     func()
}

// newRedisAccessCounter 创建 Redis Hash 访问计数器。
func newRedisAccessCounter(client *redis.Client, destroy func()) AccessCounter {
	return &redisAccessCounter{
		redisClient: client,
		destroy:     destroy,
	}
}

// Increment 把指定短链 ID 的访问次数在 Redis 中原子加一。
//
// 这里只写增量，不设置 TTL：crontab 未运行时计数必须保留，避免因为缓存过期丢访问数据。
func (c *redisAccessCounter) Increment(tableName string, id int64) error {
	key := accessCountRedisKey(tableName)
	field := strconv.FormatInt(id, 10)
	return c.redisClient.HIncrBy(context.Background(), key, field, 1).Err()
}

// Destroy 归还 Redis 客户端。
func (c *redisAccessCounter) Destroy() {
	if c.destroy != nil {
		c.destroy()
	}
}

// accessCountRedisKey 生成访问计数 Hash key。
//
// tableName 只能来自内部常量 url_map / url_map_user，不能直接使用外部请求参数。
func accessCountRedisKey(tableName string) string {
	return pkgredis.GetKey(accessCountRedisKeyBase, tableName)
}

type redisAccessCounterFactory struct {
	redisPool pkgredis.RedisPool
}

// NewRedisAccessCounterFactory 创建 Redis 访问计数器工厂。
func NewRedisAccessCounterFactory(redisPool pkgredis.RedisPool) AccessCounterFactory {
	return &redisAccessCounterFactory{redisPool: redisPool}
}

// NewAccessCounter 从连接池获取 Redis 客户端并包装为访问计数器。
func (f *redisAccessCounterFactory) NewAccessCounter() AccessCounter {
	client := f.redisPool.Get()
	return newRedisAccessCounter(client, func() {
		f.redisPool.Put(client)
	})
}
