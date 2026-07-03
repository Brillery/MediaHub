import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          const normalizedId = id.replace(/\\/g, '/')
          if (!normalizedId.includes('/node_modules/')) {
            return undefined
          }

          // 这里按“真实进入依赖图的模块路径”分包：
          // 不再声明 ['element-plus'] 聚合入口，避免把未使用的组件导出一起拉进大 chunk。
          if (normalizedId.includes('/node_modules/vue/') || normalizedId.includes('/node_modules/@vue/')) {
            return 'vue'
          }
          if (normalizedId.includes('/node_modules/axios/')) {
            return 'axios'
          }
          if (normalizedId.includes('/node_modules/element-plus/')) {
            return 'elementPlus'
          }

          // 其他第三方依赖统一进入 vendor，保持缓存稳定，也避免业务入口混入库代码。
          return 'vendor'
        },
      },
    },
  },
  server: {
    port: 5174, // 设置端口号为 5174
  },
})
