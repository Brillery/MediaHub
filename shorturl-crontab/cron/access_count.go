// Package cron 中的访问计数 flush 任务负责把 Redis 聚合计数批量落回 MySQL。
//
// 输入是 shorturl-server 写入的 Redis Hash：shorturl_access_count_url_map 和
// shorturl_access_count_url_map_user；输出是 url_map / url_map_user 表的 times 增量更新。
// 状态边界：本模块只搬运“待落库增量”，不解析短链、不生成短链、不维护 max_id 缓存。
// 外部依赖：Redis 用于保存待 flush 计数和分布式锁，MySQL 用于持久化 times。
// 并发边界：多实例 crontab 必须先抢 Redis 锁；写库成功后只从 Redis 扣减本次快照值，
// flush 期间新增的访问会作为剩余值保留，避免并发访问计数丢失。
package cron

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"shorturl-crontab/data"
	"shorturl-crontab/pkg/db/mysql"
	pkgredis "shorturl-crontab/pkg/db/redis"
	"shorturl-crontab/pkg/log"
)

const (
	// accessCountRedisKeyBase 必须与 shorturl-server/cache/access_counter.go 保持一致。
	// 实际 Redis key 是 shorturl_access_count_<table>，字段是短链 ID，值是待落库访问次数。
	accessCountRedisKeyBase = "access_count"
	// accessCountFlushLockKeyBase 是 crontab 多实例互斥锁基础 key。
	accessCountFlushLockKeyBase = "access_count_flush_lock"
	// accessCountFlushLockTTL 是单次 flush 的锁过期时间。
	// 如果实例异常退出，锁会自动释放；正常 flush 一般远小于该时间。
	accessCountFlushLockTTL = 5 * time.Minute
	// accessCountScanBatchSize 控制每次 HSCAN 的字段数量，避免一次性 HGETALL 拉取过大 Hash。
	accessCountScanBatchSize int64 = 1000
)

// accessCountCache 抽象 Redis 访问计数操作，便于用 fake 测试 flush 并发语义。
type accessCountCache interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
	Scan(ctx context.Context, key string, cursor uint64, count int64) ([]string, uint64, error)
	Decrement(ctx context.Context, key, field string, delta int64) (int64, error)
	DeleteIfNonPositive(ctx context.Context, key, field string) error
}

// accessCountData 抽象 MySQL 增量写入能力。
type accessCountData interface {
	IncrementTimes(tableName string, id int64, incrementTimes int64, now int64) error
}

// flushAccessCounts 是生产定时任务入口。
//
// 它复用全局 Redis/MySQL 连接池；具体 flush 语义放在 flushAccessCountsWithDeps，保证核心逻辑可测试。
func flushAccessCounts() {
	ctx := context.Background()

	redisPool := pkgredis.GetPool()
	client := redisPool.Get()
	defer redisPool.Put(client)

	db := data.NewData(mysql.GetDB())
	cache := &redisAccessCountCache{client: client}
	if err := flushAccessCountsWithDeps(ctx, cache, db, urlMapTables, time.Now().Unix()); err != nil {
		log.Error(err)
	}
}

