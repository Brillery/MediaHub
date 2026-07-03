# MediaHub Batch 3 Reliability Gates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 MediaHub 补上自动化验证门禁，并继续收敛短链生成的多实例重复写入风险。

**Architecture:** 本批不直接修改线上数据库结构。先新增 CI，确保后续每次提交都会跑 Go 测试、前端构建和生产依赖审计；再在短链生成链路按原始 URL 加分布式锁，降低多实例并发重复生成；最后补一份唯一索引迁移草案，明确上线前需要先清理历史重复数据。

**Tech Stack:** GitHub Actions、Go 1.26、Node.js 24、npm、gRPC shorturl service、Redis distributed lock、MySQL schema docs。

---

### Task 1: CI Verification Gates

**Files:**
- Create: `.github/workflows/mediahub-ci.yml`

- [ ] **Step 1: Add workflow**
  Create one workflow that runs on push and pull request. The workflow should run `go test ./...` for `mediahub`, `shorturl`, `shorturl-proxy`, and `shorturl-crontab`; then run `npm ci`, `npm run build`, and `npm audit --omit=dev` in `mediahub-web`.

- [ ] **Step 2: Keep toolchain explicit**
  Use Go `1.26.x` to match the current local verification environment and Node.js `24` to match local frontend verification.

- [ ] **Step 3: Verify syntax locally**
  Review the YAML content and run the same commands locally as the workflow steps.

- [ ] **Step 4: Commit**
  Commit with `ci: 新增 MediaHub 自动验证门禁`.

### Task 2: ShortUrl Creation Lock

**Files:**
- Modify: `shorturl/shorturl-server/server/server.go`
- Modify: `shorturl/shorturl-server/server/server_test.go`

- [ ] **Step 1: Write failing tests**
  Add tests proving `GetShortUrl` reuses an existing mapping that appears after the original URL lock is acquired, and falls back to direct create when the lock backend errors.

- [ ] **Step 2: Add lock helper**
  Add a helper that creates a lock key from public/private scope, user ID, and a SHA-256 digest of the original URL. Do not put the raw URL in Redis lock keys.

- [ ] **Step 3: Re-check under lock**
  After acquiring the lock, re-run `GetByOriginal` before `GenerateID`. This prevents two instances from both observing “missing” and generating duplicate short links.

- [ ] **Step 4: Preserve availability on lock errors**
  If the lock service errors, log a warning and use the previous direct create path. This keeps short link creation available when Redis locking is temporarily degraded.

- [ ] **Step 5: Verify**
  Run `go test ./shorturl-server/server` and `go test ./...` in `shorturl`.

- [ ] **Step 6: Commit**
  Commit with `fix: 收敛短链生成并发重复风险`.

### Task 3: ShortUrl Unique Index Migration Draft

**Files:**
- Create: `sql/migrations/2026-07-03-shorturl-unique-index.sql`
- Modify: `sql/create_db.sql`

- [ ] **Step 1: Add migration draft**
  Add a migration script that first queries duplicates, then shows dedupe review queries, then adds unique indexes. The script must not silently delete rows.

- [ ] **Step 2: Update fresh schema**
  Update `create_db.sql` so new deployments use unique indexes for `short_key` and the correct original URL scope from day one.

- [ ] **Step 3: Commit**
  Commit with `docs: 补充短链唯一索引迁移草案`.

### Task 4: Final Review And Merge

**Files:**
- Review all files changed in this batch.

- [ ] **Step 1: Run final verification**
  Run all CI-equivalent local commands: Go tests for all four modules, frontend build, and production npm audit.

- [ ] **Step 2: Review residual risks**
  Confirm that database unique indexes still require an operator-reviewed migration because historical duplicates may exist.

- [ ] **Step 3: Push and merge**
  Push `cz/refactor/2026-07-03-mediahub-batch-3`, merge into `master`, verify `master`, and push `master`.
