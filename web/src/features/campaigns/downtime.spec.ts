import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Downtime } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const KARA = '0190c7a8-0000-7000-8000-0000000000c1'
const INES = '0190c7a8-0000-7000-8000-0000000000c2'
const SALVE = '0190c7a8-0000-7000-8000-0000000000d1'
const SPOON = '0190c7a8-0000-7000-8000-0000000000d2'
const at = '2026-10-03T10:00:00Z'
const downtime = (dm: boolean): Downtime => ({
  dm, gameDay: 12, gameMinute: 480,
  characters: [{ id: INES, name: 'Ines', days: 7, mine: dm }, { id: KARA, name: 'Kara', days: 1, mine: true }],
  recipes: [
    { id: SALVE, name: 'Healing salve', makes: 'healing-salve', quantity: 2, tool: 'herbalism-kit', days: 3, costCp: 2550, ingredients: [{ item: 'healing-herb', count: 2 }, { item: 'vial', count: 1 }] },
    { id: SPOON, name: 'Whittling', makes: 'wooden-spoon', quantity: 1, days: 1, costCp: 0, ingredients: [] },
  ],
  log: [
    { id: '0190c7a8-0000-7000-8000-0000000000e1', character: 'Kara', activity: 'craft', detail: 'Healing salve', days: 3, at },
    { id: '0190c7a8-0000-7000-8000-0000000000e2', character: 'Ines', activity: 'work', detail: '', days: 1, at },
    { id: '0190c7a8-0000-7000-8000-0000000000e3', character: 'Ines', activity: 'train', detail: "Thieves' tools", days: 2, at },
    { id: '0190c7a8-0000-7000-8000-0000000000e4', character: 'Kara', activity: 'research', detail: 'The Ashen Hand', days: 1, at },
  ],
})

afterEach(() => { unmountAll() })

function backend(dm: boolean) {
  const calls: string[] = []
  const state = { fail: 0, detail: 'It needs a herbalism-kit at hand.' as string | undefined }
  const failing = () => jsonResponse({ type: 'about:blank', title: 'No', status: state.fail, detail: state.detail }, state.fail)
  const write = (answer: () => Response) => async (url: URL, req: Request) => {
    calls.push(`${req.method} ${url.pathname.split('/').slice(5).join('/')} ${req.method === 'DELETE' ? '' : await req.text()}`.trim())
    return state.fail ? failing() : answer()
  }
  const routes = {
    [`/api/v1/campaigns/${ID}/downtime/grants`]: write(() => jsonResponse(downtime(dm))),
    [`/api/v1/campaigns/${ID}/downtime`]: () => downtime(dm),
    [`/api/v1/campaigns/${ID}/characters/`]: write(() => jsonResponse(downtime(dm))),
    [`/api/v1/campaigns/${ID}/recipes/`]: write(() => new Response(null, { status: 204 })),
    [`/api/v1/campaigns/${ID}/recipes`]: write(() => jsonResponse(downtime(dm).recipes[0], 201)),
  }
  return { calls, routes, state }
}