// flushAccessCountsWithDeps 抢锁后按表 flush 访问计数。
//
// 拿不到锁说明已有另一个 crontab 实例在处理，本实例直接跳过；拿到锁后即使某张表失败，
// 也会继续尝试其他表，并返回第一个错误供生产入口记录。
func flushAccessCountsWithDeps(ctx context.Context, cache accessCountCache, db accessCountData, tables []string, now int64) error {
	lockKey := accessCountFlushLockKey()
	locked, err := cache.TryLock(ctx, lockKey, accessCountFlushLockTTL)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() {
		if err := cache.Unlock(ctx, lockKey); err != nil {
			log.Error(err)
		}
	}()

	var firstErr error
	for _, tableName := range tables {
		if err := flushAccessCountsForTable(ctx, cache, db, tableName, now); err != nil {
			log.Error(err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// flushAccessCountsForTable 扫描单张短链表的 Redis 计数并落库。
//
// 对每个 Hash 字段先写 MySQL，再从 Redis 扣减“扫描时看到的 count”。
// 如果写库失败，不扣 Redis，下一轮会重试，宁可重复保留也不能丢统计。
func flushAccessCountsForTable(ctx context.Context, cache accessCountCache, db accessCountData, tableName string, now int64) error {
	key := accessCountRedisKey(tableName)
	var cursor uint64
	for {
		entries, nextCursor, err := cache.Scan(ctx, key, cursor, accessCountScanBatchSize)
		if err != nil {
			return err
		}
		if len(entries)%2 != 0 {
			return fmt.Errorf("redis access count scan returned odd field/value length for %s", tableName)
		}

		for i := 0; i < len(entries); i += 2 {
			field := entries[i]
			id, count, err := parseAccessCountEntry(field, entries[i+1])
			if err != nil {
				log.Warning("跳过非法短链访问计数字段: " + err.Error())
				continue
			}
			if count <= 0 {
				if err := cache.DeleteIfNonPositive(ctx, key, field); err != nil {
					return err
				}
				continue
			}

			if err := db.IncrementTimes(tableName, id, count, now); err != nil {
				return err
			}
			remaining, err := cache.Decrement(ctx, key, field, count)
			if err != nil {
				return err
			}
			if remaining <= 0 {
				if err := cache.DeleteIfNonPositive(ctx, key, field); err != nil {
					return err
				}
			}
		}

		if nextCursor == 0 {
			return nil
		}
		cursor = nextCursor
	}
}

// parseAccessCountEntry 解析 Redis Hash 的短链 ID 和计数。
func parseAccessCountEntry(field string, value string) (int64, int64, error) {
	id, err := strconv.ParseInt(field, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid id field %q: %w", field, err)
	}
	count, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid count value %q for id %s: %w", value, field, err)
	}
	return id, count, nil
}

// accessCountRedisKey 生成与 shorturl-server 一致的访问计数 Hash key。
func accessCountRedisKey(tableName string) string {
	return pkgredis.GetKey(accessCountRedisKeyBase, tableName)
}

// accessCountFlushLockKey 生成 crontab flush 互斥锁 key。
func accessCountFlushLockKey() string {
	return pkgredis.GetKey(accessCountFlushLockKeyBase)
}

type redisAccessCountCache struct {
	client *goredis.Client
}

// TryLock 用 SETNX 获取 flush 互斥锁。
func (c *redisAccessCountCache) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, "1", ttl).Result()
}

// Unlock 释放 flush 互斥锁。
func (c *redisAccessCountCache) Unlock(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Scan 分批扫描访问计数 Hash，返回 field/value 交错数组。
func (c *redisAccessCountCache) Scan(ctx context.Context, key string, cursor uint64, count int64) ([]string, uint64, error) {
	return c.client.HScan(ctx, key, cursor, "", count).Result()
}

// Decrement 从 Redis Hash 中扣减已经成功落库的快照计数。
func (c *redisAccessCountCache) Decrement(ctx context.Context, key, field string, delta int64) (int64, error) {
	return c.client.HIncrBy(ctx, key, field, -delta).Result()
}

// DeleteIfNonPositive 原子删除当前值小于等于 0 的字段。
//
// 删除必须在 Redis 内判断当前值，避免 flush 扣到 0 后、短链服务刚新增访问又被 HDEL 误删。
func (c *redisAccessCountCache) DeleteIfNonPositive(ctx context.Context, key, field string) error {
	const script = `
local value = redis.call("HGET", KEYS[1], ARGV[1])
if not value then
  return 0
end
local number = tonumber(value)
if not number or number <= 0 then
  return redis.call("HDEL", KEYS[1], ARGV[1])
end
return 0
`
	return c.client.Eval(ctx, script, []string{key}, field).Err()
}
