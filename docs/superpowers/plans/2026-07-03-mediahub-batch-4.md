# MediaHub Batch 4 Upload Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 降低 mediahub 图片上传在并发场景下的内存压力，并让成功上传到短链生成的完整路径可单测。

**Architecture:** 本批只改 `mediahub` 上传链路，不改前端协议、对象存储接口或 shorturl gRPC 协议。上传控制器从“整文件读入 `[]byte`”改为“复制到受限临时文件并通过 `Seek` 复用文件流”；短链生成抽成接口，生产实现仍走现有 gRPC 连接池，测试实现用 fake 覆盖成功路径。

**Tech Stack:** Go 1.26、Gin multipart upload、temporary files、MD5 streaming、COS storage adapter、shorturl gRPC client pool。

---

### Task 1: ShortUrl Generator Boundary

**Files:**
- Create: `mediahub/controller/shorturl_generator.go`
- Modify: `mediahub/controller/file.go`
- Modify: `mediahub/controller/file_test.go`

- [ ] **Step 1: Write failing test**
  Add a successful upload test with fake storage and fake shortener. The test should assert the response URL comes from the shortener, storage receives a valid path, and the shortener receives the object storage URL, authenticated user ID, and public/private flag.

- [ ] **Step 2: Add shortener interface**
  Create `ShortURLGenerator` with `Generate(ctx context.Context, originalURL string, userID int64, isPublic bool) (string, error)`.

- [ ] **Step 3: Add production implementation**
  Move the existing gRPC shorturl call into `grpcShortURLGenerator`, using the existing connection pool and bearer token behavior.

- [ ] **Step 4: Inject dependency**
  Extend `Controller` with a `shortener ShortURLGenerator`, keep `NewController` backward compatible by constructing the production shortener internally, and add `NewControllerWithShortener` for tests.

- [ ] **Step 5: Verify**
  Run `go test ./controller`; expected pass after implementation.

- [ ] **Step 6: Commit**
  Commit with `refactor: 抽离上传短链生成接口`.

### Task 2: Temporary File Upload Pipeline

**Files:**
- Modify: `mediahub/controller/file.go`
- Modify: `mediahub/controller/file_test.go`

- [ ] **Step 1: Write failing tests**
  Add tests proving successful upload reads from a seekable temporary file and cleans temporary files from the configured temp directory after request completion.

- [ ] **Step 2: Add upload workspace helpers**
  Add helpers to copy the multipart file into an `os.CreateTemp` file under `os.TempDir`, enforce the existing size limit, and remove the file on return.

- [ ] **Step 3: Stream MD5 and reuse file**
  Compute MD5 by streaming from the temp file, seek back to the beginning for `image.DecodeConfig`, then seek again for storage upload.

- [ ] **Step 4: Preserve existing behavior**
  Keep current response JSON, max body limit, image type whitelist, user path rules, and storage interface unchanged.

- [ ] **Step 5: Verify**
  Run `go test ./controller` and then `go test ./...` in `mediahub`.

- [ ] **Step 6: Commit**
  Commit with `perf: 使用临时文件处理上传图片流`.

### Task 3: Final Review And Merge

**Files:**
- Review all files changed in this batch.

- [ ] **Step 1: Run final verification**
  Run `go test ./...` in `mediahub`, `shorturl`, `shorturl-proxy`, and `shorturl-crontab`, then run `npm run build` and `npm audit --omit=dev` in `mediahub-web`.

- [ ] **Step 2: Review residual risks**
  Confirm remaining work: shorturl access count async aggregation, dev dependency upgrades, and actually running the SQL duplicate checks before applying unique indexes.

- [ ] **Step 3: Push and merge**
  Push `cz/refactor/2026-07-03-mediahub-batch-4`, merge into `master`, verify `master`, and push `master`.
