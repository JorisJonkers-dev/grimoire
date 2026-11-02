import type { Location, Npc, Quest, Revision, Settlement, Shop } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import type { Core } from '../../world'
import { at as nth, prepIds as p } from './ids'

// The Campaign's world as the DM keeps it: the people of Bellhaven, its shops, the Factions that
// watch the party, and the Quests and Lore of the Journal.

const at = '2026-10-01T18:00:00Z'
const later = '2026-10-08T19:30:00Z'

export function seedNpcs(): { npcs: Npc[]; revisions: Record<string, Revision[]> } {
  const npcs: Npc[] = [
    {
      id: ids.npc, name: 'Brother Caddoc', title: 'Keeper of the drowned chapel', disposition: 'friendly', updatedAt: later,
      description: 'A stooped man in a salt-stained habit who still rings a hand bell at every tide.',
      dmNotes: 'He knows the Bell was sunk on purpose, and by whom. He will not say so until the party has earned the Wardens’ trust.',
    },
    {
      id: p.npcs.ivel, name: 'Mother Ivel', title: 'Herbalist of Bellhaven', disposition: 'neutral', updatedAt: at,
      description: 'Keeps a crowded shop of dried kelp, jars and remedies above the fish market.',
      dmNotes: 'Pays the smugglers for rare herbs and keeps their ledger under the counter.',
    },
    {
      id: p.npcs.odo, name: 'Captain Odo Marsh', title: 'Harbour watch', disposition: 'hostile', updatedAt: at,
      description: 'Broad, loud and certain that strangers bring trouble ashore with them.',
      dmNotes: 'Takes a cut from the smugglers; fears the party will find out.',
    },
  ]
  const revisions: Record<string, Revision[]> = {
    [ids.npc]: [
      { no: 2, action: 'update', author: 'Morgan', origin: 'ui', createdAt: later },
      { no: 1, action: 'create', author: 'Morgan', origin: 'mcp', client: 'Claude', createdAt: at },
    ],
    [p.npcs.ivel]: [{ no: 1, action: 'create', author: 'Morgan', origin: 'ui', createdAt: at }],
    [p.npcs.odo]: [{ no: 1, action: 'create', author: 'Morgan', origin: 'ui', createdAt: at }],
  }
  return { npcs, revisions }
}

export function seedPlaces(): { locations: Location[]; settlements: Settlement[]; shops: Shop[] } {
  const locations: Location[] = [
    { id: p.locations.bellhaven, name: 'Bellhaven', mapName: 'The Brinewater Coast' },
    { id: p.locations.ferry, name: 'Gullrock Ferry', mapName: 'The Brinewater Coast' },
    { id: p.locations.chapel, name: 'The drowned chapel', mapName: 'The Brinewater Coast' },
  ]
  const settlements: Settlement[] = [
    { id: p.settlements.bellhaven, name: 'Bellhaven', size: 'town', wealth: 'modest', locationId: p.locations.bellhaven, updatedAt: at },
    { id: p.settlements.ferry, name: 'Gullrock Ferry', size: 'hamlet', wealth: 'poor', locationId: p.locations.ferry, updatedAt: at },
  ]
  const shops: Shop[] = [
    {
      id: p.shops.rope, settlementId: p.settlements.bellhaven, name: 'The Tarred Rope', kind: 'general', factionId: p.factions.watch,
      markupPct: 10, haggleDc: 12, hagglePct: 10, restock: 'long_rest', stockedDay: 10, updatedAt: at,
      stock: [
        { itemSlug: 'rope', quantity: 6, priceCp: 100 },
        { itemSlug: 'torch', quantity: 20, priceCp: 1 },
        { itemSlug: 'rations', quantity: 12, priceCp: 50 },
      ],
    },
    {
      id: p.shops.remedies, settlementId: p.settlements.bellhaven, name: 'Mother Ivel’s Remedies', kind: 'apothecary', ownerId: p.npcs.ivel,
      markupPct: 25, haggleDc: 15, hagglePct: 15, restock: 'days', restockDays: 7, stockedDay: 7, updatedAt: at,
      stock: [{ itemSlug: 'potion-of-healing', quantity: 3, priceCp: 5000 }],
    },
  ]
  return { locations, settlements, shops }
}

/** A Faction as the DM keeps it; a Player is shown its name and the tier its score reads as. */
export type DemoFaction = { id: string; name: string; archetype: string; goals: string; territory: string; notes: string; score: number }
/** One Character's own Standing with a Faction, and whose Character it is. */
export type DemoStanding = { factionId: string; characterId: string; character: string; ownerId: string; score: number }
/** A Standing Change as suggested and decided; the delta, origin and sharing are the DM's alone. */
export type DemoChange = {
  id: string
  factionId: string
  characterId?: string
  delta: number
  reason: string
  shareReason: boolean
  status: 'pending' | 'confirmed' | 'dismissed'
  origin: 'ui' | 'mcp' | 'generator' | 'system'
  client: string
  createdAt: string
}

