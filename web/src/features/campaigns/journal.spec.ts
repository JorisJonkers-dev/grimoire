import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Journal } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SEAL = '0190c7a8-0000-7000-8000-0000000000d1'
const TRAITOR = '0190c7a8-0000-7000-8000-0000000000d2'
const OAKFORD = '0190c7a8-0000-7000-8000-0000000000e1'
const LETTER = '0190c7a8-0000-7000-8000-0000000000e2'
const at = '2026-10-03T10:00:00Z'
const seal = { id: SEAL, name: 'The stolen seal', summary: 'The Watch has lost its seal.', status: 'active' as const, updatedAt: at, steps: [{ text: 'Ask at the Lantern Inn', done: true }, { text: 'Find the fence', done: false }, { text: 'Return the seal', done: false }] }
const oakford = { id: OAKFORD, title: 'Oakford', body: 'A market town on the river.', unlocked: true, unlockedAt: at, updatedAt: at }
const seenByPlayer: Journal = { dm: false, quests: [seal, { ...seal, id: TRAITOR, name: 'The old debt', summary: '', status: 'failed', steps: [] }], lore: [oakford], readable: ['sealed-letter', 'black-book'] }
const seenByDM: Journal = {
  dm: true,
  quests: [seal, { id: TRAITOR, name: 'The traitor in the Watch', summary: '', status: 'hidden', updatedAt: at, steps: [] }],
  lore: [{ ...oakford, itemSlug: '' }, { id: LETTER, title: "The Captain's letter", body: 'Vane owes the Grey Hands.', unlocked: false, itemSlug: 'sealed-letter', updatedAt: at }],
  readable: [],
}

afterEach(() => { unmountAll() })

function backend(journal: Journal, fail = 0) {
  const calls: string[] = []
  const state = { fail }
  const write = (answer: (body: string) => Response) => async (url: URL, req: Request) => {
    const body = req.method === 'DELETE' ? '' : await req.text()
    calls.push(`${req.method} ${url.pathname.split('/').slice(5).join('/')} ${body}`.trim())
    if (state.fail) return jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
    return req.method === 'POST' ? answer(body) : new Response(null, { status: 204 })
  }
  const routes = {
    [`/api/v1/campaigns/${ID}/journal/readings`]: write((body) => jsonResponse({ unlocked: body.includes('sealed-letter') ? 2 : 1 })),
    [`/api/v1/campaigns/${ID}/journal`]: () => journal,
    [`/api/v1/campaigns/${ID}/quests`]: write(() => jsonResponse(seal, 201)),
    [`/api/v1/campaigns/${ID}/lore`]: write(() => jsonResponse(oakford, 201)),
  }
  return { calls, routes, state }
}

