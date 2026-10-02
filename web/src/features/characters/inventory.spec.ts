import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const CH = '0190c7a8-0000-7000-8000-000000000009'
const INES = '0190c7a8-0000-7000-8000-0000000000a1'
const base = `/api/v1/campaigns/${ID}/characters/${CH}/inventory`
const slots = ['head', 'cloak', 'neck', 'armor', 'hands', 'ring_1', 'ring_2', 'feet', 'main_hand', 'off_hand', 'ranged_main', 'ammunition', 'instrument']
const card = (slug: string, name: string, category: string, extra = {}) => ({
  slug, name, category, quantity: 1, weightLb: 1, identified: true, attuned: false, fits: [] as string[], ...extra,
})
const MAIL = '0190c7a8-0000-7000-8000-0000000000b1'
const mail = card('chain-mail', 'Chain Mail', 'armor', { instanceId: MAIL, slot: 'armor', weightLb: 55, fits: ['armor'] })
const view = (extra = {}) => ({
  characterId: CH, name: 'Kara',
  slots: slots.map((slot) => (slot === 'armor' ? { slot, item: mail } : { slot })),
  bag: [
    card('potion-of-healing', 'Potion of Healing', 'potion', { quantity: 2, weightLb: 0.5 }),
    card('longsword', 'Longsword', 'weapon', { weightLb: 3, fits: ['main_hand', 'off_hand', 'ranged_main'] }),
    card('rope', 'Rope', 'adventuring-gear', { weightLb: 5 }),
  ],
  coins: [{ coin: 'gp', count: 12 }], weightLb: 64, capacityLb: 240, load: 'none',
  stash: [card('shield', 'Shield', 'armor', { weightLb: 6, fits: ['off_hand'] })], stashCoins: [],
  party: [{ characterId: INES, name: 'Ines' }],
  ...extra,
})

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('inventory screen', () => {
  it('shows slots, bag and stash, and moves items by buttons and by dragging', async () => {
    const sent: { path: string; body: unknown }[] = []
    const record = (path: string) => async (_u: URL, req: Request) => {
      sent.push({ path, body: await req.clone().json() })
      return view()
    }
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/inventory`, {
      [`${base}/move`]: record('move'),
      [`${base}/take`]: record('take'),
      [`${base}/use`]: async (_u, req) => {
        const body = (await req.clone().json()) as { use: string }
        sent.push({ path: 'use', body })
        return { inventory: view(), healed: body.use === 'drink' ? 7 : 0 }
      },
      [base]: () => view(),
    })
    expect(wrapper.get('[data-testid="carrying"]').text()).toContain('64.0 / 240 lb · Carrying comfortably')
    expect(wrapper.get('[data-testid="slot-armor"]').text()).toContain('Chain Mail')
    await expectAccessible(wrapper.element as Element)

    await wrapper.get('[data-testid="bag-filter"]').setValue('sword')
    expect(wrapper.findAll('[data-testid="bag"] li').map((l) => l.attributes('data-testid'))).toEqual(['item-longsword'])
    await wrapper.get('[data-testid="bag-filter"]').setValue('')
    await wrapper.get('[data-testid="bag-sort"]').setValue('weight')
    expect(wrapper.findAll('[data-testid="bag"] li').map((l) => l.attributes('data-testid'))).toEqual(['item-rope', 'item-longsword', 'item-potion-of-healing'])
    await wrapper.get('[data-testid="bag-sort"]').setValue('category')
    expect(wrapper.findAll('[data-testid="bag"] li')[0]?.attributes('data-testid')).toBe('item-rope')

    await wrapper.get('[data-testid="equip-longsword"]').setValue('main_hand')
    await wrapper.get('[data-testid="drink-potion-of-healing"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="inventory-status"]').text()).toBe('Potion of Healing: 7 hit points back.')
    await wrapper.get('[data-testid="give-to-rope"]').setValue(INES)
    await wrapper.get('[data-testid="give-rope"]').trigger('click')
    await wrapper.get('[data-testid="stash-rope"]').trigger('click')
    await wrapper.get('[data-testid="throw-rope"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="inventory-status"]').text()).toBe('You throw away Rope.')
    await wrapper.get('[data-testid="unequip-armor"]').trigger('click')
    await wrapper.get('[data-testid="take-shield"]').trigger('click')

    for (const zone of ['slot-off_hand', 'bag', 'stash', 'ally-Ines']) await wrapper.get(`[data-testid="${zone}"]`).trigger('dragover')
    await wrapper.get('[data-testid="item-longsword"]').trigger('dragstart')
    await wrapper.get('[data-testid="slot-off_hand"]').trigger('drop')
    await wrapper.get('[data-testid="item-rope"]').trigger('dragstart')
    await wrapper.get('[data-testid="slot-head"]').trigger('drop')
    expect(wrapper.get('[data-testid="inventory-status"]').text()).toBe('Rope does not go there.')
    await wrapper.get('[data-testid="item-rope"]').trigger('dragstart')
    await wrapper.get('[data-testid="ally-Ines"]').trigger('drop')
    await wrapper.get('[data-testid="worn-armor"]').trigger('dragstart')
    await wrapper.get('[data-testid="stash"]').trigger('drop')
    await wrapper.get('[data-testid="worn-armor"]').trigger('dragstart')
    await wrapper.get('[data-testid="bag"]').trigger('drop')
    await wrapper.get('[data-testid="stash-item-shield"]').trigger('dragstart')
    await wrapper.get('[data-testid="bag"]').trigger('drop')
    await wrapper.get('[data-testid="stash-item-shield"]').trigger('dragstart')
    await wrapper.get('[data-testid="slot-off_hand"]').trigger('drop')
    await wrapper.get('[data-testid="bag"]').trigger('drop')
    await flushPromises()
    expect(sent).toEqual([
      { path: 'move', body: { slug: 'longsword', to: 'slot', count: 1, slot: 'main_hand' } },
      { path: 'use', body: { slug: 'potion-of-healing', use: 'drink' } },
      { path: 'move', body: { slug: 'rope', to: 'character', count: 1, characterId: INES } },
      { path: 'move', body: { slug: 'rope', to: 'stash', count: 1 } },
      { path: 'use', body: { slug: 'rope', use: 'throw' } },
      { path: 'move', body: { instanceId: MAIL, to: 'bag', count: 1 } },
      { path: 'take', body: { slug: 'shield', count: 1 } },
      { path: 'move', body: { slug: 'longsword', to: 'slot', count: 1, slot: 'off_hand' } },
      { path: 'move', body: { slug: 'rope', to: 'character', count: 1, characterId: INES } },
      { path: 'move', body: { instanceId: MAIL, to: 'stash', count: 1 } },
      { path: 'move', body: { instanceId: MAIL, to: 'bag', count: 1 } },
      { path: 'take', body: { slug: 'shield', count: 1 } },
    ])
  })

  it('warns when overloaded, reports refusals, drinks without healing and handles a missing Inventory', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/inventory`, {
      [`${base}/use`]: () => ({ inventory: view(), healed: 0 }),
      [`${base}/move`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'a Session is live: move items from its Inventory panel' }, 422),
      [base]: () => view({ load: 'encumbered', weightLb: 300, coins: [], bag: [card('potion-of-heroism', 'Potion of Heroism', 'potion')], stash: [], party: [] }),
    })
    expect(wrapper.get('[data-testid="carrying"]').text()).toContain('Over capacity: speed drops to 5 feet')
    expect(wrapper.text()).toContain('No coins')
    expect(wrapper.text()).toContain('The stash is empty.')
    await wrapper.get('[data-testid="drink-potion-of-heroism"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="inventory-status"]').text()).toBe('You drink Potion of Heroism.')
    await wrapper.get('[data-testid="stash-potion-of-heroism"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="inventory-problem"]').text()).toContain('a Session is live')
    await wrapper.get('[data-testid="bag"]').trigger('drop')
    unmountAll()
    const heavy = await mountApp(`/campaigns/${ID}/characters/${CH}/inventory`, {
      [base]: () => view({ load: 'immobile', bag: [] }),
    })
    expect(heavy.wrapper.text()).toContain('Too heavy to move')
    expect(heavy.wrapper.text()).toContain('Nothing here.')
    unmountAll()
    const missing = await mountApp(`/campaigns/${ID}/characters/${CH}/inventory`, {})
    expect(missing.wrapper.find('[data-testid="inventory-error"]').exists()).toBe(true)
  })
})

