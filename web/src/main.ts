import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './app/App.vue'
import { createAppRouter } from './app/router'
import { configureApi, signInOnUnauthorized } from './infrastructure/http'
import './shared/base.css'

// `vite build --mode demo` builds the same app on an in-browser mock; a normal build leaves it out.
if (import.meta.env.MODE === 'demo') {
  void import('./demo/start').then(({ startDemo }) => { startDemo() })
} else {
  configureApi()
  const router = createAppRouter()
  signInOnUnauthorized(router)
  createApp(App).use(createPinia()).use(router).use(VueQueryPlugin).mount('#app')
}
