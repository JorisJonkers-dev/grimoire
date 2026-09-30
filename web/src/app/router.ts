import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'
import HomePage from './HomePage.vue'

const GalleryPage = () => import('./GalleryPage.vue')
const SpellListPage = () => import('@/features/compendium/SpellListPage.vue')
const SpellDetailPage = () => import('@/features/compendium/SpellDetailPage.vue')
const AttributionPage = () => import('@/features/compendium/AttributionPage.vue')
const EntryListPage = () => import('@/features/compendium/EntryListPage.vue')
const EntryDetailPage = () => import('@/features/compendium/EntryDetailPage.vue')
const AutomationPage = () => import('@/features/compendium/AutomationPage.vue')
const CampaignListPage = () => import('@/features/campaigns/CampaignListPage.vue')
const CampaignHomePage = () => import('@/features/campaigns/CampaignHomePage.vue')
const JoinPage = () => import('@/features/campaigns/JoinPage.vue')

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({
    history,
    routes: [
      { path: '/', name: 'home', component: HomePage },
      { path: '/gallery', name: 'gallery', component: GalleryPage },
      { path: '/compendium/spells', name: 'spells', component: SpellListPage },
      { path: '/compendium/spells/:slug', name: 'spell', component: SpellDetailPage },
      { path: '/compendium', redirect: { name: 'spells' } },
      { path: '/compendium/:kind', name: 'entries', component: EntryListPage },
      { path: '/compendium/:kind/:slug', name: 'entry', component: EntryDetailPage },
      { path: '/about/automation', name: 'automation', component: AutomationPage },
      { path: '/campaigns', name: 'campaigns', component: CampaignListPage },
      { path: '/campaigns/:id', name: 'campaign', component: CampaignHomePage },
      { path: '/join', name: 'join', component: JoinPage },
      { path: '/about/attribution', name: 'attribution', component: AttributionPage },
    ],
  })
}
