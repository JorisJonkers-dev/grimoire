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
const LevelUpPage = () => import('@/features/characters/LevelUpPage.vue')
const SpellsPage = () => import('@/features/characters/SpellsPage.vue')
const RetrainPage = () => import('@/features/characters/RetrainPage.vue')
const InventoryPage = () => import('@/features/characters/InventoryPage.vue')
const MyCharactersPage = () => import('@/features/characters/MyCharactersPage.vue')
const MyCharacterPage = () => import('@/features/characters/MyCharacterPage.vue')
const FriendsPage = () => import('@/features/friends/FriendsPage.vue')
const DiceSetsPage = () => import('@/features/dice/DiceSetsPage.vue')
const DiceReviewPage = () => import('@/features/admin/DiceReviewPage.vue')
const ConversationsPage = () => import('@/features/conversations/ConversationsPage.vue')
const ConversationPage = () => import('@/features/conversations/ConversationPage.vue')
const NpcListPage = () => import('@/features/npcs/NpcListPage.vue')
const LibraryPage = () => import('@/features/library/LibraryPage.vue')
const LibraryEntryPage = () => import('@/features/library/LibraryEntryPage.vue')
const CampaignLibraryPage = () => import('@/features/library/CampaignLibraryPage.vue')
const ProposalsPage = () => import('@/features/library/ProposalsPage.vue')
const SharedLibraryPage = () => import('@/features/library/SharedLibraryPage.vue')
const SpellBuilderPage = () => import('@/features/library/SpellBuilderPage.vue')
const ItemBuilderPage = () => import('@/features/library/ItemBuilderPage.vue')
const SubclassBuilderPage = () => import('@/features/library/SubclassBuilderPage.vue')
const ClassBuilderPage = () => import('@/features/library/ClassBuilderPage.vue')
const SpeciesBuilderPage = () => import('@/features/library/SpeciesBuilderPage.vue')
const FeatBuilderPage = () => import('@/features/library/FeatBuilderPage.vue')
const BackgroundBuilderPage = () => import('@/features/library/BackgroundBuilderPage.vue')
const ConditionBuilderPage = () => import('@/features/library/ConditionBuilderPage.vue')
const MonsterBuilderPage = () => import('@/features/library/MonsterBuilderPage.vue')
const SharedReviewPage = () => import('@/features/library/SharedReviewPage.vue')
const ProposalPage = () => import('@/features/library/ProposalPage.vue')
const NpcPage = () => import('@/features/npcs/NpcPage.vue')
const EncountersPage = () => import('@/features/prep/EncountersPage.vue')
const LootPage = () => import('@/features/prep/LootPage.vue')
const ShopsPage = () => import('@/features/prep/ShopsPage.vue')
const ActivityPage = () => import('@/features/activity/ActivityPage.vue')
const DiceTrayPage = () => import('@/features/rolls/DiceTrayPage.vue')
const LiveSessionPage = () => import('@/features/live/LiveSessionPage.vue')
const TablePage = () => import('@/features/live/TablePage.vue')
const MapsPage = () => import('@/features/live/MapsPage.vue')
const FactionsPage = () => import('@/features/campaigns/FactionsPage.vue')
const JournalPage = () => import('@/features/campaigns/JournalPage.vue')
const MapCalibrationPage = () => import('@/features/live/MapCalibrationPage.vue')
const SignInPage = () => import('@/features/account/SignInPage.vue')
const AccountInvitePage = () => import('@/features/account/AccountInvitePage.vue')
const SignInLinkPage = () => import('@/features/account/SignInLinkPage.vue')
const AccountPage = () => import('@/features/account/AccountPage.vue')
const OidcCallbackPage = () => import('@/features/account/OidcCallbackPage.vue')
const AdminPage = () => import('@/features/admin/AdminPage.vue')
const AdminAccountPage = () => import('@/features/admin/AdminAccountPage.vue')

