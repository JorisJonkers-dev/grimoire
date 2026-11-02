import type { ActionEntry, Activity, LiveAttack, LocalMap, PlaySession, Recipe, RollRequest, RuleHook, SessionAction, TokenKind, Vehicle } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import type { Core } from '../../world'
import { at as nth, prepIds as p } from './ids'

// The table in play: the live Session on the drowned chapel's Map, the rolls made at it, the Action
// Log they left, and what the party does between Sessions.

const at = '2026-10-01T18:00:00Z'
const tonight = '2026-10-08T19:00:00Z'

export function seedMaps(core: Core): LocalMap[] {
  const image = (mapId: string) => `/api/v1/campaigns/${core.campaign.id}/maps/${mapId}/image`
  return [
    {
      id: ids.map, name: 'The drowned chapel', kind: 'local', width: 1600, height: 1200, hexSizePx: 48, originX: 800, originY: 600,
      ambient: 'dim', imageUrl: image(ids.map), gridKind: 'hexes', gridStrength: 40, scaleMiles: 0.1, found: true,
    },
    {
      id: p.worldMap, name: 'The Brinewater Coast', kind: 'world', width: 2400, height: 1800, hexSizePx: 64, originX: 1200, originY: 900,
      ambient: 'bright', imageUrl: image(p.worldMap), gridKind: 'hexes', gridStrength: 25, scaleMiles: 6, found: true,
    },
  ]
}

/** A creature on the live Map as the runtime keeps it, before any audience filter. */
export type DemoToken = {
  id: string
  label: string
  kind: TokenKind
  q: number
  r: number
  hidden: boolean
  darkvisionFt: number
  characterId?: string
  controllerId?: string
  ac: number
  hp: number
  hpMax: number
  attacks: LiveAttack[]
}

const melee = (name: string, toHit: number, damage: string, damageBonus: number, damageType: string): LiveAttack => ({ name, toHit, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage, damageBonus, damageType })

export function seedSession(core: Core): { session: PlaySession; tokens: DemoToken[]; log: SessionAction[] } {
  const session: PlaySession = { id: ids.session, number: 4, status: 'live', seq: 12, gridRadius: 12, startedAt: tonight }
  // Each Character's look at the table: darkvision by species, armour and a weapon by class.
  const kit: Record<string, { darkvisionFt: number; ac: number; attack: LiveAttack }> = {
    rogue: { darkvisionFt: 0, ac: 14, attack: melee('Shortsword', 5, '1d6', 3, 'piercing') },
    fighter: { darkvisionFt: 0, ac: 18, attack: melee('Longsword', 5, '1d8', 3, 'slashing') },
    wizard: { darkvisionFt: 60, ac: 12, attack: melee('Quarterstaff', 2, '1d6', 0, 'bludgeoning') },
    cleric: { darkvisionFt: 120, ac: 18, attack: melee('Mace', 4, '1d6', 2, 'bludgeoning') },
  }
  // The Members after the DM are the Players, in the order of the Characters they own.
  const party: DemoToken[] = core.characters.map((c, i) => {
    const k = kit[c.class] as (typeof kit)[string]
    return {
      id: nth(p.tokens.party, i), label: c.name, kind: 'party', q: i % 2, r: Math.floor(i / 2), hidden: false, darkvisionFt: k.darkvisionFt,
      characterId: c.id, controllerId: nth(core.members, i + 1).id, ac: k.ac, hp: c.hpCurrent, hpMax: c.hpMax, attacks: [k.attack],
    }
  })
  const tokens: DemoToken[] = [
    ...party,
    {
      id: p.tokens.goblin, label: 'Goblin Warrior', kind: 'enemy', q: 4, r: -1, hidden: false, darkvisionFt: 60, ac: 15, hp: 7, hpMax: 10,
      attacks: [melee('Scimitar', 4, '1d6', 2, 'slashing'), { name: 'Shortbow', toHit: 4, reachFt: 0, rangeFt: 80, longRangeFt: 320, damage: '1d6', damageBonus: 2, damageType: 'piercing' }],
    },
    // Lurking under the chapel floor: the DM sees it, the party and the Table Display must not.
    { id: p.tokens.ghoul, label: 'Ghoul', kind: 'enemy', q: 6, r: 2, hidden: true, darkvisionFt: 60, ac: 12, hp: 22, hpMax: 22, attacks: [melee('Claw', 4, '2d4', 2, 'slashing')] },
  ]
  const log: SessionAction[] = [
    { seq: 12, kind: 'attack', actor: 'Brook', origin: 'ui', label: 'Oswin Hale hits the Goblin Warrior for 3 slashing damage', tokenId: p.tokens.goblin, undoable: true, createdAt: '2026-10-08T19:42:00Z' },
    { seq: 11, kind: 'move_token', actor: 'Ash', origin: 'ui', label: 'Tamsin Reed moves 15 feet', tokenId: p.tokens.party[0], undoable: true, createdAt: '2026-10-08T19:40:00Z' },
    { seq: 10, kind: 'place_token', actor: 'Morgan', origin: 'ui', label: 'Ghoul placed, hidden', tokenId: p.tokens.ghoul, undoable: false, createdAt: '2026-10-08T19:20:00Z' },
  ]
  return { session, tokens, log }
}

/** A Roll Request as kept, with the Account of whoever rolls it so each caller's view can mark it. */
export type DemoRoll = Omit<RollRequest, 'mine' | 'canRoll'> & { rollerAccountId: string }

