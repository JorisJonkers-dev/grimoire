import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'
import HomePage from './HomePage.vue'

const GalleryPage = () => import('./GalleryPage.vue')
const SpellListPage = () => import('@/features/compendium/SpellListPage.vue')
const SpellDetailPage = () => import('@/features/compendium/SpellDetailPage.vue')
const AttributionPage = () => import('@/features/compendium/AttributionPage.vue')

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: '/', name: 'home', component: HomePage },
      { path: '/gallery', name: 'gallery', component: GalleryPage },
      { path: '/compendium/spells', name: 'spells', component: SpellListPage },
      { path: '/compendium/spells/:slug', name: 'spell', component: SpellDetailPage },
      { path: '/about/attribution', name: 'attribution', component: AttributionPage },
    ],
  })
}
