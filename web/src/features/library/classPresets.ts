import type { ClassCasting } from '@/infrastructure/api/types.gen'

export const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma']
export const srdClasses = ['barbarian', 'bard', 'cleric', 'druid', 'fighter', 'monk', 'paladin', 'ranger', 'rogue', 'sorcerer', 'warlock', 'wizard']
export const casterKinds = [
  { value: 'none', label: 'No spellcasting' },
  { value: 'full', label: 'Full caster (SRD slots)' },
  { value: 'half', label: 'Half caster (SRD slots)' },
  { value: 'pact', label: 'Pact caster (SRD slots)' },
  { value: 'slots', label: 'Its own slot table' },
  { value: 'points', label: 'Spell points' },
]

const levels = (f: (level: number) => number) => Array.from({ length: 20 }, (_, i) => f(i + 1))

// A full caster's slots by level, as a starting point for a class's own table.
const fullSlots = [
  [2], [3], [4, 2], [4, 3], [4, 3, 2], [4, 3, 3], [4, 3, 3, 1], [4, 3, 3, 2], [4, 3, 3, 3, 1], [4, 3, 3, 3, 2],
  [4, 3, 3, 3, 2, 1], [4, 3, 3, 3, 2, 1], [4, 3, 3, 3, 2, 1, 1], [4, 3, 3, 3, 2, 1, 1], [4, 3, 3, 3, 2, 1, 1, 1],
  [4, 3, 3, 3, 2, 1, 1, 1], [4, 3, 3, 3, 2, 1, 1, 1, 1], [4, 3, 3, 3, 3, 1, 1, 1, 1], [4, 3, 3, 3, 3, 2, 1, 1, 1], [4, 3, 3, 3, 3, 2, 2, 1, 1],
]
const nine = (row: number[]) => [...row, ...Array<number>(9 - row.length).fill(0)]

/** Where each kind of spellcasting starts, ready to edit. */
export function castingFor(kind: string, keep: ClassCasting): ClassCasting {
  if (kind === 'none') return { kind }
  const base: ClassCasting = {
    kind, ability: keep.ability ?? 'intelligence', spellList: keep.spellList ?? 'wizard', spellbook: keep.spellbook, afterRest: keep.afterRest,
    cantrips: keep.cantrips ?? levels((l) => (l >= 10 ? 5 : l >= 4 ? 4 : 3)),
    prepared: keep.prepared ?? levels((l) => (kind === 'half' ? Math.floor(l / 2) + 2 : l + 3)),
  }
  if (kind === 'slots') return { ...base, slots: keep.slots ?? fullSlots.map(nine) }
  if (kind === 'points') {
    return {
      ...base,
      points: keep.points ?? levels((l) => 4 * l),
      costs: keep.costs ?? [2, 3, 4, 5, 6, 7, 8, 9, 10],
      maxSpell: keep.maxSpell ?? levels((l) => Math.min(Math.ceil(l / 2), 9)),
    }
  }
  return base
}
