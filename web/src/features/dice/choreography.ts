import type { LiveDiceLook } from '@/infrastructure/api/types.gen'

/** A resolved roll as the dice stage shows it: who rolled, for what, each die, the modifier and the total. */
export type ShownDie = { faces: number; value: number; kept: boolean }
export type ShownRoll = { roller: string; purpose: string; dice: ShownDie[]; modifier: number; total: number; look?: LiveDiceLook }
export type Critical = 'hit' | 'miss' | null

type ResolvedDie = { group: number; faces: number; value?: number; kept: boolean }
type Resolved = {
  purpose: string; roller: { name: string }; total?: number
  groups: { index: number; keep?: unknown }[]; dice: ResolvedDie[]; modifiers: { value: number }[]
}
/** A resolved Roll Request as the stage shows it; in a group that keeps none in particular every die counts. */
export function shownOf(roll: Resolved): ShownRoll {
  const keeps = new Set(roll.groups.filter((g) => g.keep).map((g) => g.index))
  return {
    roller: roll.roller.name,
    purpose: roll.purpose,
    dice: roll.dice.map((d) => ({ faces: d.faces, value: d.value ?? 0, kept: keeps.has(d.group) ? d.kept : true })),
    modifier: roll.modifiers.reduce((sum, m) => sum + m.value, 0),
    total: roll.total ?? 0,
  }
}

/** The three beats of a roll (ADR-0012): the throw, the glide to the centre, and how long the total stays. */
export const THROW_MS = 1100
export const GATHER_MS = 450
export const HOLD_MS = 2400

/** A natural 20 on a d20 that counts is a critical hit, a natural 1 a critical miss; a hit wins when both show. */
export function criticalOf(roll: ShownRoll): Critical {
  const naturals = roll.dice.filter((d) => d.faces === 20 && d.kept).map((d) => d.value)
  return naturals.includes(20) ? 'hit' : naturals.includes(1) ? 'miss' : null
}

/** The breakdown under the total: the dice in the order rolled, dropped ones marked, then the modifier. */
export function breakdown(roll: ShownRoll): string {
  const dice = roll.dice.reduce((line, d, i) => {
    const said = d.kept ? String(d.value) : `${String(d.value)} dropped`
    const joiner = i === 0 ? '' : d.kept && roll.dice[i - 1]?.kept ? ' + ' : ', '
    return line + joiner + said
  }, '')
  const modifier = roll.modifier === 0 ? '' : `${roll.modifier < 0 ? '−' : '+'} ${String(Math.abs(roll.modifier))}`
  return [dice, modifier].filter(Boolean).join(' ')
}

const SHAPES = [4, 6, 8, 10, 12, 20, 100] as const
export type Sides = (typeof SHAPES)[number]
/** The shape a die is drawn as: its own when there is one, a cube for anything else. */
export const sidesOf = (faces: number): Sides => SHAPES.find((s) => s === faces) ?? 6

export type Point = { x: number; y: number }
/** One die's throw: in from an edge, to rest near the centre, then into the row the dice gather in. */
export type DieThrow = { from: Point; rest: Point; gather: Point; spin: number; yaw: number }

/** The table is ten units high and as wide as the screen makes it; dice are about two units across. */
export const TABLE_HEIGHT = 10
const SPACING = 2.2

// A small seeded generator, so one roll throws the same way on every screen.
function seeded(seed: number): () => number {
  let s = (seed >>> 0) + 0x9e3779b9
  return () => {
    s = Math.imul(s ^ (s >>> 15), 0x2c1b3c6d)
    s = Math.imul(s ^ (s >>> 12), 0x297a2d39)
    s ^= s >>> 15
    return (s >>> 0) / 0x100000000
  }
}

/** Plans the throw of every die of a roll from a seed: the same seed always throws the same way. */
export function throwPlan(dice: ShownDie[], seed: number, aspect: number): DieThrow[] {
  const next = seeded(seed)
  const half = { x: (aspect * TABLE_HEIGHT) / 2, y: TABLE_HEIGHT / 2 }
  const edge = Math.floor(next() * 4)
  return dice.map((_, i) => {
    const along = (next() - 0.5) * 1.2
    // Every die of a roll comes in over the same edge, a little apart, as from one hand.
    const from = [
      { x: -half.x - 2, y: along * half.y },
      { x: half.x + 2, y: along * half.y },
      { x: along * half.x, y: -half.y - 2 },
      { x: along * half.x, y: half.y + 2 },
    ][edge] ?? { x: -half.x - 2, y: 0 }
    // They come to rest on a ring around the centre, so none lands on another.
    const angle = ((i + next() * 0.3) / dice.length) * Math.PI * 2
    const reach = dice.length === 1 ? 0 : Math.max(SPACING, (dice.length * SPACING) / (Math.PI * 2))
    const ring = Math.min(reach, Math.min(half.x, half.y) * 0.55)
    return {
      from,
      rest: { x: Math.cos(angle) * ring, y: Math.sin(angle) * ring },
      gather: { x: (i - (dice.length - 1) / 2) * SPACING, y: 0 },
      spin: 6 + next() * 6,
      yaw: next() * Math.PI * 2,
    }
  })
}
