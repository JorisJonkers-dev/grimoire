import { describe, expect, it } from 'vitest'
import { breakdown, criticalOf, shownOf, sidesOf, throwPlan } from './choreography'

const roll = (dice: { faces: number; value: number; kept: boolean }[], modifier = 0) => ({
  roller: 'Aria', purpose: 'Attack', dice, modifier, total: dice.filter((d) => d.kept).reduce((n, d) => n + d.value, 0) + modifier,
})

describe('dice choreography', () => {
  it('marks a natural 20 as a critical hit and a natural 1 as a critical miss, on the d20 that counts', () => {
    expect(criticalOf(roll([{ faces: 20, value: 20, kept: true }]))).toBe('hit')
    expect(criticalOf(roll([{ faces: 20, value: 1, kept: true }]))).toBe('miss')
    expect(criticalOf(roll([{ faces: 20, value: 19, kept: true }]))).toBeNull()
    // With advantage the dropped die does not count, whatever it shows.
    expect(criticalOf(roll([{ faces: 20, value: 1, kept: false }, { faces: 20, value: 12, kept: true }]))).toBeNull()
    expect(criticalOf(roll([{ faces: 20, value: 20, kept: false }, { faces: 20, value: 20, kept: true }]))).toBe('hit')
    // Only a d20 can be critical: a 1 on a d6 or a 20 on a d100 is just a number.
    expect(criticalOf(roll([{ faces: 6, value: 1, kept: true }]))).toBeNull()
    expect(criticalOf(roll([{ faces: 100, value: 20, kept: true }]))).toBeNull()
    expect(criticalOf(roll([{ faces: 20, value: 20, kept: true }, { faces: 20, value: 1, kept: true }]))).toBe('hit')
    expect(criticalOf(roll([]))).toBeNull()
  })

  it('writes the breakdown beneath the total: kept and dropped dice, then the modifier', () => {
    expect(breakdown(roll([{ faces: 20, value: 14, kept: true }], 3))).toBe('14 + 3')
    expect(breakdown(roll([{ faces: 20, value: 14, kept: true }, { faces: 20, value: 3, kept: false }], -1))).toBe('14, 3 dropped − 1')
    expect(breakdown(roll([{ faces: 8, value: 6, kept: true }, { faces: 6, value: 2, kept: true }]))).toBe('6 + 2')
    expect(breakdown(roll([], 4))).toBe('+ 4')
  })

  it('reads a resolved Roll Request: in a group that keeps the highest or lowest only the kept die counts, elsewhere every die', () => {
    const request = {
      purpose: 'Longsword attack', roller: { name: 'Aria' }, total: 21,
      groups: [{ index: 0, keep: 'highest' }, { index: 1 }],
      dice: [{ group: 0, faces: 20, value: 14, kept: true }, { group: 0, faces: 20, value: 3, kept: false }, { group: 1, faces: 4, value: 2, kept: false }],
      modifiers: [{ value: 5 }],
    }
    expect(shownOf(request)).toEqual({
      roller: 'Aria', purpose: 'Longsword attack', modifier: 5, total: 21,
      dice: [{ faces: 20, value: 14, kept: true }, { faces: 20, value: 3, kept: false }, { faces: 4, value: 2, kept: true }],
    })
    expect(shownOf({ ...request, total: undefined, dice: [{ group: 1, faces: 6, kept: false }], modifiers: [] })).toMatchObject({ total: 0, modifier: 0, dice: [{ faces: 6, value: 0, kept: true }] })
  })

  it('draws each die as the nearest shape it has an icon for', () => {
    expect([4, 6, 8, 10, 12, 20, 100].map(sidesOf)).toEqual([4, 6, 8, 10, 12, 20, 100])
    expect([2, 3, 7, 30].map(sidesOf)).toEqual([6, 6, 6, 6])
  })

  it('plans the same throw for the same seed: every die in from an edge, to a place of its own near the centre', () => {
    const dice = roll([{ faces: 20, value: 14, kept: true }, { faces: 20, value: 3, kept: false }, { faces: 6, value: 2, kept: true }]).dice
    const plan = throwPlan(dice, 7, 16 / 9)
    expect(throwPlan(dice, 7, 16 / 9)).toEqual(plan)
    expect(throwPlan(dice, 8, 16 / 9)).not.toEqual(plan)
    expect(plan).toHaveLength(3)
    const half = { x: (16 / 9) * 5, y: 5 }
    for (const p of plan) {
      expect(Math.abs(p.from.x) > half.x || Math.abs(p.from.y) > half.y).toBe(true)
      expect(Math.abs(p.rest.x)).toBeLessThan(half.x * 0.6)
      expect(Math.abs(p.rest.y)).toBeLessThan(half.y * 0.6)
      expect(p.spin).toBeGreaterThan(0)
    }
    // Resting places do not overlap, and the dice gather into one centred row in the order rolled.
    for (const [i, a] of plan.entries()) for (const b of plan.slice(i + 1)) expect(Math.hypot(a.rest.x - b.rest.x, a.rest.y - b.rest.y)).toBeGreaterThan(1.9)
    expect(plan.map((p) => p.gather.x)).toEqual([-2.2, 0, 2.2])
    expect(plan.every((p) => p.gather.y === 0)).toBe(true)
    expect(throwPlan([], 1, 1)).toEqual([])
  })
})