describe('downtime', () => {
  it('shows the clock, the days each Character has, the Recipes and what was done', async () => {
    const { routes } = backend(false)
    const { wrapper } = await mountApp(`/campaigns/${ID}/downtime`, routes)
    await flushPromises()
    expect(wrapper.get('[data-testid="downtime-clock"]').text()).toBe('The Game Clock stands at day 12, 08:00.')
    expect(wrapper.findAll('[data-testid="downtime-days"]').map((d) => d.text())).toEqual(['Ines: 7 downtime days', 'Kara: 1 downtime day'])
    expect(wrapper.findAll('[data-testid^="recipe-0190"]').map((r) => r.get('[data-testid="recipe-says"]').text())).toEqual([
      'Healing salve: makes 2 × healing-salve in 3 days for 25 gp 5 sp, from 2 × healing-herb and 1 × vial, with a herbalism-kit at hand.',
      'Whittling: makes 1 × wooden-spoon in 1 day.',
    ])
    expect(wrapper.findAll('[data-testid="downtime-log"] li').map((l) => l.text())).toEqual([
      'Kara crafted Healing salve over 3 days.',
      'Ines worked for 1 day.',
      "Ines trained in Thieves' tools for 2 days.",
      'Kara researched The Ashen Hand for 1 day.',
    ])
    // A Player spends only their own Character's days, and gives none.
    expect(wrapper.findAll('[data-testid^="spend-"]').map((f) => f.attributes('data-testid'))).toEqual([`spend-${KARA}`])
    for (const id of ['downtime-grant', 'recipe-add', 'recipe-remove']) expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('lets a Player spend days crafting, working, training and researching, and says why not', async () => {
    const { calls, routes, state } = backend(false)
    const { wrapper } = await mountApp(`/campaigns/${ID}/downtime`, routes)
    await flushPromises()
    const form = wrapper.get(`[data-testid="spend-${KARA}"]`)
    const set = (id: string, value: string | number) => form.get(`[data-testid="${id}"]`).setValue(value)
    // Crafting asks for a Recipe, and takes the days the Recipe says.
    expect((form.get('[data-testid="activity"]').element as HTMLSelectElement).value).toBe('craft')
    expect(form.find('[data-testid="days"]').exists()).toBe(false)
    expect(form.find('[data-testid="subject"]').exists()).toBe(false)
    await set('recipe', SPOON)
    await form.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST characters/${KARA}/downtime {"activity":"craft","recipeId":"${SPOON}"}`)
    expect(wrapper.get('[data-testid="downtime-done"]').text()).toBe('Kara spent the time.')
    // Work asks for days alone; training and research for a subject too.
    await set('activity', 'work')
    expect(form.find('[data-testid="recipe"]').exists()).toBe(false)
    expect(form.find('[data-testid="subject"]').exists()).toBe(false)
    await set('days', 2)
    await form.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST characters/${KARA}/downtime {"activity":"work","days":2}`)
    await set('activity', 'train')
    expect(form.get('button').attributes('disabled')).toBeDefined()
    await set('subject', "  Thieves' tools ")
    await form.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST characters/${KARA}/downtime {"activity":"train","days":2,"subject":"Thieves' tools"}`)
    await set('activity', 'research')
    await set('subject', 'The Ashen Hand')
    await set('days', 1)
    await form.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST characters/${KARA}/downtime {"activity":"research","days":1,"subject":"The Ashen Hand"}`)
    // A refusal is shown in the rules' own words, and takes the last news away; with no words given, in the page's.
    state.fail = 422
    await set('activity', 'craft')
    await form.trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="downtime-problem"]').text()).toBe('It needs a herbalism-kit at hand.')
    expect(wrapper.find('[data-testid="downtime-done"]').exists()).toBe(false)
    state.detail = undefined
    await form.trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="downtime-problem"]').text()).toBe('That could not be done.')
    state.fail = 0
    await form.trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-testid="downtime-problem"]').exists()).toBe(false)
  })

  it('lets the DM give downtime, spend any Character\'s days and keep the Recipes', async () => {
    const { calls, routes } = backend(true)
    const { wrapper } = await mountApp(`/campaigns/${ID}/downtime`, routes)
    await flushPromises()
    expect(wrapper.findAll('[data-testid^="spend-"]')).toHaveLength(2)
    const grant = wrapper.get('[data-testid="downtime-grant"]')
    await grant.get('[data-testid="grant-days"]').setValue(5)
    await grant.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST downtime/grants {"days":5}')
    expect(wrapper.get('[data-testid="downtime-done"]').text()).toBe('Everyone has 5 more downtime days.')
    await grant.get('[data-testid="grant-who"]').setValue(KARA)
    await grant.get('[data-testid="grant-days"]').setValue(1)
    await grant.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST downtime/grants {"days":1,"characterId":"${KARA}"}`)
    expect(wrapper.get('[data-testid="downtime-done"]').text()).toBe('Kara has 1 more downtime day.')

    const add = wrapper.get('[data-testid="recipe-add"]')
    const set = (id: string, value: string | number) => add.get(`[data-testid="${id}"]`).setValue(value)
    expect(add.get('button').attributes('disabled')).toBeDefined()
    await set('recipe-name', ' Healing salve ')
    expect(add.get('button').attributes('disabled')).toBeDefined()
    await set('recipe-makes', ' healing-salve ')
    await set('recipe-quantity', 2)
    await set('recipe-days', 3)
    // 0.29 gp is 29 copper, whatever a float makes of it.
    await set('recipe-cost', 0.29)
    await set('recipe-tool', ' herbalism-kit ')
    await set('recipe-ingredients', '2 healing-herb\n\n vial \n3 x  ')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST recipes ${JSON.stringify({
      name: 'Healing salve', makes: 'healing-salve', quantity: 2, days: 3, costCp: 29, tool: 'herbalism-kit',
      ingredients: [{ item: 'healing-herb', count: 2 }, { item: 'vial', count: 1 }, { item: 'x', count: 3 }],
    })}`)
    expect((add.get('[data-testid="recipe-name"]').element as HTMLInputElement).value).toBe('')
    await wrapper.get(`[data-testid="recipe-${SPOON}"] [data-testid="recipe-remove"]`).trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE recipes/${SPOON}`)
    await expectAccessible(wrapper.element as Element)
  })

  it('says when there is nothing yet, and when the Campaign is not there', async () => {
    const { routes } = backend(false)
    const { wrapper } = await mountApp(`/campaigns/${ID}/downtime`, { ...routes, [`/api/v1/campaigns/${ID}/downtime`]: () => ({ dm: false, gameDay: 0, gameMinute: 5, characters: [], recipes: [], log: [] }) })
    await flushPromises()
    expect(wrapper.get('[data-testid="downtime-clock"]').text()).toBe('The Game Clock stands at day 0, 00:05.')
    expect(wrapper.get('[data-testid="no-recipes"]').text()).toBe('No Recipes in this Campaign yet.')
    expect(wrapper.find('[data-testid="downtime-log"]').exists()).toBe(false)
    const missing = await mountApp(`/campaigns/${ID}/downtime`, { [`/api/v1/campaigns/${ID}/downtime`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404) })
    await flushPromises()
    expect(missing.wrapper.get('[data-testid="downtime-missing"]').text()).toBe('That Campaign is not available.')
  })
})
