import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { createWebHashHistory } from 'vue-router'
import { createAppRouter } from '@/app/router'
import { configureApi, signInOnUnauthorized } from '@/infrastructure/http'
import { guardPictures } from './assets'
import { createDemo } from './demo'
import DemoRoot from './DemoRoot.vue'

/** The real app on the in-browser mock: nothing it asks for leaves the browser. */
export function startDemo(): App {
  const demo = createDemo()
  guardPictures()
  // A live Session opens its socket with the global WebSocket; in the demo that socket is answered here.
  globalThis.WebSocket = demo.Socket
  // The base is never dialled: the demo fetch answers every request before it could be.
  configureApi({ baseUrl: 'https://demo.invalid', fetch: demo.fetch })
  // A hash route works wherever the build is served, from any folder and inside a frame, with no rewrites.
  const router = createAppRouter(createWebHashHistory())
  signInOnUnauthorized(router)
  const app = createApp(DemoRoot, { world: demo.world }).use(createPinia()).use(router).use(VueQueryPlugin)
  app.mount('#app')
  return app
}
