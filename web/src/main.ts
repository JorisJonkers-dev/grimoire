import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './app/App.vue'
import { createAppRouter } from './app/router'
import { configureApi } from './infrastructure/http'
import './shared/base.css'

configureApi()
createApp(App).use(createPinia()).use(createAppRouter()).use(VueQueryPlugin).mount('#app')