export function seedRolls(core: Core): { rolls: DemoRoll[]; actions: ActionEntry[] } {
  const ash = nth(core.members, 1)
  const brook = nth(core.members, 2)
  const rolls: DemoRoll[] = [
    {
      id: p.rolls[1], purpose: 'Athletics: haul the chapel door', notation: '1d20+5', requestedBy: 'Morgan', roller: { id: brook.id, name: brook.displayName },
      rollerAccountId: brook.accountId, status: 'pending', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }],
      dice: [{ no: 0, group: 0, faces: 20, kept: true }], modifiers: [{ label: 'Athletics', value: 5 }], createdAt: '2026-10-08T19:45:00Z',
    },
    {
      id: p.rolls[0], purpose: 'Stealth: slip past the watch', notation: '1d20+5', requestedBy: 'Morgan', roller: { id: ash.id, name: ash.displayName },
      rollerAccountId: ash.accountId, status: 'resolved', total: 19, groups: [{ index: 0, count: 1, faces: 20, sign: 1 }],
      dice: [{ no: 0, group: 0, faces: 20, value: 14, mode: 'auto', kept: true }], modifiers: [{ label: 'Stealth', value: 5 }],
      createdAt: '2026-10-08T19:05:00Z', resolvedAt: '2026-10-08T19:05:30Z',
    },
  ]
  const actions: ActionEntry[] = [
    { seq: 4, kind: 'roll_requested', actor: 'Morgan', origin: 'ui', rollId: p.rolls[1], value: 0, createdAt: '2026-10-08T19:45:00Z' },
    { seq: 3, kind: 'roll_resolved', actor: 'Ash', origin: 'ui', rollId: p.rolls[0], value: 19, createdAt: '2026-10-08T19:05:30Z' },
    { seq: 2, kind: 'die_rolled', actor: 'Ash', origin: 'ui', seed: '310998412', rollId: p.rolls[0], dieNo: 0, value: 14, createdAt: '2026-10-08T19:05:30Z' },
    { seq: 1, kind: 'roll_requested', actor: 'Morgan', origin: 'ui', rollId: p.rolls[0], value: 0, createdAt: '2026-10-08T19:05:00Z' },
  ]
  return { rolls, actions }
}

export function seedActivity(): Activity[] {
  return [
    { revisionId: p.revisions[0], entityType: 'npc', entityId: ids.npc, name: 'Brother Caddoc', no: 1, action: 'create', author: 'Morgan', origin: 'mcp', client: 'Claude', createdAt: at, undoable: false },
    { revisionId: p.revisions[1], entityType: 'encounter_table', entityId: p.tables.road, name: 'The Brinewater road', no: 1, action: 'create', author: 'Morgan', origin: 'mcp', client: 'Claude', createdAt: tonight, undoable: true },
  ]
}

export type DemoDowntime = {
  gameDay: number
  gameMinute: number
  days: Record<string, number>
  recipes: Recipe[]
  log: { id: string; character: string; activity: 'craft' | 'work' | 'train' | 'research'; detail: string; days: number; at: string }[]
}

export function seedDowntime(core: Core): DemoDowntime {
  const tamsin = nth(core.characters, 0)
  const liri = nth(core.characters, 2)
  return {
    gameDay: 12,
    gameMinute: 540,
    days: Object.fromEntries(core.characters.map((c, i) => [c.id, 5 - i])),
    recipes: [
      { id: p.recipes.potion, name: 'Potion of Healing', makes: 'potion-of-healing', quantity: 1, tool: 'Herbalism Kit', days: 1, costCp: 2500, ingredients: [] },
      { id: p.recipes.oil, name: 'Flask of oil', makes: 'oil', quantity: 2, days: 1, costCp: 5, ingredients: [{ item: 'Flask', count: 2 }] },
    ],
    log: [
      { id: p.downtime[1], character: liri.name, activity: 'research', detail: 'The Bell’s founding', days: 2, at: '2026-10-05T12:00:00Z' },
      { id: p.downtime[0], character: tamsin.name, activity: 'work', detail: '', days: 1, at: '2026-10-04T12:00:00Z' },
    ],
  }
}

export function seedVehicles(): Vehicle[] {
  return [
    {
      id: p.vehicles.gull, name: 'The Lantern Gull', kind: 'water', hull: 80, hullMax: 100, threshold: 10, milesPerDay: 24, speed: 24, shortHanded: false,
      components: [{ id: p.parts[0], name: 'Sails', hp: 30, hpMax: 30, drives: true }, { id: p.parts[1], name: 'Rudder', hp: 20, hpMax: 20, drives: false }],
      stations: [{ id: p.parts[2], name: 'Helm', crew: 1, posted: 1 }, { id: p.parts[3], name: 'Rigging', crew: 2, posted: 2 }],
    },
    {
      id: p.vehicles.wagon, name: 'Ferry cart', kind: 'land', hull: 10, hullMax: 20, threshold: 0, milesPerDay: 16, speed: 8, shortHanded: true,
      components: [{ id: p.parts[4], name: 'Wheels', hp: 10, hpMax: 10, drives: true }],
      stations: [{ id: p.parts[5], name: 'Driver', crew: 1, posted: 0 }],
    },
  ]
}

export function seedRuleHooks(): { hooks: RuleHook[]; tables: { id: string; name: string }[] } {
  const table = { id: ids.library.table, name: 'Shipboard mishaps' }
  return { hooks: [{ id: p.hooks.deck, name: 'Slippery deck', hook: 'natural-1', rollTableId: table.id, tableName: table.name }], tables: [table] }
}
