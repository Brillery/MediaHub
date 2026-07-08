import { cp, mkdir, rm, stat } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const webRoot = path.resolve(scriptDir, '..')
const distDir = path.resolve(webRoot, 'dist')
const embeddedWwwDir = path.resolve(webRoot, '..', 'mediahub', 'www')

/**
 * 将 Vite 构建结果同步到 Go 服务内置的静态目录。
 *
 * MediaHub 当前有两种前端交付路径：
 * - `mediahub-web/dist`：独立 Nginx / 静态站点部署使用。
 * - `mediahub/www`：Go 服务通过 `http.FileServer` 直接托管使用。
 *
 * 这个脚本只负责把已构建产物复制到内置目录，不负责执行 Vite 构建，
 * 也不读取生产环境变量；调用方必须先完成 `npm run build`，保证 dist
 * 已经反映当前源码和环境配置。
 */
async function syncEmbeddedWww() {
  await assertDistReady()

  // 先删除再复制，避免旧 hash 文件残留后被浏览器或部署脚本误引用。
  await rm(embeddedWwwDir, { recursive: true, force: true })
  await mkdir(embeddedWwwDir, { recursive: true })
  await cp(distDir, embeddedWwwDir, { recursive: true })

  console.log(`已同步前端构建产物: ${distDir} -> ${embeddedWwwDir}`)
}

async function assertDistReady() {
  const indexPath = path.join(distDir, 'index.html')

  try {
    await stat(indexPath)
  } catch (error) {
    throw new Error(`未找到 ${indexPath}，请先执行 npm run build 后再同步。`)
  }
}

syncEmbeddedWww().catch((error) => {
  console.error(error.message)
  process.exitCode = 1
})
