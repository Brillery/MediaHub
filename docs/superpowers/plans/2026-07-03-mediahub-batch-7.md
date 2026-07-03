# MediaHub Web Chunk Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 降低 `mediahub-web` 首屏第三方 JS chunk 体积，消除 Element Plus 单块超过 500 kB 的构建告警。

**Architecture:** 只在前端构建边界内处理包体问题：入口文件使用 Element Plus ESM 组件子路径导入，Vite 只按真实依赖路径做稳定 vendor 分包，不再通过 `element-plus` 聚合入口人为制造大块。业务页面、接口协议、上传/复制行为保持不变。

**Tech Stack:** Vue 3、Vite 6、Element Plus、TypeScript、npm。

---

### Task 1: 记录基线与影响面

**Files:**
- Create: `docs/superpowers/plans/2026-07-03-mediahub-batch-7.md`

- [x] **Step 1: 确认当前构建基线**

Run:

```bash
cd mediahub-web && npm ci && npm run build
```

Expected:
- 构建成功。
- `elementPlus-*.js` 约 921 kB，并触发 chunk size warning。

- [x] **Step 2: 明确本批边界**

本批只处理：
- `mediahub-web/src/main.ts` 的 Element Plus 组件导入入口。
- `mediahub-web/src/request/axios.ts` 和 `mediahub-web/src/views/components/upload.vue` 的 Element Plus 服务 API 导入入口。
- `mediahub-web/vite.config.ts` 的 vendor 分包策略。

本批不处理：
- 不修改上传接口、短链服务、后端数据库、权限、租户或缓存。
- 不引入路由懒加载，因为当前页面只有一个首页入口，拆路由收益不稳定。

### Task 2: 改为 Element Plus 子路径按需导入

**Files:**
- Modify: `mediahub-web/src/main.ts`
- Modify: `mediahub-web/src/request/axios.ts`
- Modify: `mediahub-web/src/views/components/upload.vue`

- [x] **Step 1: 替换组件根入口导入**

在 `src/main.ts` 中把 `element-plus` 根入口改为组件子路径：

```ts
import { ElAvatar } from 'element-plus/es/components/avatar/index'
import { ElButton } from 'element-plus/es/components/button/index'
import { ElCarousel, ElCarouselItem } from 'element-plus/es/components/carousel/index'
import { ElMenu, ElMenuItem, ElSubMenu } from 'element-plus/es/components/menu/index'
```

- [x] **Step 2: 替换服务 API 根入口导入**

在 axios 拦截器中使用：

```ts
import { ElMessage } from 'element-plus/es/components/message/index'
```

在上传组件中使用：

```ts
import { ElLoading } from 'element-plus/es/components/loading/index'
import { ElMessage } from 'element-plus/es/components/message/index'
```

- [x] **Step 3: 保留现有 CSS 显式引入**

`src/main.ts` 中继续保留 `base.css`、组件 CSS、`el-message.css`、`el-loading.css`。这些样式是服务 API 运行时弹层展示的边界，不能因为 JS 按需导入而删掉。

### Task 3: 收紧 Vite vendor 分包策略

**Files:**
- Modify: `mediahub-web/vite.config.ts`

- [x] **Step 1: 删除 `element-plus` 聚合入口分包**

不要继续写：

```ts
manualChunks: {
  elementPlus: ['element-plus'],
}
```

这个配置会把包的根入口当成稳定 chunk，容易把未使用的导出也拉到同一分析边界。

- [x] **Step 2: 改成基于模块路径的函数分包**

使用函数式 `manualChunks`：

```ts
manualChunks(id) {
  if (!id.includes('/node_modules/')) return undefined
  if (id.includes('/node_modules/vue/')) return 'vue'
  if (id.includes('/node_modules/axios/')) return 'axios'
  if (id.includes('/node_modules/element-plus/')) return 'elementPlus'
  return 'vendor'
}
```

这样只把真实进入依赖图的 Element Plus 模块聚到稳定缓存块中，不再主动声明根包入口。

### Task 4: 验证与提交

**Files:**
- Modify: `mediahub-web/src/main.ts`
- Modify: `mediahub-web/src/request/axios.ts`
- Modify: `mediahub-web/src/views/components/upload.vue`
- Modify: `mediahub-web/vite.config.ts`

- [x] **Step 1: 构建验证**

Run:

```bash
cd mediahub-web && npm run build
```

Expected:
- 构建成功。
- 不再出现 `Some chunks are larger than 500 kB after minification`。
- 实际输出中最大 JS chunk 为 `elementPlus` 约 85 kB，`vendor` 约 53 kB，`vue` 约 77 kB。

- [x] **Step 2: 安全审计**

Run:

```bash
cd mediahub-web && npm audit && npm audit --omit=dev
```

Expected:
- 两个命令都输出 `found 0 vulnerabilities`。

- [x] **Step 3: 提交**

```bash
git add docs/superpowers/plans/2026-07-03-mediahub-batch-7.md
git commit -m "docs: 规划 MediaHub 第七批重构"
git add mediahub-web/src/main.ts mediahub-web/src/request/axios.ts mediahub-web/src/views/components/upload.vue mediahub-web/vite.config.ts
git commit -m "perf: 优化前端 Element Plus 分包"
```
