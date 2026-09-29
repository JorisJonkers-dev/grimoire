import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'
import HomePage from './HomePage.vue'

const GalleryPage = () => import('./GalleryPage.vue')

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: '/', name: 'home', component: HomePage },
      { path: '/gallery', name: 'gallery', component: GalleryPage },
    ],
  })
}
