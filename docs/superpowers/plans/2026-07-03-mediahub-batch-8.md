# Shorturl Counter Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复短链 Redis 锁释放的归属边界，并补齐访问计数定时落库的可观测统计。

**Architecture:** 本批只触碰 `shorturl` 和 `shorturl-crontab` 内部实现，不改 gRPC 协议、不改前端、不改短链表结构。Redis 锁 token 必须由当前进程实例本地持有，释放时通过 Lua 对比 token 后删除；访问计数 flush 继续使用 Redis Hash 快照落库，再扣减同一快照值，并输出表级统计。

**Tech Stack:** Go、Redis、MySQL、robfig/cron、logrus。

---

### Task 1: 修复 shorturl-server 分布式锁归属

**Files:**
- Modify: `shorturl/shorturl-server/cache/lock.go`
- Test: `shorturl/shorturl-server/cache/lock_test.go`

- [x] **Step 1: 把锁 token 放回本地实例**

`RedisDistributedLock` 增加本地 `ownerTokens map[string]string`，`Lock` 成功后记录 `key -> token`，不再写 Redis `key_owner`。释放时只拿本地 token 和 Redis key 当前值做 Lua 比对，避免旧实例读取到新实例 owner 后误删新锁。

- [x] **Step 2: 使用加密随机 token**

`generateRandomValue` 改为 `crypto/rand` 生成 16 字节随机值并转 hex；如果随机源异常，再返回时间戳兜底 token。锁 token 是释放校验用的归属标识，不承载业务含义。

- [x] **Step 3: 增加锁归属测试**

新增测试覆盖：

```go
func TestRedisDistributedLockUnlockOnlyDeletesOwnedToken(t *testing.T)
func TestRedisDistributedLockUnlockKeepsNewOwnerAfterTTLReacquire(t *testing.T)
```

Expected:
- 当前实例释放自己持有的 token 时删除 Redis key。
- Redis key 被新 token 覆盖后，旧实例 `Unlock` 不删除新 token。

### Task 2: 修复 crontab flush 锁归属

**Files:**
- Modify: `shorturl-crontab/cron/access_count.go`
- Test: `shorturl-crontab/cron/access_count_test.go`

- [x] **Step 1: 给 flush 锁增加 owner token**

`accessCountCache.TryLock` 改为接收 `ownerToken`，Redis 实现使用 `SETNX key ownerToken TTL`。`Unlock` 改为接收同一个 token，并使用 Lua 脚本原子校验当前值后删除。

- [x] **Step 2: 测试释放 token 传递**

fake cache 记录 `TryLock` 和 `Unlock` 的 token，确保 flush 释放的是当前实例申请锁时的 token。

### Task 3: 补访问计数 flush 统计

**Files:**
- Modify: `shorturl-crontab/cron/access_count.go`
- Test: `shorturl-crontab/cron/access_count_test.go`

- [x] **Step 1: 新增统计类型**

新增 `accessCountFlushStats` 和 `accessCountTableFlushStats`：

```go
type accessCountFlushStats struct {
    LockAcquired bool
    Tables []accessCountTableFlushStats
}

type accessCountTableFlushStats struct {
    TableName string
    ScannedEntries int
    FlushedRows int
    FlushedCount int64
    InvalidEntries int
    NonPositiveEntries int
}
```

- [x] **Step 2: flush 返回统计**

`flushAccessCountsWithDeps` 返回 `(accessCountFlushStats, error)`，拿不到锁时返回 `LockAcquired=false`；每张表 flush 后追加表级统计，失败时保留已扫描统计并返回第一个错误。

- [x] **Step 3: 生产入口记录结构化摘要**

`flushAccessCounts` 调用后统一输出：
- 拿不到锁：记录已有实例在 flush。
- 拿到锁：记录每张表的扫描条目数、成功落库行数、成功落库总增量、非法字段数、非正数字段清理数。

### Task 4: 完善迁移说明和验证

**Files:**
- Modify: `sql/migrations/2026-07-03-shorturl-unique-index.sql`

- [x] **Step 1: 把迁移从草案整理成正式脚本说明**

说明上线顺序、重复数据自检、DDL 执行边界和回滚语句。脚本仍不自动删除重复数据，避免误删线上短链。

- [x] **Step 2: 验证**

Run:

```bash
cd shorturl && go test ./...
cd ../shorturl-crontab && go test ./...
```

Expected:
- 两个模块测试全部通过。

- [ ] **Step 3: 提交**

```bash
git add docs/superpowers/plans/2026-07-03-mediahub-batch-8.md
git commit -m "docs: 规划 MediaHub 第八批重构"
git add shorturl shorturl-crontab sql/migrations/2026-07-03-shorturl-unique-index.sql docs/superpowers/plans/2026-07-03-mediahub-batch-8.md
git commit -m "fix: 收紧短链 Redis 锁归属"
```