describe('magic items on the inventory screen', () => {
  it('attunes, identifies and spends charges', async () => {
    const UNKNOWN = '0190c7a8-0000-7000-8000-0000000000e1'
    const sent: unknown[] = []
    const magic = view({
      bag: [
        card('amulet-of-health', 'Amulet of Health', 'wondrous-item', { requiresAttunement: true, fits: ['neck'] }),
        card('ring-of-protection', 'Ring of Protection', 'ring', { instanceId: '0190c7a8-0000-7000-8000-0000000000e2', requiresAttunement: true, attuned: true, fits: ['ring_1', 'ring_2'] }),
        card('wand-of-magic-missiles', 'Wand of Magic Missiles', 'wand', { instanceId: '0190c7a8-0000-7000-8000-0000000000e3', maxCharges: 7, charges: 5 }),
        card('unknown', 'Unknown wand', 'wand', { instanceId: UNKNOWN, identified: false }),
      ],
    })
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/inventory`, {
      [`${base}/use`]: async (_u, req) => {
        sent.push(await req.clone().json())
        return { inventory: magic, healed: 0 }
      },
      [base]: () => magic,
    })
    expect(wrapper.get('[data-testid="carrying"]').text()).toContain('Attuned to 1 of 3')
    expect(wrapper.get('[data-testid="item-wand-of-magic-missiles"]').text()).toContain('5/7 charges')
    expect(wrapper.get('[data-testid="item-unknown"]').text()).toContain('Unknown wand')
    await expectAccessible(wrapper.element as Element)
    for (const button of ['attune-amulet-of-health', 'unattune-ring-of-protection', `identify-${UNKNOWN}`, 'charge-wand-of-magic-missiles']) {
      await wrapper.get(`[data-testid="${button}"]`).trigger('click')
      await flushPromises()
    }
    expect(wrapper.get('[data-testid="inventory-status"]').text()).toBe('You spend a charge of Wand of Magic Missiles.')
    expect(sent).toEqual([
      { slug: 'amulet-of-health', use: 'attune' },
      { instanceId: '0190c7a8-0000-7000-8000-0000000000e2', use: 'unattune' },
      { instanceId: UNKNOWN, use: 'identify' },
      { instanceId: '0190c7a8-0000-7000-8000-0000000000e3', use: 'charge' },
    ])
  })
})
