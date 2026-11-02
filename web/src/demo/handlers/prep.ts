import type { Faction, Journal, Npc, PersonalStanding, Quest, Revision, RollRequest, StandingChange, StandingTier } from '@/infrastructure/api/types.gen'
import { notFound, type Handlers } from '../operations'
import type { Viewer } from '../role'
import { factionArchetypes, hookPoints, ruleVariantCatalogue } from '../seeds/prep/catalogues'
import { at as nth } from '../seeds/prep/ids'
import type { DemoChange, DemoFaction, DemoStanding } from '../seeds/prep/world'
import type { World } from '../world'
import { dm, member, signedIn } from './access'

// The DM's prep, the Campaign's world and the table in play. Most of it is the DM's alone, as the
// contract says; what every Member reads is filtered here the way the API's own views filter it.

const now = () => new Date().toISOString()

// A request without a query string reaches a handler with no query at all, whatever the type says.
const limitOf = (query: { limit?: number } | undefined, fallback: number) => query?.limit ?? fallback

const onlyDm = (who: Viewer, campaignId: string, world: World) => {
  dm(who, campaignId, world)
  return world.prep
}

/** The tier a hidden Standing score reads as. */
export function tierOf(score: number): StandingTier {
  if (score <= -60) return 'hostile'
  if (score <= -20) return 'unfriendly'
  if (score < 20) return 'neutral'
  if (score < 60) return 'friendly'
  return 'allied'
}

// A Player sees confirmed changes for the party and their own Characters, and only the reasons the
// DM shared; never an amount.
function changeView(c: DemoChange, isDm: boolean, mine: (characterId: string) => boolean): StandingChange | null {
  const base: StandingChange = { id: c.id, ...(c.characterId ? { characterId: c.characterId } : {}), rose: c.delta > 0, reason: c.reason, status: c.status, createdAt: c.createdAt }
  if (isDm) return { ...base, dm: { delta: c.delta, shareReason: c.shareReason, origin: c.origin, client: c.client } }
  if (c.status !== 'confirmed' || (c.characterId && !mine(c.characterId))) return null
  return c.shareReason ? base : { ...base, reason: '' }
}

function factionView(f: DemoFaction, isDm: boolean, accountId: string, standings: DemoStanding[], changes: DemoChange[]): Faction {
  const mine = (characterId: string) => standings.some((s) => s.characterId === characterId && s.ownerId === accountId)
  const personal: PersonalStanding[] = standings
    .filter((s) => s.factionId === f.id && (isDm || s.ownerId === accountId))
    .map((s) => ({ characterId: s.characterId, character: s.character, tier: tierOf(s.score), ...(isDm ? { score: s.score } : {}) }))
  const seen = changes.filter((c) => c.factionId === f.id).map((c) => changeView(c, isDm, mine)).filter((c): c is StandingChange => c !== null)
  return {
    id: f.id, name: f.name, archetype: f.archetype, tier: tierOf(f.score),
    ...(isDm ? { dm: { goals: f.goals, territory: f.territory, notes: f.notes, score: f.score } } : {}),
    personal, changes: seen,
  }
}

function journalFor(world: World, isDm: boolean, accountId: string): Journal {
  const { quests, lore, carried } = world.prep
  const mine = carried.filter((c) => c.accountId === null || c.accountId === accountId).map((c) => c.itemSlug)
  const readable = [...new Set(lore.filter((l) => !l.unlockedAt && mine.includes(l.itemSlug)).map((l) => l.itemSlug))]
  const shown = isDm ? lore : lore.filter((l) => l.unlockedAt)
  return {
    dm: isDm,
    quests: isDm ? quests : quests.filter((q) => q.status !== 'hidden'),
    lore: shown.map((l) => ({
      id: l.id, title: l.title, body: l.body, unlocked: Boolean(l.unlockedAt), ...(l.unlockedAt ? { unlockedAt: l.unlockedAt } : {}),
      ...(isDm ? { itemSlug: l.itemSlug } : {}), updatedAt: l.updatedAt,
    })),
    readable,
  }
}

function npcRevision(world: World, who: Viewer, npcId: string, action: Revision['action']) {
  const list = (world.prep.npcRevisions[npcId] ??= [])
  list.unshift({ no: (list[0]?.no ?? 0) + 1, action, author: signedIn(who).nickname, origin: 'ui', createdAt: now() })
}

function findNpc(world: World, npcId: string): Npc {
  const npc = world.prep.npcs.find((n) => n.id === npcId)
  if (!npc) throw notFound('NPC')
  return npc
}

