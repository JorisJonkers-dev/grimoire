import type { LiveToken } from '@/infrastructure/api/types.gen'

type LiveEffect = NonNullable<LiveToken['effects']>[number]

/** "Exhaustion 3", "Bless · 7 rounds · concentration": an Effect as the effects card and icons name it. */
export function effectLabel(e: LiveEffect, long = false): string {
  const name = e.level ? `${e.name} ${String(e.level)}` : e.name
  if (!long) return name
  return [name, ...(e.roundsLeft ? [`${String(e.roundsLeft)} rounds`] : []), ...(e.concentration ? ['concentration'] : [])].join(' · ')
}

/** Every SRD condition and the modelled spells, for the effect picker. */
export const knownEffects = [
  { slug: 'bless', name: 'Bless' },
  { slug: 'faerie-fire', name: 'Faerie Fire' },
  { slug: 'hunters-mark', name: "Hunter's Mark" },
  ...['blinded', 'charmed', 'deafened', 'exhaustion', 'frightened', 'grappled', 'incapacitated', 'invisible', 'paralyzed', 'petrified', 'poisoned', 'prone', 'restrained', 'stunned', 'unconscious'].map(
    (slug) => ({ slug, name: (slug[0] ?? '').toUpperCase() + slug.slice(1) }),
  ),
]
