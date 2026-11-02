import type { DeletedNpc } from '@/infrastructure/api/types.gen'
import type { Core } from '../world'
import { seedEncounters } from './prep/encounters'
import { seedActivity, seedDowntime, seedMaps, seedRolls, seedRuleHooks, seedSession, seedVehicles } from './prep/play'
import { seedFactions, seedJournal, seedNpcs, seedPlaces } from './prep/world'

/**
 * The prep part of the seeded world: the DM's NPCs, encounters, loot and shops, the Factions and the
 * Journal, the rules the table plays by, and the live Session with its Map, rolls and Action Log.
 */
export function seedPrep(core: Core) {
  const { npcs, revisions } = seedNpcs()
  const { pools, tables, checks, loot } = seedEncounters()
  const { rolls, actions } = seedRolls(core)
  const { hooks, tables: rollTables } = seedRuleHooks()
  return {
    npcs,
    deletedNpcs: [] as DeletedNpc[],
    npcRevisions: revisions,
    ...seedPlaces(),
    pools,
    encounterTables: tables,
    checks,
    loot,
    activity: seedActivity(),
    rolls,
    actions,
    maps: seedMaps(core),
    ...seedFactions(core),
    ...seedJournal(core),
    /** What the Campaign has each Rule Variant at; one not named plays as the rules do without it. */
    variants: { flanking: 'on', 'short-rest-cap': '2' } as Record<string, string>,
    hooks,
    rollTables,
    downtime: seedDowntime(core),
    vehicles: seedVehicles(),
    live: seedSession(core),
  }
}
