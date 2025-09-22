import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    open: true,
    host: true,
    // Proxy API requests to the backend during development
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      }
    }
  },
  define: {
    global: 'globalThis',
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    // Optimize bundle for production
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom'],
          utils: ['./src/utils/api.ts']
        }
      }
    }
  },
  // Path aliases for cleaner imports (optional)
  resolve: {
    alias: {
      '@': '/src',
      '@/components': '/src/components',
      '@/utils': '/src/utils',
      '@/types': '/src/types'
    }
  },
  // Environment variable prefix
  envPrefix: 'VITE_'
})