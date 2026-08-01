import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The admin bundle is lazy-loaded (see src/App.tsx), so a customer never
// downloads admin code. No PWA: that decision is recorded as D22.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true } },
  },
  build: {
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: { react: ['react', 'react-dom', 'react-router-dom'] },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    globals: true,
  },
})
