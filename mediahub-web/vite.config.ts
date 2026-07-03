import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        // 将稳定第三方依赖从业务入口拆出来，降低业务代码变更导致的浏览器缓存失效范围。
        manualChunks: {
          vue: ['vue'],
          axios: ['axios'],
          elementPlus: ['element-plus'],
        },
      },
    },
  },
  server: {
    port: 5174, // 设置端口号为 5174
  },
})
