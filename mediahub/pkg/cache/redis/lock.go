// Package redis 的锁辅助逻辑负责缓存击穿保护中的锁归属判断。
//
// 输入：
// - 缓存策略生成的 lockKey 与 lockToken。
// - Cache 接口或实现了 CompareAndDelete 的 RedisCache。
//
// 输出：
// - 当前请求持有锁时释放锁；锁已被其他 worker 接管时保持不动。
//
// 状态边界：
// - 不保存进程内锁状态，锁归属完全以 Redis 中的 token 为准。
// - 不负责业务数据查询、不负责缓存 key 的租户/用户隔离设计。
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// compareAndDeleteCache 表示支持原子 compare-and-delete 的缓存实现。
//
// RedisCache 会用 Lua 在 Redis 内原子完成比较和删除；测试 fake 或其他缓存实现
// 可以选择实现它，未实现时 releaseCacheLock 会退回到非原子但有 token 校验的路径。
type compareAndDeleteCache interface {
	CompareAndDelete(ctx context.Context, key string, expected string) (bool, error)
}

// newCacheLockToken 生成缓存击穿锁的持有者标识。
//
// 这个 token 只用于区分“当前请求持有的锁”和“锁过期后下一轮请求重新获得的锁”，
// 不承载认证语义。随机值生成失败时回退到纳秒时间戳，保证仍然不会使用固定值释放锁。
func newCacheLockToken() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}

	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// releaseCacheLock 只释放当前请求持有的缓存击穿锁。
//
// 多实例场景下锁可能已经过期，并被其他 worker 用新的 token 重新抢到。
// 因此这里必须先比较锁值，再删除；真实 Redis 实现会通过 Lua 保证比较和删除原子完成。
func releaseCacheLock(ctx context.Context, cache Cache, lockKey string, lockToken string) error {
	if releaser, ok := cache.(compareAndDeleteCache); ok {
		_, err := releaser.CompareAndDelete(ctx, lockKey, lockToken)
		return err
	}

	value, err := cache.Get(ctx, lockKey)
	if err != nil {
		return err
	}
	if value != lockToken {
		return nil
	}

	return cache.Delete(ctx, lockKey)
}
