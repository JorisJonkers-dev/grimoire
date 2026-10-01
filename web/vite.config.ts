import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'
import { defineConfig } from 'vitest/config'

const api = process.env.GRIMOIRE_API_ORIGIN ?? 'http://localhost:8080'
const proxy = { '/api': { target: api, ws: true }, '/mcp': api, '/.well-known': api, '/healthz': api, '/readyz': api }

// The installable app: its manifest, and a service worker that keeps the shell, the compendium and
// Character sheets for offline use and shows push notifications.
const pwa = VitePWA({
  strategies: 'injectManifest',
  srcDir: 'src/sw',
  filename: 'sw.ts',
  registerType: 'autoUpdate',
  injectRegister: 'script-defer',
  includeAssets: ['favicon.svg', 'icons/apple-touch-icon.png'],
  injectManifest: { globPatterns: ['**/*.{js,css,html,svg,png,woff2}'] },
  manifest: {
    name: 'Grimoire',
    short_name: 'Grimoire',
    description: 'A D&D table companion: phones for players, a laptop for the DM, a TV for the table.',
    start_url: '/',
    scope: '/',
    display: 'standalone',
    orientation: 'any',
    background_color: '#0E0B09',
    theme_color: '#0E0B09',
    icons: [
      { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
      { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
      { src: '/icons/maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
    ],
  },
})

export default defineConfig({
  plugins: [vue(), ...(process.env.VITEST ? [] : [pwa])],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { proxy },
  preview: { proxy },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.spec.ts'],
    setupFiles: ['src/test/setup.ts'],
    // Whole-app scenario tests mount the router and query client; under coverage they need headroom.
    testTimeout: 15_000,
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,vue}'],
      // The service worker runs in the browser alone; the offline E2E covers it.
      exclude: ['src/infrastructure/api/**', 'src/test/**', 'src/main.ts', 'src/**/*.spec.ts', 'src/env.d.ts', 'src/sw/**'],
      thresholds: { lines: 98, branches: 95, functions: 98, statements: 98 },
    },
  },
})
