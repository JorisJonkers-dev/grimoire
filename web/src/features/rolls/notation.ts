import type { DiceGroup, GroupLabel } from '@/infrastructure/api/types.gen'

export const dieSizes = [4, 6, 8, 10, 12, 20, 100] as const
export type DieSize = (typeof dieSizes)[number]
export type Edge = 'normal' | 'advantage' | 'disadvantage'

export type Throw = { count: number; faces: DieSize; edge: Edge; bless: boolean; bane: boolean }

/** Builds notation and group labels; advantage and disadvantage apply to a single d20. */
export function notationFor(t: Throw): { notation: string; labels: GroupLabel[] } {
  const labels: GroupLabel[] = []
  let notation = `${String(t.count)}d${String(t.faces)}`
  if (t.count === 1 && t.faces === 20 && t.edge !== 'normal') {
    notation = t.edge === 'advantage' ? '2d20kh1' : '2d20kl1'
    labels.push({ group: 0, label: t.edge === 'advantage' ? 'Advantage' : 'Disadvantage' })
  }
  if (t.bless) {
    labels.push({ group: 1, label: 'Bless' })
    notation += '+1d4'
  }
  if (t.bane) {
    labels.push({ group: t.bless ? 2 : 1, label: 'Bane' })
    notation += '-1d4'
  }
  return { notation, labels }
}

/** Says in words what to throw for one group, e.g. "2 × d20, keep highest". */
export function describeGroup(g: DiceGroup): string {
  const keep = g.keep ? `, keep ${g.keep}${g.keepCount === 1 ? '' : ` ${String(g.keepCount ?? '')}`}` : ''
  const sign = g.sign < 0 ? '− ' : ''
  return `${sign}${String(g.count)} × d${String(g.faces)}${keep}${g.label ? ` — ${g.label}` : ''}`
}

export function signed(n: number): string {
  return n >= 0 ? `+${String(n)}` : `−${String(-n)}`
}
