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
	"crypto/rand"
	"encoding/hex"
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
	TryLock(ctx context.Context, key string, ownerToken string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string, ownerToken string) error
	Scan(ctx context.Context, key string, cursor uint64, count int64) ([]string, uint64, error)
	Decrement(ctx context.Context, key, field string, delta int64) (int64, error)
	DeleteIfNonPositive(ctx context.Context, key, field string) error
}

// accessCountData 抽象 MySQL 增量写入能力。
type accessCountData interface {
	IncrementTimes(tableName string, id int64, incrementTimes int64, now int64) error
}

// accessCountFlushStats 描述一次访问计数 flush 的整体结果。
//
// LockAcquired=false 表示当前实例没有拿到分布式锁，说明已有其他 crontab 实例在执行；
// Tables 只包含当前实例实际处理过的表，用于生产日志和测试校验。
type accessCountFlushStats struct {
	LockAcquired bool
	Tables       []accessCountTableFlushStats
}

// accessCountTableFlushStats 是单张短链表的访问计数落库统计。
//
// ScannedEntries 是 Redis Hash 快照中扫描到的字段数；FlushedRows/FlushedCount 是成功写入 MySQL 的行数和访问增量。
// InvalidEntries 与 NonPositiveEntries 不会写库，前者保留日志证据，后者会尝试清理 Redis 中的非正数字段。
type accessCountTableFlushStats struct {
	TableName          string
	ScannedEntries     int
	FlushedRows        int
	FlushedCount       int64
	InvalidEntries     int
	NonPositiveEntries int
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
	stats, err := flushAccessCountsWithDeps(ctx, cache, db, urlMapTables, time.Now().Unix())
	logAccessCountFlushStats(stats)
	if err != nil {
		log.Error(err)
	}
}

// flushAccessCountsWithDeps 抢锁后按表 flush 访问计数。
//
// 拿不到锁说明已有另一个 crontab 实例在处理，本实例直接跳过；拿到锁后即使某张表失败，
// 也会继续尝试其他表，并返回第一个错误供生产入口记录。
func flushAccessCountsWithDeps(ctx context.Context, cache accessCountCache, db accessCountData, tables []string, now int64) (accessCountFlushStats, error) {
	stats := accessCountFlushStats{}
	lockKey := accessCountFlushLockKey()
	ownerToken := newAccessCountFlushLockToken()
	locked, err := cache.TryLock(ctx, lockKey, ownerToken, accessCountFlushLockTTL)
	if err != nil {
		return stats, err
	}
	stats.LockAcquired = locked
	if !locked {
		return stats, nil
	}
	defer func() {
		if err := cache.Unlock(ctx, lockKey, ownerToken); err != nil {
			log.Error(err)
		}
	}()

	var firstErr error
	for _, tableName := range tables {
		tableStats, err := flushAccessCountsForTable(ctx, cache, db, tableName, now)
		stats.Tables = append(stats.Tables, tableStats)
		if err != nil {
			log.Error(err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return stats, firstErr
}

// flushAccessCountsForTable 扫描单张短链表的 Redis 计数并落库。
//
// 对每个 Hash 字段先写 MySQL，再从 Redis 扣减“扫描时看到的 count”。
// 如果写库失败，不扣 Redis，下一轮会重试，宁可重复保留也不能丢统计。
func flushAccessCountsForTable(ctx context.Context, cache accessCountCache, db accessCountData, tableName string, now int64) (accessCountTableFlushStats, error) {
	stats := accessCountTableFlushStats{TableName: tableName}
	key := accessCountRedisKey(tableName)
	var cursor uint64
	for {
		entries, nextCursor, err := cache.Scan(ctx, key, cursor, accessCountScanBatchSize)
		if err != nil {
			return stats, err
		}
		if len(entries)%2 != 0 {
			return stats, fmt.Errorf("redis access count scan returned odd field/value length for %s", tableName)
		}

		for i := 0; i < len(entries); i += 2 {
			field := entries[i]
			stats.ScannedEntries++
			id, count, err := parseAccessCountEntry(field, entries[i+1])
			if err != nil {
				stats.InvalidEntries++
				log.Warning("跳过非法短链访问计数字段: " + err.Error())
				continue
			}
			if count <= 0 {
				stats.NonPositiveEntries++
				if err := cache.DeleteIfNonPositive(ctx, key, field); err != nil {
					return stats, err
				}
				continue
			}

			if err := db.IncrementTimes(tableName, id, count, now); err != nil {
				return stats, err
			}
			stats.FlushedRows++
			stats.FlushedCount += count
			remaining, err := cache.Decrement(ctx, key, field, count)
			if err != nil {
				return stats, err
			}
			if remaining <= 0 {
				if err := cache.DeleteIfNonPositive(ctx, key, field); err != nil {
					return stats, err
				}
			}
		}

		if nextCursor == 0 {
			return stats, nil
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

// newAccessCountFlushLockToken 生成当前 crontab 实例持有 flush 锁的 owner token。
//
// token 只用于 Unlock 时确认“锁仍归当前实例所有”，不进入业务数据；随机源异常时用时间戳兜底。
func newAccessCountFlushLockToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// logAccessCountFlushStats 输出访问计数 flush 摘要。
//
// 这里不直接暴露 Redis value 明细，只输出表级数量，便于线上判断 crontab 是否在持续落库、
// 是否存在非法字段、是否有大量非正数字段需要清理。
func logAccessCountFlushStats(stats accessCountFlushStats) {
	if !stats.LockAcquired {
		log.Info("短链访问计数 flush 已由其他实例处理，当前实例跳过")
		return
	}

	for _, tableStats := range stats.Tables {
		log.InfoF(
			"短链访问计数 flush 完成 table=%s scanned=%d flushed_rows=%d flushed_count=%d invalid=%d non_positive=%d",
			tableStats.TableName,
			tableStats.ScannedEntries,
			tableStats.FlushedRows,
			tableStats.FlushedCount,
			tableStats.InvalidEntries,
			tableStats.NonPositiveEntries,
		)
	}
}

type redisAccessCountCache struct {
	client *goredis.Client
}

// TryLock 用 SETNX 获取 flush 互斥锁。
func (c *redisAccessCountCache) TryLock(ctx context.Context, key string, ownerToken string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, ownerToken, ttl).Result()
}

// Unlock 释放 flush 互斥锁。
//
// 释放前必须校验 token，避免当前实例执行时间超过 TTL 后误删另一个实例新拿到的锁。
func (c *redisAccessCountCache) Unlock(ctx context.Context, key string, ownerToken string) error {
	const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`
	return c.client.Eval(ctx, script, []string{key}, ownerToken).Err()
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
