# MediaHub Baseline Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 MediaHub 当前阻断构建、上传鉴权、安全依赖和短链基础边界问题，让第一阶段代码重新具备可验证基线。

**Architecture:** 本阶段不做大规模目录迁移，先保留现有多模块结构，按最小可上线修复收敛风险。行为修复以测试先行，配置和依赖治理以最小验证为准。

**Tech Stack:** Go 1.26、Gin、gRPC、go-redis、MySQL、Vue 3、Vite、Element Plus、Axios。

---

### Task 1: Git And Repository Hygiene

**Files:**
- Modify: `.gitignore`
- Create: `.env.example`
- Create: `mediahub/dev.config.example.yaml`
- Create: `shorturl/dev.config.example.yaml`
- Create: `shorturl-proxy/dev.config.example.yaml`
- Create: `shorturl-crontab/dev.config.example.yaml`

- [ ] **Step 1: Add ignore rules**
  Add rules for local config, logs, build outputs, archives, node modules, binaries, and local worktrees.

- [ ] **Step 2: Add example config files**
  Provide non-secret example files that document required keys without exposing credentials.

- [ ] **Step 3: Verify**
  Run `git status --short` and confirm only intended hygiene files changed.

- [ ] **Step 4: Commit**
  Commit with `chore: 收敛仓库忽略规则和配置样例`.

### Task 2: MediaHub Backend Build Baseline

**Files:**
- Modify: `mediahub/pkg/grpc_client_pool/client_pool.go`
- Modify: `mediahub/services/shorturl/client.go`
- Modify: `mediahub/pkg/cache/redis/factory.go`
- Modify: `mediahub/pkg/cache/redis/cache.go`
- Modify: `mediahub/pkg/cache/redis/strategy.go`
- Modify: `mediahub/middleware/auth.go`

- [ ] **Step 1: Write failing build check**
  Run `go test ./...` in `mediahub`; expected failure includes `undefined: grpc_client_pool.NewPool` and go-redis removed fields.

- [ ] **Step 2: Fix gRPC pool API mismatch**
  Reuse the fixed-size pool implementation and call `NewClientCusPool` from `NewShortUrlClientPool`.

- [ ] **Step 3: Fix stale Redis options and unused imports**
  Replace removed `MaxConnAge` / `IdleTimeout` with current go-redis v9 fields and remove unused imports.

- [ ] **Step 4: Fix auth log formatting vet issue**
  Use the formatted log method or plain structured text consistently.

- [ ] **Step 5: Verify**
  Run `go test ./...` in `mediahub`; expected pass.

- [ ] **Step 6: Commit**
  Commit with `fix: 修复 mediahub 后端构建基线`.

### Task 3: Upload Auth And Validation Boundary

**Files:**
- Modify: `mediahub/middleware/auth.go`
- Modify: `mediahub/controller/file.go`
- Create: `mediahub/controller/file_test.go`
- Create: `mediahub/middleware/auth_test.go`

- [ ] **Step 1: Write failing tests**
  Add tests proving authenticated context exposes `user_id`, invalid image upload returns without calling storage, and oversize upload is rejected.

- [ ] **Step 2: Verify red**
  Run targeted tests in `mediahub/controller` and `mediahub/middleware`; expected failure on current behavior.

- [ ] **Step 3: Implement minimal fix**
  Use one exported auth context key, enforce request size before reading body, use image header validation, and return immediately after validation errors.

- [ ] **Step 4: Verify green**
  Run targeted tests; expected pass.

- [ ] **Step 5: Commit**
  Commit with `fix: 收紧上传鉴权和图片校验边界`.

### Task 4: ShortUrl Correctness Baseline

**Files:**
- Modify: `shorturl/pkg/utils/base62.go`
- Modify: `shorturl/pkg/utils/base62_test.go`
- Modify: `shorturl/shorturl-server/data/url_map.go`
- Modify: `shorturl/shorturl-server/server/server.go`

- [ ] **Step 1: Write failing tests**
  Add tests for invalid Base62 characters and data not-found semantics.

- [ ] **Step 2: Verify red**
  Run narrow `go test` commands in `shorturl`; expected failure for invalid key handling or not-found behavior.

- [ ] **Step 3: Implement minimal fix**
  Reject invalid Base62 input by returning zero, return nil entity for not-found, and keep cache-miss branches reachable.

- [ ] **Step 4: Verify green**
  Run targeted tests and then `go test ./...` in `shorturl`; expected pass.

- [ ] **Step 5: Commit**
  Commit with `fix: 修复短链解析和未命中语义`.

### Task 5: Dependency And Frontend Baseline

**Files:**
- Modify: `mediahub-web/package.json`
- Modify: `mediahub-web/package-lock.json`
- Modify: Go module files only where necessary.

- [ ] **Step 1: Upgrade safe dependency patches**
  Run `npm audit fix` in `mediahub-web` and update Go module patch versions for known vulnerable direct dependencies.

- [ ] **Step 2: Verify**
  Run `npm run build`, `npm audit --omit=dev`, and targeted Go tests for touched modules.

- [ ] **Step 3: Commit**
  Commit with `chore: 升级 MediaHub 安全补丁依赖`.

### Task 6: Final Review

**Files:**
- Review git diff across all commits.

- [ ] **Step 1: Run final lightweight verification**
  Run `git status --short`, `go test ./...` in touched Go modules, and `npm run build`.

- [ ] **Step 2: Summarize residual risks**
  List anything intentionally deferred, including deeper cache refactor, unique indexes, upload streaming, and frontend chunk splitting.
