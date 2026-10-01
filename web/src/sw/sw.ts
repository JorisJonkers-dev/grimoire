import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'
import { NetworkFirst, StaleWhileRevalidate } from 'workbox-strategies'

declare const self: ServiceWorkerGlobalScope

// The app shell works offline; every page falls back to it, except what the API owns.
precacheAndRoute(self.__WB_MANIFEST)
cleanupOutdatedCaches()
registerRoute(new NavigationRoute(createHandlerBoundToURL('/index.html'), { denylist: [/^\/api\//, /^\/mcp/, /^\/\.well-known\//] }))

// The compendium rarely changes: answer from the cache at once and refresh it behind.
registerRoute(({ url, request }) => request.method === 'GET' && url.pathname.startsWith('/api/v1/compendium/'), new StaleWhileRevalidate({ cacheName: 'compendium' }))

// Campaigns and Character sheets prefer the network and fall back to the last copy offline.
registerRoute(
  ({ url, request }) => request.method === 'GET' && /^\/api\/v1\/(me|campaigns(\/[^/]+(\/characters(\/[^/]+(\/(portrait|token))?)?)?)?)$/.test(url.pathname),
  new NetworkFirst({ cacheName: 'sheets', networkTimeoutSeconds: 4 }),
)

self.addEventListener('install', () => {
  void self.skipWaiting()
})
self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim())
})

type Notice = { title?: string; body?: string; url?: string }

// A turn or a Reaction Prompt reaches a locked phone as a notification.
self.addEventListener('push', (event) => {
  const n = (event.data?.json() ?? {}) as Notice
  event.waitUntil(
    self.registration.showNotification(n.title ?? 'Grimoire', {
      body: n.body ?? '',
      icon: '/icons/icon-192.png',
      badge: '/icons/icon-192.png',
      tag: n.url ?? 'grimoire',
      data: { url: n.url ?? '/' },
    }),
  )
})

// Tapping it opens the Session, reusing a window that already shows the app.
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data as { url?: string } | null)?.url ?? '/'
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(async (windows) => {
      const open = windows[0]
      if (open) {
        await open.navigate(url)
        await open.focus()
        return
      }
      await self.clients.openWindow(url)
    }),
  )
})
