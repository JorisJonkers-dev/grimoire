import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'
import { defineConfig } from 'vitest/config'
import { accessibleCss } from './src/shared/a11y/css.ts'

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

// `--mode demo` builds the same app on the in-browser mock (src/demo): relative paths so it opens from
// any folder or page, its own output folder, and no service worker, which would outlive the demo.
export default defineConfig(({ mode }) => ({
  plugins: [vue(), ...(process.env.VITEST || mode === 'demo' ? [] : [pwa])],
  ...(mode === 'demo' ? { base: './', build: { outDir: 'dist-demo' } } : {}),
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  css: { postcss: { plugins: [accessibleCss()] } },
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
      // The service worker and the dice worker run in the browser alone: one needs a registration, the
      // other WebGL. The offline and the dice E2E cover them.
      exclude: ['src/infrastructure/api/**', 'src/test/**', 'src/main.ts', 'src/**/*.spec.ts', 'src/env.d.ts', 'src/sw/**', 'src/features/dice/diceWorker.ts'],
      thresholds: { lines: 98, branches: 95, functions: 98, statements: 98 },
    },
  },
}))
