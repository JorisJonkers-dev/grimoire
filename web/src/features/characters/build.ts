import type { Ability, AbilityBase, AbilityBonus } from '@/infrastructure/api/types.gen'

export const abilities: readonly Ability[] = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma']
export const abbrev: Record<Ability, string> = {
  strength: 'STR',
  dexterity: 'DEX',
  constitution: 'CON',
  intelligence: 'INT',
  wisdom: 'WIS',
  charisma: 'CHA',
}
export const standardArray = [15, 14, 13, 12, 10, 8] as const

export type Method = 'standard-array' | 'point-buy' | 'rolled'

export function modifier(score: number): number {
  return Math.floor((score - 10) / 2)
}

export function signed(n: number): string {
  return n >= 0 ? `+${String(n)}` : String(n)
}

export function pointCost(score: number): number {
  return score <= 13 ? score - 8 : 5 + (score - 13) * 2
}

export function pointsSpent(base: AbilityBase): number {
  return abilities.reduce((sum, a) => sum + pointCost(base[a]), 0)
}

export function defaultBase(method: Method): AbilityBase {
  const [a, b, c, d, e, f] = standardArray
  if (method === 'standard-array') return { strength: a, dexterity: b, constitution: c, intelligence: d, wisdom: e, charisma: f }
  const score = method === 'point-buy' ? 8 : 10
  return { strength: score, dexterity: score, constitution: score, intelligence: score, wisdom: score, charisma: score }
}

/** True when the base scores fit the method; the server has the final word. */
export function baseValid(method: Method, base: AbilityBase, budget: number): boolean {
  const values = abilities.map((a) => base[a])
  if (method === 'standard-array') return [...values].sort((x, y) => y - x).join() === standardArray.join()
  if (method === 'point-buy') return values.every((v) => v >= 8 && v <= 15) && pointsSpent(base) <= budget
  return values.every((v) => Number.isInteger(v) && v >= 3 && v <= 18)
}

export type BonusPattern = 'two-one' | 'one-one-one'

/** Builds the origin increases: +2 to first and +1 to second, or +1 to each of three. */
export function bonusFor(pattern: BonusPattern, picks: readonly Ability[]): AbilityBonus {
  const out: AbilityBonus = {}
  if (pattern === 'two-one') {
    const [plusTwo, plusOne] = picks
    if (plusTwo) out[plusTwo] = 2
    if (plusOne && plusOne !== plusTwo) out[plusOne] = 1
    return out
  }
  for (const a of picks.slice(0, 3)) out[a] = 1
  return out
}

export function bonusValid(bonus: AbilityBonus): boolean {
  const values = Object.values(bonus).sort()
  return values.join() === '1,2' || values.join() === '1,1,1'
}
