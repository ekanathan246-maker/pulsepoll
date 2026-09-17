import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In dev the Vite server proxies /api (including SSE) to the Go backend,
// so cookies and cross-origin concerns all but disappear.
// Set VITE_PROXY_TARGET if your Go backend runs elsewhere (see README).
// Port 18080 avoids clashing with other local services that commonly use 8080.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_PROXY_TARGET || 'http://localhost:18080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
})