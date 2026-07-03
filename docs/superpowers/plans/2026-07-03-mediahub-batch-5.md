# MediaHub Batch 5 ShortURL Access Count Aggregation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把短链解析路径上的每次访问同步写 MySQL，改为 Redis 聚合计数加定时批量落库，降低高频短链访问对 `url_map.times` / `url_map_user.times` 的写放大。

**Architecture:** `shorturl-server` 仍负责短链解析和返回原始 URL，但访问次数只写 Redis Hash 计数器；`shorturl-crontab` 负责抢分布式 flush 锁、扫描 Redis 计数快照、批量增量写回 MySQL。短链 gRPC 协议、proxy HTTP 路由、mediahub 上传协议都不变。

**Concurrency Boundary:** Redis Hash 字段以短链 ID 为维度累加，key 以表名区分 public/user 两张表。crontab 刷新时先获取 Redis 锁，避免多实例重复落库；每条记录写库成功后只从 Redis 中减去本次快照值，期间新增访问会保留为剩余值，避免并发访问计数丢失。

**Tech Stack:** Go、go-redis/v9、MySQL、robfig/cron、existing shorturl data layer.

---

### Task 1: Plan And Impact Map

**Files:**
- Create: `docs/superpowers/plans/2026-07-03-mediahub-batch-5.md`

- [ ] **Step 1: Document current bottleneck**
  Record that `GetOriginalUrl` currently calls `IncrementTimes` synchronously after every successful resolution.

- [ ] **Step 2: Define aggregation contract**
  Use Redis key `shorturl_access_count_<table>` and hash field `<id>` so `shorturl-server` and `shorturl-crontab` share the same stable contract.

- [ ] **Step 3: Commit**
  Commit with `docs: 规划 MediaHub 第五批重构`.

### Task 2: Record Access Counts To Redis

**Files:**
- Create: `shorturl/shorturl-server/cache/access_counter.go`
- Modify: `shorturl/shorturl-server/server/server.go`
- Modify: `shorturl/shorturl-server/server/server_test.go`
- Modify: `shorturl/shorturl-server/main.go`

- [ ] **Step 1: Add failing service tests**
  Cover successful shortlink resolution recording Redis access count and access-counter failure not affecting redirects.

- [ ] **Step 2: Add access counter abstraction**
  Add `AccessCounter` / `AccessCounterFactory` so service logic does not depend on concrete Redis client calls.

- [ ] **Step 3: Replace synchronous MySQL increment**
  Change `GetOriginalUrl` to call the access counter after successful URL resolution and only log failures.

- [ ] **Step 4: Wire production factory**
  Construct Redis-backed access counter in `shorturl-server/main.go` from the existing Redis pool.

- [ ] **Step 5: Verify and commit**
  Run `go test ./shorturl-server/server ./shorturl-server/cache`, then commit with `perf: 聚合记录短链访问次数`.

### Task 3: Flush Access Counts From Crontab

**Files:**
- Modify: `shorturl-crontab/cron/crontab.go`
- Modify: `shorturl-crontab/data/url_map.go`
- Add tests under `shorturl-crontab/cron` and/or `shorturl-crontab/data`

- [ ] **Step 1: Add flush data method**
  Add `IncrementTimes(tableName string, id int64, incrementTimes int64, now int64)` to crontab data layer.

- [ ] **Step 2: Add Redis flush job**
  Every minute, acquire a Redis lock, scan `shorturl_access_count_url_map` and `shorturl_access_count_url_map_user`, write increments to MySQL, then decrement Redis by the flushed snapshot.

- [ ] **Step 3: Preserve max_id job**
  Keep the existing daily `setUrlMapID` behavior unchanged and continue running it at startup.

- [ ] **Step 4: Verify and commit**
  Run `go test ./...` in `shorturl-crontab`, then commit with `feat: 定时落库短链访问计数`.

### Task 4: Final Review And Merge

**Files:**
- Review all files changed in this batch.

- [ ] **Step 1: Run final verification**
  Run `go test ./...` in `mediahub`, `shorturl`, `shorturl-proxy`, and `shorturl-crontab`, then run `npm run build` and `npm audit --omit=dev` in `mediahub-web`.

- [ ] **Step 2: Review residual risks**
  Confirm remaining work: frontend dev dependency upgrades, production SQL unique-index migration execution, and optional access-count observability.

- [ ] **Step 3: Push and merge**
  Push `cz/refactor/2026-07-03-mediahub-batch-5`, merge into `master`, verify `master`, and push `master`.
