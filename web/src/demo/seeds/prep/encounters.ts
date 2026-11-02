import type { EncounterCheck, EncounterPool, EncounterTable, LootTable } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import { prepIds as p } from './ids'

// What the coast throws at the party, and what it leaves behind. Every monster is SRD 5.2.

const at = '2026-10-01T18:00:00Z'

export function seedEncounters(): { pools: EncounterPool[]; tables: EncounterTable[]; checks: EncounterCheck[]; loot: LootTable[] } {
  const pools: EncounterPool[] = [
    {
      id: p.pools.shore, name: 'Shore scavengers', levelMin: 1, levelMax: 4, difficulty: 'low', updatedAt: at,
      members: [{ monsterSlug: 'giant-crab', weight: 2, min: 1, max: 3 }, { monsterSlug: 'wolf', weight: 1, min: 2, max: 3 }],
    },
    {
      id: p.pools.dead, name: 'The chapel’s dead', levelMin: 2, levelMax: 5, difficulty: 'moderate', updatedAt: at,
      members: [{ monsterSlug: 'skeleton', weight: 3, min: 2, max: 4 }, { monsterSlug: 'zombie', weight: 1, min: 1, max: 2 }],
    },
  ]
  const tables: EncounterTable[] = [
    {
      id: p.tables.road, name: 'The Brinewater road', regionId: p.locations.ferry, chancePct: 30, visibility: 'secret', updatedAt: at,
      entries: [
        { weight: 2, kind: 'pool', label: 'Shore scavengers', poolId: p.pools.shore },
        { weight: 1, kind: 'encounter', label: 'A Boatmen’s toll', factionId: p.factions.smugglers, monsters: [{ monsterSlug: 'bandit', count: 3 }] },
        { weight: 3, kind: 'nothing', label: 'Only gulls and wind' },
      ],
    },
  ]
  const checks: EncounterCheck[] = [
    {
      id: p.checks.first, sessionId: ids.session, tableName: 'The Brinewater road', trigger: 'travel_leg', mode: 'normal', visibility: 'secret',
      seed: '84211907', chancePct: 30, chanceRoll: 17, status: 'resolved', outcome: 'encounter', entryLabel: 'Shore scavengers',
      monsters: [{ monsterSlug: 'giant-crab', count: 2 }], createdAt: '2026-10-08T19:10:00Z',
    },
  ]
  const loot: LootTable[] = [
    {
      id: p.loot.purse, name: 'A drowned sailor’s purse', rolls: 1, updatedAt: at,
      entries: [
        { weight: 3, kind: 'currency', coin: 'sp', amount: '2d6' },
        { weight: 1, kind: 'item', itemSlug: 'potion-of-healing' },
        { weight: 2, kind: 'nothing' },
      ],
    },
  ]
  return { pools, tables, checks, loot }
}
