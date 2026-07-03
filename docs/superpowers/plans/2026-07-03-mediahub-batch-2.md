# MediaHub Batch 2 Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在第一批构建和基础安全修复后，继续收敛 MediaHub 的前端包体、上传图片元数据和短链缓存回源边界。

**Architecture:** 本批仍不做大目录迁移，选择可独立回滚的三个切片。前端只调整依赖引入和构建分包；上传链路只规范图片格式识别和对象存储元数据；短链服务只抽取缓存回源语义并修复空值缓存、max_id 缓存缺失时的降级边界。

**Tech Stack:** Go 1.26、Gin、gRPC、MySQL、Redis cache interface、Vue 3、Vite、Element Plus。

---

### Task 1: Frontend Bundle Split

**Files:**
- Modify: `mediahub-web/src/main.ts`
- Modify: `mediahub-web/vite.config.ts`

- [ ] **Step 1: Inspect current bundle**
  Run `npm run build` in `mediahub-web`; expected pass with a large chunk warning.

- [ ] **Step 2: Replace full Element Plus registration**
  Register only the Element Plus components used by the current app: `ElButton`, `ElCarousel`, and `ElCarouselItem`. Keep service APIs such as `ElMessage` and `ElLoading` imported in the components that call them.

- [ ] **Step 3: Split vendor chunks**
  Add Vite `manualChunks` for `vue`, `element-plus`, and `axios` so browser cache can reuse stable vendor code.

- [ ] **Step 4: Verify**
  Run `npm run build` in `mediahub-web`; expected pass and the previous single large app chunk should be split into smaller assets.

- [ ] **Step 5: Commit**
  Commit with `perf: 拆分 mediahub-web 前端依赖包`.

### Task 2: Upload Image Metadata Boundary

**Files:**
- Modify: `mediahub/controller/file.go`
- Modify: `mediahub/controller/file_test.go`
- Modify: `mediahub/pkg/storage/cos/cos.go`
- Modify: `mediahub/pkg/storage/stroage.go`

- [ ] **Step 1: Write failing tests**
  Add tests proving a valid JPEG uploaded with a misleading extension is stored with a canonical `.jpg` path and an invalid storage extension falls back to `application/octet-stream`.

- [ ] **Step 2: Implement format detection**
  Replace boolean image validation with a helper that returns canonical image metadata. Use decoded content format, not user-provided filename extension, when building object keys.

- [ ] **Step 3: Harden COS content type**
  Use a whitelist map for object content type. Unknown extensions must not become arbitrary `image/<ext>` metadata.

- [ ] **Step 4: Verify**
  Run `go test ./controller ./pkg/storage/cos` in `mediahub`; expected pass.

- [ ] **Step 5: Commit**
  Commit with `fix: 规范上传图片格式和对象元数据`.

### Task 3: ShortUrl Cache Miss Semantics

**Files:**
- Modify: `shorturl/shorturl-server/server/server.go`
- Create: `shorturl/shorturl-server/server/server_test.go`

- [ ] **Step 1: Write failing tests**
  Add tests for three cases: cached empty sentinel returns not found without DB access, missing `max_id` cache falls back to DB instead of rejecting valid IDs, and DB miss writes a short-lived empty sentinel.

- [ ] **Step 2: Extract cache resolve helper**
  Move duplicate lock/no-lock DB fallback logic into one helper that returns original URL or a typed not-found error.

- [ ] **Step 3: Fix negative cache**
  Store a non-URL sentinel instead of empty string so the cache can distinguish “known missing” from “cache miss”.

- [ ] **Step 4: Verify**
  Run `go test ./shorturl-server/server` and then `go test ./...` in `shorturl`; expected pass.

- [ ] **Step 5: Commit**
  Commit with `fix: 收敛短链回源和空值缓存语义`.

### Task 4: Final Review And Merge

**Files:**
- Review all files changed in this batch.

- [ ] **Step 1: Run final verification**
  Run `go test ./...` in `mediahub`, `shorturl`, `shorturl-proxy`, and `shorturl-crontab`, then run `npm run build` and `npm audit --omit=dev` in `mediahub-web`.

- [ ] **Step 2: Review residual risks**
  Confirm remaining work: Go toolchain upgrade, CI security gates, deeper upload streaming, and optional shortlink schema/index migration.

- [ ] **Step 3: Push and merge**
  Push `cz/refactor/2026-07-03-mediahub-batch-2`, merge into `master`, verify `master`, and push `master`.