describe('journal', () => {
  it('shows a Player the Quests the party has and the Lore it knows, and lets them read what they carry', async () => {
    const { calls, routes, state } = backend(seenByPlayer)
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, routes)
    await flushPromises()
    const quest = wrapper.get(`[data-testid="quest-${SEAL}"]`)
    expect(quest.get('h3').text()).toBe('The stolen seal')
    expect(quest.get('[data-testid="quest-status"]').text()).toBe('Active')
    expect(quest.get('[data-testid="quest-summary"]').text()).toBe('The Watch has lost its seal.')
    expect(quest.findAll('[data-testid="step"]').map((s) => s.text())).toEqual(['Done: Ask at the Lantern Inn', 'To do: Find the fence', 'To do: Return the seal'])
    const other = wrapper.get(`[data-testid="quest-${TRAITOR}"]`)
    expect(other.get('[data-testid="quest-status"]').text()).toBe('Failed')
    expect(other.find('[data-testid="quest-summary"]').exists()).toBe(false)
    expect(other.find('[data-testid="step"]').exists()).toBe(false)
    const lore = wrapper.get(`[data-testid="lore-${OAKFORD}"]`)
    expect(lore.get('h3').text()).toBe('Oakford')
    expect(lore.get('[data-testid="lore-body"]').text()).toBe('A market town on the river.')
    for (const id of ['quest-add', 'lore-add', 'quest-remove', 'lore-remove', 'lore-toggle', 'lore-state', 'quest-set-status', 'step-add']) expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    expect(quest.find('input').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)

    // What the Player carries that still holds Lore can be read.
    expect(wrapper.findAll('[data-testid="readable"] button').map((b) => b.text())).toEqual(['Read the Sealed letter', 'Read the Black book'])
    await wrapper.get('[data-testid="read-sealed-letter"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST journal/readings {"itemSlug":"sealed-letter"}')
    expect(wrapper.get('[data-testid="journal-read"]').text()).toBe('You learned 2 new Lore entries.')
    await wrapper.get('[data-testid="read-black-book"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="journal-read"]').text()).toBe('You learned 1 new Lore entry.')
    expect(wrapper.find('[data-testid="journal-problem"]').exists()).toBe(false)
    // A reading that fails takes the last one's news away, and the next that works takes the failure away.
    state.fail = 422
    await wrapper.get('[data-testid="read-black-book"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="journal-problem"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="journal-read"]').exists()).toBe(false)
    state.fail = 0
    await wrapper.get('[data-testid="read-black-book"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="journal-problem"]').exists()).toBe(false)
  })

  it('says so when the Journal is empty, and when a reading comes to nothing', async () => {
    const { routes } = backend({ dm: false, quests: [], lore: [], readable: ['rope'] }, 422)
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, routes)
    await flushPromises()
    expect(wrapper.get('[data-testid="no-quests"]').text()).toBe('No Quests yet.')
    expect(wrapper.get('[data-testid="no-lore"]').text()).toBe('No Lore yet.')
    await wrapper.get('[data-testid="read-rope"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="journal-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    expect(wrapper.find('[data-testid="journal-read"]').exists()).toBe(false)
  })

  it('says when the Campaign is not there', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, { [`/api/v1/campaigns/${ID}/journal`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404) })
    await flushPromises()
    expect(wrapper.get('[data-testid="journal-missing"]').text()).toBe('That Campaign is not available.')
    expect(wrapper.find('[data-testid="no-quests"]').exists()).toBe(false)
  })

  it('lets the DM keep Quests: steps, status, new ones, and removing them', async () => {
    const { calls, routes } = backend(seenByDM)
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, routes)
    await flushPromises()
    const quest = wrapper.get(`[data-testid="quest-${SEAL}"]`)
    const hidden = wrapper.get(`[data-testid="quest-${TRAITOR}"]`)
    expect(hidden.get('[data-testid="quest-status"]').text()).toBe('Hidden from the Players')
    expect(wrapper.find('[data-testid="readable"]').exists()).toBe(false)
    const ticks = quest.findAll<HTMLInputElement>('[data-testid="step"] input')
    expect(ticks.map((t) => t.element.checked)).toEqual([true, false, false])
    // Ticking a step saves the whole Quest with that step done.
    await ticks[1]?.setValue(true)
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT quests/${SEAL} {"name":"The stolen seal","summary":"The Watch has lost its seal.","status":"active","steps":[{"text":"Ask at the Lantern Inn","done":true},{"text":"Find the fence","done":true},{"text":"Return the seal","done":false}]}`)
    await ticks[0]?.setValue(false)
    await flushPromises()
    expect(calls.at(-1)).toContain('"steps":[{"text":"Ask at the Lantern Inn","done":false},{"text":"Find the fence","done":false},{"text":"Return the seal","done":false}]')
    await hidden.get('[data-testid="quest-set-status"]').setValue('active')
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT quests/${TRAITOR} {"name":"The traitor in the Watch","summary":"","status":"active","steps":[]}`)
    // A new step joins the end; an empty one is not sent.
    const stepAdd = hidden.get('[data-testid="step-add"]')
    expect(stepAdd.get('button').attributes('disabled')).toBeDefined()
    await stepAdd.get('input').setValue('  Confront Vane ')
    await stepAdd.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT quests/${TRAITOR} {"name":"The traitor in the Watch","summary":"","status":"hidden","steps":[{"text":"Confront Vane","done":false}]}`)
    expect((stepAdd.get('input').element as HTMLInputElement).value).toBe('')
    await quest.get('[data-testid="quest-remove"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE quests/${SEAL}`)

    // A new Quest starts hidden, with a step for each line written.
    const add = wrapper.get('[data-testid="quest-add"]')
    expect(add.get('button').attributes('disabled')).toBeDefined()
    await add.get('[data-testid="quest-name"]').setValue(' The missing heir ')
    await add.get('[data-testid="quest-summary-new"]').setValue('Lady Mora wants her son found.')
    await add.get('[data-testid="quest-steps"]').setValue('Search the docks\n\n  Ask the harbourmaster  \n')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST quests {"name":"The missing heir","summary":"Lady Mora wants her son found.","status":"hidden","steps":[{"text":"Search the docks","done":false},{"text":"Ask the harbourmaster","done":false}]}')
    expect((add.get('[data-testid="quest-name"]').element as HTMLInputElement).value).toBe('')
    await add.get('[data-testid="quest-name"]').setValue('Given at once')
    await add.get('[data-testid="quest-status-new"]').setValue('active')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST quests {"name":"Given at once","summary":"","status":"active","steps":[]}')
    await expectAccessible(wrapper.element as Element)
  })

  it('lets the DM keep Lore: what holds it, locking and unlocking, new entries, and removing them', async () => {
    const { calls, routes } = backend(seenByDM)
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, routes)
    await flushPromises()
    const known = wrapper.get(`[data-testid="lore-${OAKFORD}"]`)
    const locked = wrapper.get(`[data-testid="lore-${LETTER}"]`)
    expect(known.get('[data-testid="lore-state"]').text()).toBe('The party knows this.')
    expect(locked.get('[data-testid="lore-state"]').text()).toBe('Locked. Reading the Sealed letter unlocks it.')
    await locked.get('[data-testid="lore-toggle"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT lore/${LETTER} {"title":"The Captain's letter","body":"Vane owes the Grey Hands.","itemSlug":"sealed-letter","unlocked":true}`)
    expect(known.get('[data-testid="lore-toggle"]').text()).toBe('Lock')
    await known.get('[data-testid="lore-toggle"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`PUT lore/${OAKFORD} {"title":"Oakford","body":"A market town on the river.","itemSlug":"","unlocked":false}`)
    await locked.get('[data-testid="lore-remove"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE lore/${LETTER}`)

    const add = wrapper.get('[data-testid="lore-add"]')
    expect(add.get('button').attributes('disabled')).toBeDefined()
    await add.get('[data-testid="lore-title"]').setValue(' The Ashen Hand ')
    await add.get('[data-testid="lore-body-new"]').setValue('A cult older than the town.')
    await add.get('[data-testid="lore-item"]').setValue(' black-book ')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST lore {"title":"The Ashen Hand","body":"A cult older than the town.","itemSlug":"black-book","unlocked":false}')
    expect((add.get('[data-testid="lore-title"]').element as HTMLInputElement).value).toBe('')
    await add.get('[data-testid="lore-title"]').setValue('The river')
    await add.get('[data-testid="lore-unlocked"]').setValue(true)
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST lore {"title":"The river","body":"","itemSlug":"","unlocked":true}')
    // A locked entry that no item holds says only that it is locked.
    const bare = await mountApp(`/campaigns/${ID}/journal`, backend({ ...seenByDM, lore: [{ ...oakford, unlocked: false, unlockedAt: undefined, itemSlug: '' }] }).routes)
    await flushPromises()
    expect(bare.wrapper.get('[data-testid="lore-state"]').text()).toBe('Locked.')
    expect(bare.wrapper.get('[data-testid="lore-toggle"]').text()).toBe('Unlock')
  })

  it('says when the DM\'s change could not be made', async () => {
    const { routes } = backend(seenByDM, 422)
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, routes)
    await flushPromises()
    for (const act of [
      () => wrapper.get('[data-testid="quest-remove"]').trigger('click'),
      () => wrapper.get('[data-testid="quest-set-status"]').setValue('failed'),
      () => wrapper.get('[data-testid="lore-toggle"]').trigger('click'),
      () => wrapper.get('[data-testid="lore-remove"]').trigger('click'),
      async () => { await wrapper.get('[data-testid="quest-name"]').setValue('X'); await wrapper.get('[data-testid="quest-add"]').trigger('submit') },
      async () => { await wrapper.get('[data-testid="lore-title"]').setValue('X'); await wrapper.get('[data-testid="lore-add"]').trigger('submit') },
    ]) {
      await act()
      await flushPromises()
      expect(wrapper.get('[data-testid="journal-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    }
    expect((wrapper.get('[data-testid="quest-name"]').element as HTMLInputElement).value).toBe('X')
  })
})
