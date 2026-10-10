import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发态：npm run dev（默认 3100 端口），/api、/healthz 代理到本地采集服务（8090）。
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3100,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8090', changeOrigin: true },
      '/healthz': { target: 'http://127.0.0.1:8090', changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
})
