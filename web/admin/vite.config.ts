import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import compress from 'vite-plugin-compression2'

export default defineConfig({
  plugins: [
    react(),
    // Generate both .gz and .br in a single plugin instance (v2 supports array of algorithms).
    // Threshold 1024: skip files <1KB to avoid overhead on tiny assets.
    compress({
      algorithm: ['gzip', 'brotliCompress'],
      threshold: 1024,
      // Delete originals after compression to keep dist lean (server serves from embed fs).
      deleteOriginFile: false,
    }),
  ],
  base: '/admin/ui/',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-query': ['@tanstack/react-query'],
          // recharts + its d3 dependencies in one chunk (saves ~100KB in main bundle)
          'vendor-charts': ['recharts', 'd3-array', 'd3-scale', 'd3-interpolate'],
          'vendor-i18n': ['i18next', 'react-i18next'],
          'vendor-form': ['react-hook-form', '@hookform/resolvers', 'zod'],
          'vendor-sonner': ['sonner'],
        },
      },
    },
  },
})
