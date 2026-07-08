// Package redis 的锁辅助逻辑负责 CacheService 中缓存击穿锁的安全释放。
//
// 输入：
// - CacheService 生成的 lockKey 与 lockToken。
// - go-redis Client。
//
// 输出：
// - 当前请求持有锁时释放锁；锁已过期并被下一轮请求接管时保持不动。
//
// 状态边界：
// - 不依赖进程内锁或本机内存状态，适配多实例部署。
// - 不负责业务缓存值写入、空值缓存语义或数据库查询。
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// compareAndDeleteScript 是 Redis 锁释放脚本。
//
// KEYS[1] 是锁 key，ARGV[1] 是当前请求持有的 token；只有二者匹配时才删除，
// 保证 GET 与 DEL 在 Redis 单条脚本内原子执行。
const compareAndDeleteScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

// newRedisLockToken 生成 CacheService 击穿保护锁的持有者标识。
//
// token 只用于释放锁时确认归属，不参与业务鉴权。使用随机值是为了避免多实例、
// 多协程都写入固定 `"1"` 后，旧请求在锁过期后误删新请求的锁。
func newRedisLockToken() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}

	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// releaseRedisLock 只释放当前 token 持有的 Redis 锁。
//
// Redis 锁可能因为业务查询慢而先过期，再被下一轮请求重新抢到。这里通过 Lua
// 把 GET 和 DEL 放在 Redis 单条脚本里执行，保证比较和删除是原子的。
func releaseRedisLock(ctx context.Context, client *goredis.Client, lockKey string, lockToken string) error {
	return client.Eval(ctx, compareAndDeleteScript, []string{lockKey}, lockToken).Err()
}
