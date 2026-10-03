import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { ItemBuild, ItemDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ITEM = '0190c7a8-0000-7000-8000-0000000000a1'
const at = '2026-10-02T20:00:00Z'
const start: ItemDesign = { kind: 'weapon', base: 'longbow', rarity: 'uncommon', enchantment: 0, weightLb: 2, valueGp: 400, properties: [] }
const entry = { id: ITEM, kind: 'item' as const, name: 'Ashwood Bow', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: ItemDesign, extra: Partial<ItemBuild> = {}): ItemBuild => ({
  entry,
  design,
  slug: 'hb-0190c7a80000',
  card: ['Ashwood Bow', 'Weapon (longbow), uncommon'],
  price: { points: 1, suggested: 'uncommon', priceGp: 400, fits: true, notes: [] },
  ...extra,
})

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('item builder', () => {
  it('builds the Ashwood Bow from rows, previews its card and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${ITEM}/item`, {
      '/api/v1/builders/items/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: ItemDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, {
          card: ['Ashwood Bow', 'Weapon (longbow), rare (requires attunement by a ranger)', 'Charges: 3, regains 1d4 at dawn.', 'Spell: Hunter\'s Mark (1 charge).'],
          price: { points: 6, suggested: 'very_rare', priceGp: 4000, fits: false, notes: ['The properties point to very rare.'] },
        })
      },
      [`/api/v1/builders/items/${ITEM}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as ItemDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    expect(wrapper.get('h1').text()).toContain('Ashwood Bow')
    expect(wrapper.get('[data-testid="price-fits"]').text()).toBe('Fits')
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    await set('item-kind', 'ring')
    await set('item-kind', 'weapon')
    await set('item-base', 'ashwood-longbow')
    await set('item-base', 'longbow')
    await set('item-weight', 3)
    await set('item-weight', 2)
    await set('item-rarity', 'rare')
    await set('item-enchantment', 1)
    await set('item-value', 4000)
    await set('item-attunes', true)
    await set('attune-kind', 'class')
    await set('attune-value', 'ranger')
    await set('item-charged', true)
    await set('charges-max', 3)
    await set('charges-on', 'long_rest')
    await set('charges-dice', 2)
    await set('charges-faces', 6)
    await set('charges-bonus', 1)
    await set('item-weapon', true)
    await wrapper.get('[data-testid="weapon-ammunition"]').trigger('change')
    await wrapper.get('[data-testid="weapon-heavy"]').trigger('change')
    await wrapper.get('[data-testid="weapon-two-handed"]').trigger('change')
    await wrapper.get('[data-testid="weapon-heavy"]').trigger('change')
    await set('weapon-mastery', 'custom')
    await set('weapon-custom', 'The target glows until your next turn.')
    await set('weapon-mastery', 'slow')
    await wrapper.get('[data-testid="quick-boost"]').trigger('click')
    await wrapper.get('[data-testid="quick-cantrip"]').trigger('click')
    await set('add-property', 'spell')
    await wrapper.get('[data-testid="add-row"]').trigger('click')
    await set('add-property', 'curse')
    await wrapper.get('[data-testid="add-row"]').trigger('click')
    await set('row-0-skill', 'survival')
    await set('row-0-value', 2)
    await set('row-0-hidden', true)
    await set('row-2-name', "Hunter's Mark")
    await set('row-2-spell', 'hunters-mark')
    await set('row-3-text', 'You cannot let go of it.')
    await set('row-3-cannotDrop', true)
    await wrapper.get('[data-testid="row-1-remove"]').trigger('click')
    await set('item-charged', false)
    await set('item-charged', true)
    await set('item-attunes', false)
    await set('item-attunes', true)
    await set('attune-kind', 'class')
    await set('attune-value', 'ranger')
    expect(wrapper.findAll('[data-testid="item-rows"] .kind').map((k) => k.text())).toEqual(['skill boost', 'spell', 'curse'])
    await wrapper.get('[data-testid="item-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="item-card"]').text()).toContain('Spell: Hunter\'s Mark (1 charge).')
    expect(wrapper.get('[data-testid="price-fits"]').text()).toBe('Check it')
    expect(wrapper.get('[data-testid="price-check"]').text()).toContain('The properties point to very rare.')
    expect((sent[0]?.body as { name: string }).name).toBe('Ashwood Bow')
    await wrapper.get('[data-testid="item-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="item-status"]').text()).toBe('Saved as Revision 2.')
    const saved = sent.find((x) => x.method === 'PUT')?.body as ItemDesign
    expect(saved).toEqual({
      kind: 'weapon', base: 'longbow', rarity: 'rare', enchantment: 1, weightLb: 2, valueGp: 4000,
      attunement: { kind: 'class', value: 'ranger' },
      weapon: { properties: ['ammunition', 'two-handed'], mastery: 'slow', custom: 'The target glows until your next turn.' },
      charges: { max: 3, on: 'dawn', dice: 1, faces: 4 },
      properties: [
        { type: 'skill_boost', skill: 'survival', mode: 'advantage', value: 2, hidden: true },
        { type: 'spell', name: "Hunter's Mark", spell: 'hunters-mark', level: 1, cost: 1 },
        { type: 'curse', text: 'You cannot let go of it.', cannotDrop: true },
      ],
    })
  })

  it('shows a Shared Library copy without saving, reports a design that will not build, and refuses what is not an item', async () => {
    const { wrapper } = await mountApp(`/library/${ITEM}/item`, {
      '/api/v1/builders/items/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'a spell row needs a spell' }, 422),
      [`/api/v1/builders/items/${ITEM}`]: () => build({ ...start, kind: 'ring', properties: [{ type: 'manual', text: 'Hums.' }, { type: 'unknown' }] }, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="item-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="item-save"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="item-weapon"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="row-0-text"]').element).toHaveProperty('value', 'Hums.')
    await wrapper.get('[data-testid="item-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="item-problem"]').text()).toBe('a spell row needs a spell')
    const refused = await mountApp(`/library/${ITEM}/item`, {
      [`/api/v1/builders/items/${ITEM}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="item-error"]').text()).toContain('cannot be opened')
  })

  it('opens an item or a spell in its builder from the Library entry', async () => {
    const routes = {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/items/${ITEM}`]: () => build(start),
      [`/api/v1/library/${ITEM}`]: () => ({ entry, revisions: [], uses: [] }),
    }
    const { wrapper } = await mountApp(`/library/${ITEM}`, routes)
    expect(wrapper.find('[data-testid="open-builder"]').exists()).toBe(false)
    await wrapper.get('[data-testid="open-item-builder"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('h1').text()).toContain('Item builder')
    const spell = await mountApp(`/library/${ITEM}`, { ...routes, [`/api/v1/library/${ITEM}`]: () => ({ entry: { ...entry, kind: 'spell' }, revisions: [], uses: [] }) })
    expect(spell.wrapper.find('[data-testid="open-item-builder"]').exists()).toBe(false)
    expect(spell.wrapper.get('[data-testid="open-builder"]').attributes('href')).toBe(`/library/${ITEM}/spell`)
  })
})
