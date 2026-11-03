import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5317,
    strictPort: true,
    proxy: { '/api': { target: 'http://127.0.0.1:8099', changeOrigin: false } },
  },
  build: {
    target: 'es2022',
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/@arco-design')) return 'arco'
          if (id.includes('node_modules/@vue') || id.includes('node_modules/vue')) return 'vue'
        },
      },
    },
  },
})
