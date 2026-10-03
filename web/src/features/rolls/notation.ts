import type { DiceGroup, GroupLabel, RollRequest } from '@/infrastructure/api/types.gen'

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

/**
 * What a resolved roll's total is made of, a line for each part: every group of dice by its label
 * (Advantage, Bless, Bane) with its dropped dice marked, each modifier, and Heroic Inspiration when it
 * was spent on a reroll.
 */
export function rollBreakdown(roll: RollRequest): { label: string; value: string }[] {
  const groups = roll.groups.map((g) => {
    const dice = roll.dice.filter((d) => d.group === g.index)
    const counted = dice.filter((d) => !g.keep || d.kept).reduce((sum, d) => sum + (d.value ?? 0), 0)
    // A lone bonus die reads like a modifier: Bless +3. Dice that are chosen between show each face.
    const value = g.label && !g.keep ? signed(g.sign * counted) : dice.map((d) => `${String(d.value ?? 0)}${g.keep && !d.kept ? ' dropped' : ''}`).join(g.keep ? ', ' : ' + ')
    return { label: g.label ?? `d${String(g.faces)}`, value }
  })
  // A line that only changes how the die is rolled, such as a Standing that gives Advantage, adds nothing.
  const modifiers = roll.modifiers.map((m) => ({ label: m.label, value: m.value === 0 ? '' : signed(m.value) }))
  return [...groups, ...modifiers, ...(roll.rerolled ? [{ label: 'Heroic Inspiration', value: 'rerolled a die' }] : [])]
}
