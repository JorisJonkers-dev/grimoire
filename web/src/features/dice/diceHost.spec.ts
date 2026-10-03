import { flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { jsonResponse, mountWithQuery } from '@/test/mountWithQuery'
import type { ShownRoll } from './choreography'
import DiceHost from './DiceHost.vue'
import { throwDice } from './stage'

const ember = {
  id: '0190c7a8-0000-7000-8000-000000000041', name: 'Ember', hasImage: false, sharing: 'private', review: 'none', mine: true, copy: false, by: 'aria',
  updatedAt: '2026-10-03T10:00:00Z', design: { dice: { d20: { pattern: 'marble', body: '#102030', numbers: '#fafafa' } } },
}
const roll: ShownRoll = { roller: 'Bram', purpose: 'Athletics', dice: [{ faces: 20, value: 14, kept: true }], modifier: 0, total: 14 }

describe('the dice host', () => {
  it('throws the viewer\'s own roll in their Dice Set, and anyone else\'s in the look it came with', async () => {
    const w = mountWithQuery(DiceHost, vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse({ items: [ember], chosen: ember.id }))))
    await flushPromises()
    const tint = () => (w.element as Element).querySelector('[data-testid="dice-2d"] [data-testid="die-tint"]')?.getAttribute('fill')
    throwDice(roll, 'host-1')
    await flushPromises()
    expect(w.get('[data-testid="dice-total"]').text()).toBe('14')
    expect(tint()).toBeUndefined()
    throwDice({ ...roll, look: { dice: { d20: { pattern: 'plain', body: '#ffffff', numbers: '#000000' } } } }, 'host-2')
    await flushPromises()
    expect(tint()).toBe('#ffffff')
    throwDice({ ...roll, roller: 'Aria' }, 'host-3', true)
    await flushPromises()
    expect(tint()).toBe('#102030')
    // The same roll comes round again from the table: it is not thrown twice.
    throwDice(roll, 'host-3')
    await flushPromises()
    expect(w.get('[data-testid="dice-result"]').text()).toContain('Aria')
    w.unmount()

    // On the plain dice the viewer's own roll is plain.
    const plain = mountWithQuery(DiceHost, vi.fn<typeof fetch>(() => Promise.resolve(jsonResponse({ items: [ember] }))))
    await flushPromises()
    throwDice(roll, 'host-4', true)
    await flushPromises()
    expect((plain.element as Element).querySelector('[data-testid="die-tint"]')).toBeNull()
    plain.unmount()
  })
})
