import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Dev: Vite on 0.0.0.0:5173, proxies /api (incl. WebSocket/SSE) to the Go API on :8080.
// Build: output is embedded into the Go binary (backend/internal/webui/dist).
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true, ws: true },
    },
  },
  build: {
    outDir: '../backend/internal/webui/dist',
    emptyOutDir: true,
  },
})
