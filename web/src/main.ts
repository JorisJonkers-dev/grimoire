import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './app/App.vue'
import { createAppRouter } from './app/router'
import { configureApi, signInOnUnauthorized } from './infrastructure/http'
import './shared/base.css'

configureApi()
const router = createAppRouter()
signInOnUnauthorized(router)
createApp(App).use(createPinia()).use(router).use(VueQueryPlugin).mount('#app')
