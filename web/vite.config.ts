import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

const api = process.env.GRIMOIRE_API_ORIGIN ?? 'http://localhost:8080'
const proxy = { '/api': { target: api, ws: true }, '/healthz': api, '/readyz': api }

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { proxy },
  preview: { proxy },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.spec.ts'],
    // Whole-app scenario tests mount the router and query client; under coverage they need headroom.
    testTimeout: 15_000,
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/infrastructure/api/**', 'src/test/**', 'src/main.ts', 'src/**/*.spec.ts', 'src/env.d.ts'],
      thresholds: { lines: 98, branches: 95, functions: 98, statements: 98 },
    },
  },
})