function findQuest(world: World, questId: string): Quest {
  const quest = world.prep.quests.find((q) => q.id === questId)
  if (!quest) throw notFound('Quest')
  return quest
}

export const prepHandlers = {
  // NPCs: the DM's alone.
  listNpcs: ({ who, world, path }) => onlyDm(who, path.campaignId, world).npcs,
  getNpc: ({ who, world, path }) => {
    onlyDm(who, path.campaignId, world)
    return findNpc(world, path.npcId)
  },
  listNpcRevisions: ({ who, world, path }) => {
    onlyDm(who, path.campaignId, world)
    findNpc(world, path.npcId)
    return world.prep.npcRevisions[path.npcId] ?? []
  },
  listDeletedNpcs: ({ who, world, path }) => onlyDm(who, path.campaignId, world).deletedNpcs,
  createNpc: ({ who, world, path, body }) => {
    const prep = onlyDm(who, path.campaignId, world)
    const npc: Npc = { id: crypto.randomUUID(), name: body.name, title: body.title ?? '', description: body.description ?? '', dmNotes: body.dmNotes ?? '', disposition: body.disposition, updatedAt: now() }
    prep.npcs.push(npc)
    npcRevision(world, who, npc.id, 'create')
    return npc
  },
  updateNpc: ({ who, world, path, body }) => {
    onlyDm(who, path.campaignId, world)
    const npc = findNpc(world, path.npcId)
    Object.assign(npc, { name: body.name, title: body.title ?? '', description: body.description ?? '', dmNotes: body.dmNotes ?? '', disposition: body.disposition, updatedAt: now() })
    npcRevision(world, who, npc.id, 'update')
    return npc
  },
  deleteNpc: ({ who, world, path }) => {
    const prep = onlyDm(who, path.campaignId, world)
    const npc = findNpc(world, path.npcId)
    prep.npcs = prep.npcs.filter((n) => n.id !== npc.id)
    prep.deletedNpcs.push({ id: npc.id, name: npc.name, deletedAt: now() })
    npcRevision(world, who, npc.id, 'delete')
    return undefined
  },

  // Encounters, loot, settlements and shops: the DM's prep.
  listEncounterPools: ({ who, world, path }) => onlyDm(who, path.campaignId, world).pools,
  listEncounterTables: ({ who, world, path }) => onlyDm(who, path.campaignId, world).encounterTables,
  listEncounterChecks: ({ who, world, path }) => onlyDm(who, path.campaignId, world).checks,
  listLocations: ({ who, world, path }) => onlyDm(who, path.campaignId, world).locations,
  listLootTables: ({ who, world, path }) => onlyDm(who, path.campaignId, world).loot,
  listSettlements: ({ who, world, path }) => onlyDm(who, path.campaignId, world).settlements,
  listShops: ({ who, world, path }) => onlyDm(who, path.campaignId, world).shops,
  listActivity: ({ who, world, path }) => onlyDm(who, path.campaignId, world).activity,

  // Maps: the DM's until the party plays on them.
  listMaps: ({ who, world, path }) => onlyDm(who, path.campaignId, world).maps,
  getMap: ({ who, world, path }) => {
    const map = onlyDm(who, path.campaignId, world).maps.find((m) => m.id === path.mapId)
    if (!map) throw notFound('Map')
    return map
  },

  // Rolls: every Member sees them, marked for whoever may roll; the Action Log is the DM's.
  listRolls: ({ who, world, path, query }) => {
    const role = member(who, path.campaignId, world)
    const me = signedIn(who).id
    return world.prep.rolls.slice(0, limitOf(query, 20)).map(({ rollerAccountId, ...r }): RollRequest => {
      const mine = rollerAccountId === me
      return { ...r, mine, canRoll: mine || role === 'dm' }
    })
  },
  getActionLog: ({ who, world, path, query }) => onlyDm(who, path.campaignId, world).actions.slice(0, limitOf(query, 50)),

  // The live Session: every Member may open it; its Action Log is the DM's.
  getSession: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    if (path.sessionId !== world.prep.live.session.id) throw notFound('Session')
    return world.prep.live.session
  },
  getSessionLog: ({ who, world, path, query }) => {
    const { live } = onlyDm(who, path.campaignId, world)
    if (path.sessionId !== live.session.id) throw notFound('Session')
    return live.log.slice(0, limitOf(query, 50))
  },

  // Factions: names and tiers for everyone; goals, territory, notes and every number for the DM.
  listFactionArchetypes: ({ who }) => {
    signedIn(who)
    return factionArchetypes
  },
  listFactions: ({ who, world, path }) => {
    const isDm = member(who, path.campaignId, world) === 'dm'
    const { factions, standings, changes } = world.prep
    return factions.map((f) => factionView(f, isDm, signedIn(who).id, standings, changes))
  },
  createFaction: ({ who, world, path, body }) => {
    const prep = onlyDm(who, path.campaignId, world)
    const f: DemoFaction = { id: crypto.randomUUID(), name: body.name.trim(), archetype: body.archetype ?? '', goals: body.goals ?? '', territory: body.territory ?? '', notes: body.notes ?? '', score: 0 }
    prep.factions.push(f)
    return factionView(f, true, signedIn(who).id, prep.standings, prep.changes)
  },
  deleteFaction: ({ who, world, path }) => {
    const prep = onlyDm(who, path.campaignId, world)
    if (!prep.factions.some((f) => f.id === path.factionId)) throw notFound('Faction')
    prep.factions = prep.factions.filter((f) => f.id !== path.factionId)
    return undefined
  },

  // The Journal: a Player gets no hidden Quest and no locked Lore.
  getJournal: ({ who, world, path }) => journalFor(world, member(who, path.campaignId, world) === 'dm', signedIn(who).id),
  createQuest: ({ who, world, path, body }) => {
    const prep = onlyDm(who, path.campaignId, world)
    const quest: Quest = { id: crypto.randomUUID(), name: body.name, summary: body.summary ?? '', status: body.status, steps: body.steps ?? [], updatedAt: now() }
    prep.quests.push(quest)
    return quest
  },
  updateQuest: ({ who, world, path, body }) => {
    onlyDm(who, path.campaignId, world)
    Object.assign(findQuest(world, path.questId), { name: body.name, summary: body.summary ?? '', status: body.status, steps: body.steps ?? [], updatedAt: now() })
    return undefined
  },
  deleteQuest: ({ who, world, path }) => {
    const prep = onlyDm(who, path.campaignId, world)
    findQuest(world, path.questId)
    prep.quests = prep.quests.filter((q) => q.id !== path.questId)
    return undefined
  },

  // Rule Variants: every Member sees how the table plays; the DM switches them.
  listRuleVariants: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    return ruleVariantCatalogue.map((v) => {
      const chosen = world.prep.variants[v.slug]
      const value = v.options.some((o) => o.value === chosen) ? (chosen as string) : nth(v.options, 0).value
      return { slug: v.slug, name: v.name, description: v.description, automated: v.automated, value, options: v.options }
    })
  },
  setRuleVariants: ({ who, world, path, body }) => {
    const prep = onlyDm(who, path.campaignId, world)
    for (const c of body.choices) prep.variants[c.slug] = c.value
    return undefined
  },
  listRuleHooks: ({ who, world, path }) => {
    const isDm = member(who, path.campaignId, world) === 'dm'
    return { dm: isDm, hooks: world.prep.hooks, points: hookPoints, tables: isDm ? world.prep.rollTables : [] }
  },
  createRuleHook: ({ who, world, path, body }) => {
    const prep = onlyDm(who, path.campaignId, world)
    const table = prep.rollTables.find((t) => t.id === body.rollTableId)
    const hook = { id: crypto.randomUUID(), name: body.name, hook: body.hook, ...(table ? { rollTableId: table.id, tableName: table.name } : {}), ...(body.effect ? { effect: body.effect } : {}) }
    prep.hooks.push(hook)
    return hook
  },
  deleteRuleHook: ({ who, world, path }) => {
    const prep = onlyDm(who, path.campaignId, world)
    if (!prep.hooks.some((h) => h.id === path.hookId)) throw notFound('Rule hook')
    prep.hooks = prep.hooks.filter((h) => h.id !== path.hookId)
    return undefined
  },

  // Downtime and vehicles: every Member sees them; a Player spends only their own Characters' days.
  getDowntime: ({ who, world, path }) => {
    const isDm = member(who, path.campaignId, world) === 'dm'
    const me = signedIn(who).id
    const { gameDay, gameMinute, days, recipes, log } = world.prep.downtime
    const characters = world.characters.map((c) => ({ id: c.id, name: c.name, days: days[c.id] ?? 0, mine: isDm || c.ownerId === me }))
    return { dm: isDm, gameDay, gameMinute, characters, recipes, log }
  },
  listVehicles: ({ who, world, path }) => ({ dm: member(who, path.campaignId, world) === 'dm', vehicles: world.prep.vehicles }),
} satisfies Handlers
