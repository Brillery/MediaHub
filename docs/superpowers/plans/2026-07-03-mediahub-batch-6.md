# MediaHub Batch 6 Frontend Dev Dependency Upgrade Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清理 `mediahub-web` 当前 `npm ci` 暴露的 dev 依赖漏洞，保持生产依赖 `npm audit --omit=dev` 为 0，并确保前端生产构建仍可通过。

**Architecture:** 本批只调整 `mediahub-web/package.json` 和 `package-lock.json` 中的开发工具链版本。业务运行时代码、API 路由、后端服务、短链协议和构建产物不纳入提交。

**Impact Map:**
- 触达项目：仅 `mediahub-web`。
- 运行时影响：无直接运行时依赖变更；`axios`、`element-plus`、`vue` 保持原依赖声明。
- 构建影响：升级 Vite / @vitejs/plugin-vue / vue-tsc / TypeScript 后必须通过 `npm run build`。
- 安全影响：目标是消除 Vite、esbuild、vue-template-compiler、vue-tsc 链路的 dev audit 漏洞。
- 角色和权限边界：不涉及登录、token、多租户、数据库或短链访问权限。

---

### Task 1: Document Upgrade Scope

**Files:**
- Create: `docs/superpowers/plans/2026-07-03-mediahub-batch-6.md`

- [ ] **Step 1: Record audit findings**
  Document that the current vulnerable chains are Vite/esbuild and vue-tsc/@vue/language-core/vue-template-compiler.

- [ ] **Step 2: Commit**
  Commit with `docs: 规划 MediaHub 第六批重构`.

### Task 2: Upgrade Frontend Toolchain

**Files:**
- Modify: `mediahub-web/package.json`
- Modify: `mediahub-web/package-lock.json`

- [ ] **Step 1: Upgrade dev dependencies**
  Upgrade `vite`, `@vitejs/plugin-vue`, `vue-tsc`, and `typescript` to versions that satisfy current audit fixes and Node engine constraints.

- [ ] **Step 2: Verify audit and build**
  Run `npm audit`, `npm audit --omit=dev`, and `npm run build`.

- [ ] **Step 3: Commit**
  Commit with `chore: 升级前端构建工具依赖`.

### Task 3: Final Review And Merge

**Files:**
- Review all files changed in this batch.

- [ ] **Step 1: Run final verification**
  Run `go test ./...` in `mediahub`, `shorturl`, `shorturl-proxy`, and `shorturl-crontab`, then run frontend build and audit in `mediahub-web`.

- [ ] **Step 2: Push and merge**
  Push `cz/chore/2026-07-03-mediahub-web-deps`, merge into `master`, verify `master`, and push `master`.
