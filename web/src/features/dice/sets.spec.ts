import { describe, expect, it } from 'vitest'
import type { DiceSet } from '@/infrastructure/api/types.gen'
import { CENTRED, PLAIN, cellOf, everyDie, lookFor, lookOf, pictureStyle, reviewNote, sheetCols } from './sets'

const set: DiceSet = {
  id: '0190c7a8-0000-7000-8000-000000000041', name: 'Ember', hasImage: false, sharing: 'private', review: 'none', mine: true, copy: false, by: 'aria', updatedAt: '2026-10-03T10:00:00Z',
  design: { dice: { d20: { pattern: 'marble', body: '#102030', numbers: '#ffffff' } } },
}

describe('a Dice Set', () => {
  it('lays the unwrapped faces of a die out in a square grid', () => {
    expect([4, 6, 8, 10, 12, 20].map(sheetCols)).toEqual([2, 3, 3, 4, 4, 5])
    expect([0, 4, 5, 19].map((i) => cellOf(i, 5))).toEqual([{ col: 0, row: 0 }, { col: 4, row: 0 }, { col: 0, row: 1 }, { col: 4, row: 3 }])
  })

  it('dresses only the dice it has a look for', () => {
    const look = lookOf(set)
    expect(look).toEqual({ dice: set.design.dice })
    expect(lookFor(look, 20)).toEqual(set.design.dice.d20)
    expect(lookFor(look, 6)).toBeUndefined()
    expect(lookFor(look, 7)).toBeUndefined()
    expect(lookFor(undefined, 20)).toBeUndefined()
    expect(lookOf({ ...set, imageUrl: '/api/v1/dice-sets/x/image?v=1' }).imageUrl).toBe('/api/v1/dice-sets/x/image?v=1')
    const all = everyDie(set.design.dice)
    expect(Object.keys(all)).toEqual(['d4', 'd6', 'd8', 'd10', 'd12', 'd20', 'd100'])
    expect([all.d20.pattern, all.d6]).toEqual(['marble', PLAIN])
    // Each die gets a look of its own to edit, never the shared plain one.
    expect(all.d6).not.toBe(PLAIN)
  })

  it('puts a picture where it was placed on the sheet', () => {
    expect(pictureStyle(CENTRED)).toEqual({ left: '50%', top: '50%', width: '100%', transform: 'translate(-50%, -50%) rotate(0deg)' })
    expect(pictureStyle({ x: 0.125, y: 1, scale: 2.5, rotation: -45 })).toEqual({ left: '12.5%', top: '100%', width: '250%', transform: 'translate(-50%, -50%) rotate(-45deg)' })
  })

  it('tells its owner where a set shared with everyone stands', () => {
    const everyone = { ...set, sharing: 'everyone' as const }
    expect(reviewNote(set)).toBe('')
    expect(reviewNote(everyone)).toBe('')
    expect(reviewNote({ ...everyone, review: 'pending' })).toContain('Waiting for an Admin')
    expect(reviewNote({ ...everyone, review: 'approved' })).toContain('approved')
    expect(reviewNote({ ...everyone, review: 'rejected' })).toContain('turned its picture down')
    // A copy carries its original's approval, which says nothing about the copy's owner.
    expect(reviewNote({ ...set, copy: true, review: 'approved' })).toBe('')
    expect(reviewNote({ ...set, sharing: 'friends', review: 'pending' })).toBe('')
  })
})
