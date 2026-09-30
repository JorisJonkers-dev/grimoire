import { describe, expect, it } from 'vitest'
import { baseValid, bonusFor, bonusValid, defaultBase, modifier, pointCost, pointsSpent, signed } from './build'

describe('build helpers', () => {
  it('computes modifiers and signs', () => {
    expect([modifier(8), modifier(10), modifier(15), modifier(20)]).toEqual([-1, 0, 2, 5])
    expect([signed(2), signed(0), signed(-1)]).toEqual(['+2', '+0', '-1'])
  })

  it('prices point buy and checks each method', () => {
    expect([pointCost(8), pointCost(13), pointCost(14), pointCost(15)]).toEqual([0, 5, 7, 9])
    const array = defaultBase('standard-array')
    expect(baseValid('standard-array', array, 27)).toBe(true)
    expect(baseValid('standard-array', { ...array, strength: 14 }, 27)).toBe(false)
    const bought = defaultBase('point-buy')
    expect(pointsSpent(bought)).toBe(0)
    expect(baseValid('point-buy', { ...bought, strength: 15, dexterity: 15, constitution: 15 }, 27)).toBe(true)
    expect(baseValid('point-buy', { ...bought, strength: 15, dexterity: 15, constitution: 15, wisdom: 15 }, 27)).toBe(false)
    expect(baseValid('point-buy', { ...bought, strength: 16 }, 27)).toBe(false)
    expect(baseValid('rolled', defaultBase('rolled'), 27)).toBe(true)
    expect(baseValid('rolled', { ...defaultBase('rolled'), wisdom: 19 }, 27)).toBe(false)
  })

  it('builds origin increases', () => {
    expect(bonusFor('two-one', ['strength', 'constitution'])).toEqual({ strength: 2, constitution: 1 })
    expect(bonusFor('two-one', ['strength', 'strength'])).toEqual({ strength: 2 })
    expect(bonusFor('two-one', [])).toEqual({})
    expect(bonusFor('one-one-one', ['strength', 'dexterity', 'constitution', 'wisdom'])).toEqual({ strength: 1, dexterity: 1, constitution: 1 })
    expect(bonusValid({ strength: 2, wisdom: 1 })).toBe(true)
    expect(bonusValid({ strength: 1, wisdom: 1, charisma: 1 })).toBe(true)
    expect(bonusValid({ strength: 2 })).toBe(false)
  })
})
