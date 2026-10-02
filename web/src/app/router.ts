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
const CharacterBuilderPage = () => import('@/features/characters/CharacterBuilderPage.vue')
const CharacterSheetPage = () => import('@/features/characters/CharacterSheetPage.vue')
const NpcListPage = () => import('@/features/npcs/NpcListPage.vue')
const NpcPage = () => import('@/features/npcs/NpcPage.vue')
const EncountersPage = () => import('@/features/prep/EncountersPage.vue')
const LootPage = () => import('@/features/prep/LootPage.vue')
const ShopsPage = () => import('@/features/prep/ShopsPage.vue')
const ActivityPage = () => import('@/features/activity/ActivityPage.vue')
const DiceTrayPage = () => import('@/features/rolls/DiceTrayPage.vue')
const LiveSessionPage = () => import('@/features/live/LiveSessionPage.vue')
const TablePage = () => import('@/features/live/TablePage.vue')
const MapsPage = () => import('@/features/live/MapsPage.vue')
const MapCalibrationPage = () => import('@/features/live/MapCalibrationPage.vue')
const SignInPage = () => import('@/features/account/SignInPage.vue')
const AccountInvitePage = () => import('@/features/account/AccountInvitePage.vue')
const SignInLinkPage = () => import('@/features/account/SignInLinkPage.vue')
const AccountPage = () => import('@/features/account/AccountPage.vue')

/** Pages anyone may open without signing in. */
export const publicPages = ['sign-in', 'account-invite', 'sign-in-link', 'spells', 'spell', 'entries', 'entry', 'attribution', 'automation', 'gallery']

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
      { path: '/campaigns/:id/characters/new', name: 'character-new', component: CharacterBuilderPage },
      { path: '/campaigns/:id/characters/:characterId', name: 'character', component: CharacterSheetPage },
      { path: '/campaigns/:id/npcs', name: 'npcs', component: NpcListPage },
      { path: '/campaigns/:id/npcs/:npcId', name: 'npc', component: NpcPage },
      { path: '/campaigns/:id/encounters', name: 'encounters', component: EncountersPage },
      { path: '/campaigns/:id/loot', name: 'loot', component: LootPage },
      { path: '/campaigns/:id/shops', name: 'shops', component: ShopsPage },
      { path: '/campaigns/:id/activity', name: 'activity', component: ActivityPage },
      { path: '/campaigns/:id/dice', name: 'dice', component: DiceTrayPage },
      { path: '/campaigns/:id/sessions/:sid', name: 'session', component: LiveSessionPage },
      { path: '/campaigns/:id/sessions/:sid/table', name: 'table', component: TablePage, meta: { bare: true } },
      { path: '/campaigns/:id/maps', name: 'maps', component: MapsPage },
      { path: '/campaigns/:id/maps/:mapId', name: 'map', component: MapCalibrationPage },
      { path: '/join', name: 'join', component: JoinPage },
      { path: '/about/attribution', name: 'attribution', component: AttributionPage },
      { path: '/sign-in', name: 'sign-in', component: SignInPage },
      { path: '/account-invite', name: 'account-invite', component: AccountInvitePage },
      { path: '/sign-in-link', name: 'sign-in-link', component: SignInLinkPage },
      { path: '/account', name: 'account', component: AccountPage },
    ],
  })
}
