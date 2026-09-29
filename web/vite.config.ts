import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

const api = process.env.GRIMOIRE_API_ORIGIN ?? 'http://localhost:8080'
const proxy = { '/api': api, '/healthz': api, '/readyz': api }

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { proxy },
  preview: { proxy },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.spec.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/infrastructure/api/**', 'src/main.ts', 'src/**/*.spec.ts', 'src/env.d.ts'],
      thresholds: { lines: 95, branches: 95, functions: 95, statements: 95 },
    },
  },
})
