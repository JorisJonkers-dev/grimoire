import type { LiveToken } from '@/infrastructure/api/types.gen'

type LiveEffect = NonNullable<LiveToken['effects']>[number]

/** "Exhaustion 3", "Bless · 7 rounds · concentration": an Effect as the effects card and icons name it. */
export function effectLabel(e: LiveEffect, long = false): string {
  const leveled = e.level ? `${e.name} ${String(e.level)}` : e.name
  const name = e.mode ? `${leveled} (${e.mode})` : leveled
  if (!long) return name
  return [name, ...(e.roundsLeft ? [`${String(e.roundsLeft)} rounds`] : []), ...(e.concentration ? ['concentration'] : [])].join(' · ')
}

/** Every SRD condition and the modelled spells, for the effect picker. */
export const knownEffects: { slug: string; name: string; concentration?: boolean }[] = [
  { slug: 'bless', name: 'Bless', concentration: true },
  { slug: 'faerie-fire', name: 'Faerie Fire', concentration: true },
  { slug: 'hunters-mark', name: "Hunter's Mark", concentration: true },
  { slug: 'false-life', name: 'False Life' },
  { slug: 'dispel-magic', name: 'Dispel Magic' },
  { slug: 'counterspell', name: 'Counterspell' },
  { slug: 'polymorph', name: 'Polymorph', concentration: true },
  { slug: 'wild-shape', name: 'Wild Shape' },
  ...['blinded', 'charmed', 'deafened', 'exhaustion', 'frightened', 'grappled', 'incapacitated', 'invisible', 'paralyzed', 'petrified', 'poisoned', 'prone', 'restrained', 'stunned', 'unconscious'].map(
    (slug) => ({ slug, name: (slug[0] ?? '').toUpperCase() + slug.slice(1) }),
  ),
]
