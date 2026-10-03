import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { RollTableBuild, RollTableDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const TABLE = '0190c7a8-0000-7000-8000-0000000000f7'
const at = '2026-10-03T20:00:00Z'
const start: RollTableDesign = { dice: '1d20', results: [] }
const entry = { id: TABLE, kind: 'table' as const, name: 'Fumbles', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: RollTableDesign, extra: Partial<RollTableBuild> = {}): RollTableBuild => ({ entry, design, lines: ['Roll 1d20.'], ...extra })

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('roll table builder', () => {
  it('builds a fumble table: dice, ranges, words, an Effect and an Item, previewed and saved', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${TABLE}/table`, {
      '/api/v1/builders/roll-tables/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: RollTableDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { lines: ['Roll 1d6.', '1: You fall flat on your face. Applies prone.'] })
      },
      [`/api/v1/builders/roll-tables/${TABLE}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as RollTableDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    expect(wrapper.get('h1').text()).toContain('Roll Table builder')
    expect(wrapper.get('[data-testid="table-lines"]').text()).toContain('Roll 1d20.')
    const set = (id: string, value: string | number) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    const value = (id: string) => (wrapper.get(`[data-testid="${id}"]`).element as HTMLInputElement).value
    await set('table-dice', '1d6')
    // Each new result starts where the last one ended.
    await wrapper.get('[data-testid="table-add-result"]').trigger('click')
    expect([value('table-result-0-from'), value('table-result-0-to')]).toEqual(['1', '1'])
    await set('table-result-0-text', 'You fall flat on your face.')
    await set('table-result-0-effect', 'prone')
    await wrapper.get('[data-testid="table-add-result"]').trigger('click')
    expect([value('table-result-1-from'), value('table-result-1-to')]).toEqual(['2', '2'])
    await set('table-result-1-to', 3)
    await set('table-result-1-text', 'Your weapon slips from your grip.')
    await wrapper.get('[data-testid="table-add-result"]').trigger('click')
    expect(value('table-result-2-from')).toBe('4')
    await set('table-result-2-from', 5)
    await set('table-result-2-to', 6)
    await set('table-result-2-text', 'You find a coin.')
    // How many of an Item is asked only once there is an Item, and goes when the Item does.
    expect(wrapper.find('[data-testid="table-result-2-quantity"]').exists()).toBe(false)
    await set('table-result-2-item', 'gold-piece')
    await set('table-result-2-quantity', 2)
    await set('table-result-1-item', 'dagger')
    await set('table-result-1-quantity', 3)
    await set('table-result-1-item', '')
    expect(wrapper.find('[data-testid="table-result-1-quantity"]').exists()).toBe(false)
    await wrapper.get('[data-testid="table-add-result"]').trigger('click')
    await wrapper.get('[data-testid="table-result-3-remove"]').trigger('click')
    expect(wrapper.find('[data-testid="table-result-3-text"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)

    await wrapper.get('[data-testid="table-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="table-lines"]').text()).toContain('1: You fall flat on your face. Applies prone.')
    expect(sent.at(-1)).toMatchObject({ method: 'PREVIEW', body: { name: 'Fumbles', design: { dice: '1d6' } } })
    await wrapper.get('[data-testid="table-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="table-status"]').text()).toBe('Saved as Revision 2.')
    expect(sent.find((x) => x.method === 'PUT')?.body).toEqual({
      dice: '1d6',
      results: [
        { from: 1, to: 1, text: 'You fall flat on your face.', effect: 'prone' },
        { from: 2, to: 3, text: 'Your weapon slips from your grip.', item: '' },
        { from: 5, to: 6, text: 'You find a coin.', item: 'gold-piece', quantity: 2 },
      ],
    })
  })

  it('shows a Shared Library copy, reports a design that will not do, and opens from the entry', async () => {
    const design: RollTableDesign = { dice: '1d6', results: [{ from: 1, to: 6, text: 'Something.', item: 'rope', quantity: 4 }] }
    const { wrapper } = await mountApp(`/library/${TABLE}/table`, {
      '/api/v1/builders/roll-tables/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'The ranges go in order and do not overlap.' }, 422),
      [`/api/v1/builders/roll-tables/${TABLE}`]: () => build(design, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="table-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="table-save"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="table-result-0-quantity"]').element as HTMLInputElement).value).toBe('4')
    await wrapper.get('[data-testid="table-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="table-problem"]').text()).toBe('The ranges go in order and do not overlap.')
    // On a table with results, a new one starts after the last.
    await wrapper.get('[data-testid="table-add-result"]').trigger('click')
    expect((wrapper.get('[data-testid="table-result-1-from"]').element as HTMLInputElement).value).toBe('7')
    const refused = await mountApp(`/library/${TABLE}/table`, {
      [`/api/v1/builders/roll-tables/${TABLE}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="table-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${TABLE}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/roll-tables/${TABLE}`]: () => build(start),
      [`/api/v1/library/${TABLE}`]: () => ({ entry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-table-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Roll Table builder')
  })

  it('says a problem with no detail in its own words', async () => {
    const { wrapper } = await mountApp(`/library/${TABLE}/table`, {
      '/api/v1/builders/roll-tables/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
      [`/api/v1/builders/roll-tables/${TABLE}`]: () => build(start),
    })
    await wrapper.get('[data-testid="table-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="table-problem"]').text()).toBe('That table will not do.')
  })
})
