import { ids, slugs } from './ids'
import type { Persona } from './role'

/** One page of the app as the demo opens it, and the callers it serves. */
export type DemoPage = { name: string; path: string; personas: Persona[] }

const everyone: Persona[] = ['signed-out', 'player', 'dm', 'admin']
const signedIn: Persona[] = ['player', 'dm', 'admin']
const members: Persona[] = ['player', 'dm']
const c = `/campaigns/${ids.campaign}`
const mine = `${c}/characters/${ids.characters[0]}`
const lib = (kind: keyof typeof ids.library) => `/library/${ids.library[kind]}/${kind}`

/**
 * Every route of the app, opened on a seeded record, with the callers it serves. The demo test opens
 * each for each of them; a route missing here fails it, so a new page brings its demo slice along.
 */
export const demoPages: DemoPage[] = [
  { name: 'home', path: '/', personas: signedIn },
  { name: 'gallery', path: '/gallery', personas: everyone },
  { name: 'search', path: '/search?q=fire', personas: signedIn },
  { name: 'spells', path: '/compendium/spells', personas: everyone },
  { name: 'spell', path: `/compendium/spells/${slugs.spell}`, personas: everyone },
  { name: 'guides', path: '/compendium/guides', personas: signedIn },
  { name: 'entries', path: `/compendium/${slugs.entryKind}`, personas: everyone },
  { name: 'entry', path: `/compendium/${slugs.entryKind}/${slugs.entry}`, personas: everyone },
  { name: 'automation', path: '/about/automation', personas: everyone },
  { name: 'attribution', path: '/about/attribution', personas: everyone },
  { name: 'campaigns', path: '/campaigns', personas: signedIn },
  { name: 'campaign', path: c, personas: members },
  { name: 'character-new', path: `${c}/characters/new`, personas: ['player'] },
  { name: 'character', path: mine, personas: members },
  { name: 'level-up', path: `${mine}/level-up`, personas: members },
  { name: 'character-spells', path: `${mine}/spells`, personas: members },
  { name: 'character-retrain', path: `${mine}/retrain`, personas: ['player'] },
  { name: 'character-inventory', path: `${mine}/inventory`, personas: members },
  { name: 'npcs', path: `${c}/npcs`, personas: ['dm'] },
  { name: 'npc', path: `${c}/npcs/${ids.npc}`, personas: ['dm'] },
  { name: 'campaign-library', path: `${c}/library`, personas: ['dm'] },
  { name: 'proposals', path: `${c}/proposals`, personas: members },
  { name: 'proposal', path: `${c}/proposals/${ids.proposal}`, personas: members },
  { name: 'library', path: '/library', personas: signedIn },
  { name: 'shared-library', path: '/shared-library', personas: signedIn },
  { name: 'admin-shared', path: '/admin/shared-library', personas: ['admin'] },
  { name: 'library-entry', path: `/library/${ids.library.spell}`, personas: ['dm'] },
  { name: 'spell-builder', path: lib('spell'), personas: ['dm'] },
  { name: 'item-builder', path: lib('item'), personas: ['dm'] },
  { name: 'subclass-builder', path: lib('subclass'), personas: ['dm'] },
  { name: 'class-builder', path: lib('class'), personas: ['dm'] },
  { name: 'species-builder', path: lib('species'), personas: ['dm'] },
  { name: 'feat-builder', path: lib('feat'), personas: ['dm'] },
  { name: 'background-builder', path: lib('background'), personas: ['dm'] },
  { name: 'condition-builder', path: lib('condition'), personas: ['dm'] },
  { name: 'table-builder', path: lib('table'), personas: ['dm'] },
  { name: 'monster-builder', path: lib('monster'), personas: ['dm'] },
  { name: 'encounters', path: `${c}/encounters`, personas: ['dm'] },
  { name: 'loot', path: `${c}/loot`, personas: ['dm'] },
  { name: 'shops', path: `${c}/shops`, personas: ['dm'] },
  { name: 'activity', path: `${c}/activity`, personas: ['dm'] },
  { name: 'dice', path: `${c}/dice`, personas: members },
  { name: 'session', path: `${c}/sessions/${ids.session}`, personas: members },
  { name: 'table', path: `${c}/sessions/${ids.session}/table`, personas: ['dm'] },
  { name: 'maps', path: `${c}/maps`, personas: ['dm'] },
  { name: 'map', path: `${c}/maps/${ids.map}`, personas: ['dm'] },
  { name: 'factions', path: `${c}/factions`, personas: members },
  { name: 'journal', path: `${c}/journal`, personas: members },
  { name: 'rule-variants', path: `${c}/rules`, personas: members },
  { name: 'tracks', path: `${c}/tracks`, personas: members },
  { name: 'downtime', path: `${c}/downtime`, personas: members },
  { name: 'vehicles', path: `${c}/vehicles`, personas: members },
  { name: 'join', path: `/join#${ids.inviteToken}`, personas: ['admin'] },
  { name: 'sign-in', path: '/sign-in', personas: ['signed-out'] },
  { name: 'account-invite', path: '/account-invite#an-account-invite-from-the-demo', personas: ['signed-out'] },
  { name: 'sign-in-link', path: '/sign-in-link#a-sign-in-link-from-the-demo', personas: ['signed-out'] },
  { name: 'oidc-callback', path: '/oidc/callback?code=demo&state=state-from-the-provider', personas: ['signed-out'] },
  { name: 'account', path: '/account', personas: signedIn },
  { name: 'accessibility', path: '/accessibility', personas: signedIn },
  { name: 'admin', path: '/admin', personas: ['admin'] },
  { name: 'admin-account', path: `/admin/accounts/${ids.players[0]}`, personas: ['admin'] },
  { name: 'my-characters', path: '/characters', personas: signedIn },
  { name: 'my-character', path: `/characters/${ids.characters[0]}`, personas: ['player'] },
  { name: 'friends', path: '/friends', personas: signedIn },
  { name: 'dice-sets', path: '/dice-sets', personas: signedIn },
  { name: 'admin-dice', path: '/admin/dice-sets', personas: ['admin'] },
  { name: 'conversations', path: '/conversations', personas: signedIn },
  { name: 'conversation', path: `/conversations/${ids.conversation}`, personas: ['player'] },
]