export function seedFactions(core: Core): { factions: DemoFaction[]; standings: DemoStanding[]; changes: DemoChange[] } {
  const tamsin = nth(core.characters, 0)
  const oswin = nth(core.characters, 1)
  const factions: DemoFaction[] = [
    {
      id: p.factions.wardens, name: 'The Bell Wardens', archetype: 'temple', score: 30,
      goals: 'Keep the Bell where it lies until the tide that frees what sleeps beneath it has passed.',
      territory: 'The drowned chapel and the cliff path above it.', notes: 'Brother Caddoc is their last voice in Bellhaven.',
    },
    {
      id: p.factions.smugglers, name: 'The Gullrock Boatmen', archetype: 'smuggling-ring', score: -10,
      goals: 'Keep the coves at Gullrock open for their night cargo, and the watch paid to look away.',
      territory: 'Gullrock Ferry and the sea caves south of it.', notes: 'Mother Ivel keeps their ledger.',
    },
    {
      id: p.factions.watch, name: 'Bellhaven Harbour Watch', archetype: 'city-watch', score: -35,
      goals: 'Keep strangers in line and the harbour fees flowing.', territory: 'The harbour and the fish market.',
      notes: 'Captain Odo takes a cut from the Boatmen.',
    },
  ]
  const standings: DemoStanding[] = [
    { factionId: p.factions.smugglers, characterId: tamsin.id, character: tamsin.name, ownerId: tamsin.ownerId, score: 25 },
    { factionId: p.factions.watch, characterId: oswin.id, character: oswin.name, ownerId: oswin.ownerId, score: -65 },
  ]
  const changes: DemoChange[] = [
    { id: p.changes[0], factionId: p.factions.wardens, delta: 30, reason: 'Pulled the drowned bell-ringer from the surf.', shareReason: true, status: 'confirmed', origin: 'ui', client: '', createdAt: at },
    { id: p.changes[1], factionId: p.factions.watch, delta: -35, reason: 'Seen talking to the Boatmen at night.', shareReason: false, status: 'confirmed', origin: 'ui', client: '', createdAt: at },
    { id: p.changes[2], factionId: p.factions.watch, characterId: oswin.id, delta: -30, reason: 'Oswin knocked a watchman into the harbour.', shareReason: true, status: 'confirmed', origin: 'ui', client: '', createdAt: later },
    { id: p.changes[3], factionId: p.factions.smugglers, delta: 10, reason: 'Kept quiet about the cove.', shareReason: true, status: 'pending', origin: 'mcp', client: 'Claude', createdAt: later },
  ]
  return { factions, standings, changes }
}

/** A Lore entry as the DM keeps it: locked until the party reads the item that holds it. */
export type DemoLore = { id: string; title: string; body: string; itemSlug: string; unlockedAt?: string; updatedAt: string }

export function seedJournal(core: Core): { quests: Quest[]; lore: DemoLore[]; carried: { accountId: string | null; itemSlug: string }[] } {
  const quests: Quest[] = [
    {
      id: p.quests.ferryman, name: 'The missing ferryman', status: 'completed', updatedAt: at,
      summary: 'The Gullrock ferry came back without its ferryman.',
      steps: [{ text: 'Search the ferry landing', done: true }, { text: 'Follow the tracks to the sea caves', done: true }],
    },
    {
      id: p.quests.bell, name: 'Raise the Sunken Bell', status: 'active', updatedAt: later,
      summary: 'Brother Caddoc says Bellhaven was safe while its Bell still rang.',
      steps: [{ text: 'Find where the Bell went down', done: true }, { text: 'Hire a boat that can carry it', done: false }],
    },
    {
      id: p.quests.secret, name: 'What the Wardens buried', status: 'hidden', updatedAt: at,
      summary: 'The Bell was sunk on purpose; the Wardens fear what it held down.',
      steps: [{ text: 'Learn why the Wardens sank the Bell', done: false }],
    },
  ]
  const lore: DemoLore[] = [
    {
      id: p.lore.drowning, title: 'The drowning of old Bellhaven', itemSlug: '', unlockedAt: at, updatedAt: at,
      body: 'A hundred years ago a storm tide took the lower town and the chapel with it. The Bell rang until the water closed over it.',
    },
    {
      id: p.lore.chart, title: 'The tide chart’s margin', itemSlug: 'tide-chart', updatedAt: at,
      body: 'Inked in the margin: “At the lowest tide of the year, the chapel stair stands dry for an hour.”',
    },
    {
      id: p.lore.letter, title: 'The ferryman’s last letter', itemSlug: 'ferrymans-letter', updatedAt: later,
      body: 'He wrote that the Boatmen were paid to row something heavy out to the chapel, and that he would not go again.',
    },
  ]
  // The tide chart lies in the Party Stash, which every Member may read from; the ferryman's letter
  // rides with the first Player's Character, so only that Player may read it.
  const carried = [
    { accountId: null, itemSlug: 'tide-chart' },
    { accountId: core.accounts.player.id, itemSlug: 'ferrymans-letter' },
  ]
  return { quests, lore, carried }
}