/** Pages anyone may open without signing in. */
export const publicPages = ['sign-in', 'account-invite', 'sign-in-link', 'oidc-callback', 'spells', 'spell', 'entries', 'entry', 'attribution', 'automation', 'gallery']

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
      { path: '/campaigns/:id/characters/:characterId/level-up', name: 'level-up', component: LevelUpPage },
      { path: '/campaigns/:id/characters/:characterId/spells', name: 'character-spells', component: SpellsPage },
      { path: '/campaigns/:id/characters/:characterId/retrain', name: 'character-retrain', component: RetrainPage },
      { path: '/campaigns/:id/characters/:characterId/inventory', name: 'character-inventory', component: InventoryPage },
      { path: '/campaigns/:id/npcs', name: 'npcs', component: NpcListPage },
      { path: '/campaigns/:id/library', name: 'campaign-library', component: CampaignLibraryPage },
      { path: '/campaigns/:id/proposals', name: 'proposals', component: ProposalsPage },
      { path: '/campaigns/:id/proposals/:proposalId', name: 'proposal', component: ProposalPage },
      { path: '/library', name: 'library', component: LibraryPage },
      { path: '/shared-library', name: 'shared-library', component: SharedLibraryPage },
      { path: '/admin/shared-library', name: 'admin-shared', component: SharedReviewPage },
      { path: '/library/:entryId', name: 'library-entry', component: LibraryEntryPage },
      { path: '/library/:entryId/spell', name: 'spell-builder', component: SpellBuilderPage },
      { path: '/library/:entryId/item', name: 'item-builder', component: ItemBuilderPage },
      { path: '/library/:entryId/subclass', name: 'subclass-builder', component: SubclassBuilderPage },
      { path: '/library/:entryId/class', name: 'class-builder', component: ClassBuilderPage },
      { path: '/library/:entryId/species', name: 'species-builder', component: SpeciesBuilderPage },
      { path: '/library/:entryId/feat', name: 'feat-builder', component: FeatBuilderPage },
      { path: '/library/:entryId/background', name: 'background-builder', component: BackgroundBuilderPage },
      { path: '/library/:entryId/condition', name: 'condition-builder', component: ConditionBuilderPage },
      { path: '/library/:entryId/monster', name: 'monster-builder', component: MonsterBuilderPage },
      { path: '/campaigns/:id/npcs/:npcId', name: 'npc', component: NpcPage },
      { path: '/campaigns/:id/encounters', name: 'encounters', component: EncountersPage },
      { path: '/campaigns/:id/loot', name: 'loot', component: LootPage },
      { path: '/campaigns/:id/shops', name: 'shops', component: ShopsPage },
      { path: '/campaigns/:id/activity', name: 'activity', component: ActivityPage },
      { path: '/campaigns/:id/dice', name: 'dice', component: DiceTrayPage },
      { path: '/campaigns/:id/sessions/:sid', name: 'session', component: LiveSessionPage },
      { path: '/campaigns/:id/sessions/:sid/table', name: 'table', component: TablePage, meta: { bare: true } },
      { path: '/campaigns/:id/maps', name: 'maps', component: MapsPage },
      { path: '/campaigns/:id/factions', name: 'factions', component: FactionsPage },
      { path: '/campaigns/:id/journal', name: 'journal', component: JournalPage },
      { path: '/campaigns/:id/maps/:mapId', name: 'map', component: MapCalibrationPage },
      { path: '/join', name: 'join', component: JoinPage },
      { path: '/about/attribution', name: 'attribution', component: AttributionPage },
      { path: '/sign-in', name: 'sign-in', component: SignInPage },
      { path: '/account-invite', name: 'account-invite', component: AccountInvitePage },
      { path: '/sign-in-link', name: 'sign-in-link', component: SignInLinkPage },
      { path: '/account', name: 'account', component: AccountPage },
      { path: '/oidc/callback', name: 'oidc-callback', component: OidcCallbackPage },
      { path: '/admin', name: 'admin', component: AdminPage },
      { path: '/characters', name: 'my-characters', component: MyCharactersPage },
      { path: '/friends', name: 'friends', component: FriendsPage },
      { path: '/dice-sets', name: 'dice-sets', component: DiceSetsPage },
      { path: '/admin/dice-sets', name: 'admin-dice', component: DiceReviewPage },
      { path: '/conversations', name: 'conversations', component: ConversationsPage },
      { path: '/conversations/:conversationId', name: 'conversation', component: ConversationPage },
      { path: '/characters/:characterId', name: 'my-character', component: MyCharacterPage },
      { path: '/admin/accounts/:id', name: 'admin-account', component: AdminAccountPage },
    ],
  })
}
