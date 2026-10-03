import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Tracks } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const STRESS = '0190c7a8-0000-7000-8000-0000000000a1'
const RENOWN = '0190c7a8-0000-7000-8000-0000000000a2'
const TAMSIN = '0190c7a8-0000-7000-8000-0000000000c1'
const MADNESS = '0190c7a8-0000-7000-8000-0000000000f7'
const seenByDM: Tracks = {
  dm: true,
  tracks: [
    {
      id: STRESS, name: 'Stress', scope: 'character', min: 0, max: 10, start: 1,
      thresholds: [
        { at: 4, rising: true, label: 'Shaken', effect: 'frightened' },
        { at: 8, rising: true, label: 'Breaking point', rollTableId: MADNESS },
        { at: 2, rising: false, label: 'Calm again' },
        { at: 9, rising: true, label: 'Lost', rollTableId: '0190c7a8-0000-7000-8000-0000000000f8' },
      ],
      standings: [{ characterId: TAMSIN, name: 'Tamsin', value: 3 }],
    },
    { id: RENOWN, name: 'Renown', scope: 'party', min: -5, max: 5, start: 0, thresholds: [], standings: [{ name: '', value: -2 }] },
  ],
}
const seenByPlayer: Tracks = { dm: false, tracks: seenByDM.tracks.map((t) => ({ ...t, thresholds: undefined })) }
const hooks = { dm: true, hooks: [], points: [], tables: [{ id: MADNESS, name: 'Madness' }] }

afterEach(() => { unmountAll() })

function backend(tracks: Tracks) {
  const calls: string[] = []
  const state = { fail: 0, crossed: [{ at: 4, rising: true, label: 'Shaken', effect: 'frightened' }] as object[] }
  const failing = () => jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
  const routes = {
    [`/api/v1/campaigns/${ID}/tracks/`]: async (url: URL, req: Request) => {
      calls.push(`${req.method} ${url.pathname.split('/').slice(5).join('/')} ${req.method === 'DELETE' ? '' : await req.text()}`.trim())
      if (state.fail) return failing()
      return req.method === 'DELETE' ? new Response(null, { status: 204 }) : jsonResponse({ value: 4, crossed: state.crossed })
    },
    [`/api/v1/campaigns/${ID}/tracks`]: async (_u: URL, req: Request) => {
      if (req.method === 'GET') return tracks
      calls.push(`POST tracks ${await req.text()}`)
      if (state.fail) return failing()
      return jsonResponse(seenByDM.tracks[0], 201)
    },
    [`/api/v1/campaigns/${ID}/rule-hooks`]: () => hooks,
  }
  return { calls, routes, state }
}

describe('tracks', () => {
  it('shows a Player the Tracks with their own score and the party\'s, and nothing to move them with', async () => {
    const { routes } = backend(seenByPlayer)
    const { wrapper } = await mountApp(`/campaigns/${ID}/tracks`, routes)
    await flushPromises()
    const stress = wrapper.get(`[data-testid="track-${STRESS}"]`)
    expect(stress.get('h2').text()).toBe('Stress')
    expect(stress.get('[data-testid="track-kept"]').text()).toBe('Kept for each Character, from 0 to 10.')
    expect(stress.findAll('[data-testid="standing"]').map((s) => s.text())).toEqual(['Tamsin: 3'])
    const renown = wrapper.get(`[data-testid="track-${RENOWN}"]`)
    expect(renown.get('[data-testid="track-kept"]').text()).toBe('Kept for the party, from -5 to 5.')
    expect(renown.findAll('[data-testid="standing"]').map((s) => s.text())).toEqual(['The party: -2'])
    for (const id of ['track-add', 'track-remove', 'thresholds', 'raise', 'lower']) expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('says so when there are none, and when the Campaign is not there', async () => {
    const { routes } = backend({ dm: false, tracks: [] })
    const { wrapper } = await mountApp(`/campaigns/${ID}/tracks`, routes)
    await flushPromises()
    expect(wrapper.get('[data-testid="no-tracks"]').text()).toBe('No Tracks in this Campaign yet.')
    const missing = await mountApp(`/campaigns/${ID}/tracks`, { [`/api/v1/campaigns/${ID}/tracks`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404) })
    await flushPromises()
    expect(missing.wrapper.get('[data-testid="tracks-missing"]').text()).toBe('That Campaign is not available.')
  })

  it('lets the DM read the thresholds, move a score and hear what it crossed, and remove a Track', async () => {
    const { calls, routes, state } = backend(seenByDM)
    const { wrapper } = await mountApp(`/campaigns/${ID}/tracks`, routes)
    await flushPromises()
    const stress = wrapper.get(`[data-testid="track-${STRESS}"]`)
    expect(stress.findAll('[data-testid="thresholds"] li').map((li) => li.text())).toEqual([
      'At 4 on the way up: Shaken. Applies frightened.',
      'At 8 on the way up: Breaking point. Rolls on Madness.',
      'At 2 on the way down: Calm again.',
      'At 9 on the way up: Lost. Rolls on a Roll Table this Campaign no longer sees.',
    ])
    await stress.get('[data-testid="raise"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST tracks/${STRESS}/adjustments {"delta":1,"characterId":"${TAMSIN}"}`)
    expect(wrapper.get('[data-testid="tracks-moved"]').text()).toBe('Stress for Tamsin is now 4. Crossed: Shaken.')
    state.crossed = []
    await stress.get('[data-testid="lower"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST tracks/${STRESS}/adjustments {"delta":-1,"characterId":"${TAMSIN}"}`)
    expect(wrapper.get('[data-testid="tracks-moved"]').text()).toBe('Stress for Tamsin is now 4.')
    // A larger move in one go, for the party.
    const renown = wrapper.get(`[data-testid="track-${RENOWN}"]`)
    await renown.get('[data-testid="by"]').setValue(3)
    await renown.get('[data-testid="raise"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST tracks/${RENOWN}/adjustments {"delta":3}`)
    expect(wrapper.get('[data-testid="tracks-moved"]').text()).toBe('Renown for the party is now 4.')
    state.crossed = [{ at: 2, rising: false, label: 'Calm again' }, { at: 1, rising: false, label: 'Serene' }]
    await renown.get('[data-testid="lower"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST tracks/${RENOWN}/adjustments {"delta":-3}`)
    expect(wrapper.get('[data-testid="tracks-moved"]').text()).toBe('Renown for the party is now 4. Crossed: Calm again, Serene.')
    await expectAccessible(wrapper.element as Element)
    // A failure says so, and takes the last move's news away.
    state.fail = 422
    await stress.get('[data-testid="raise"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="tracks-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    expect(wrapper.find('[data-testid="tracks-moved"]').exists()).toBe(false)
    state.fail = 0
    await renown.get('[data-testid="track-remove"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE tracks/${RENOWN}`)
    expect(wrapper.find('[data-testid="tracks-problem"]').exists()).toBe(false)
    state.fail = 422
    await stress.get('[data-testid="track-remove"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="tracks-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
  })

  it('lets the DM add a Track with its bounds and thresholds', async () => {
    const { calls, routes, state } = backend(seenByDM)
    const { wrapper } = await mountApp(`/campaigns/${ID}/tracks`, routes)
    await flushPromises()
    const add = wrapper.get('[data-testid="track-add"]')
    const set = (id: string, value: string | number) => add.get(`[data-testid="${id}"]`).setValue(value)
    expect(add.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    await set('track-name', '  Sanity ')
    await set('track-scope', 'party')
    await set('track-min', 0)
    await set('track-max', 20)
    await set('track-start', 20)
    await add.get('[data-testid="threshold-add"]').trigger('click')
    await set('threshold-0-at', 10)
    await set('threshold-0-way', 'down')
    await set('threshold-0-label', ' Rattled ')
    await set('threshold-0-effect', ' frightened ')
    await add.get('[data-testid="threshold-add"]').trigger('click')
    await set('threshold-1-at', 5)
    await set('threshold-1-way', 'down')
    await set('threshold-1-label', 'Unravelling')
    await set('threshold-1-effect', 'prone')
    await set('threshold-1-table', MADNESS)
    // A Roll Table takes the place of the Effect.
    expect(add.find('[data-testid="threshold-1-effect"]').exists()).toBe(false)
    await add.get('[data-testid="threshold-add"]').trigger('click')
    await set('threshold-2-label', 'Whole again')
    await add.get('[data-testid="threshold-add"]').trigger('click')
    await add.get('[data-testid="threshold-3-remove"]').trigger('click')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST tracks ${JSON.stringify({
      name: 'Sanity', scope: 'party', min: 0, max: 20, start: 20,
      thresholds: [
        { at: 10, rising: false, label: 'Rattled', effect: 'frightened' },
        { at: 5, rising: false, label: 'Unravelling', rollTableId: MADNESS },
        { at: 0, rising: true, label: 'Whole again' },
      ],
    })}`)
    expect((add.get('[data-testid="track-name"]').element as HTMLInputElement).value).toBe('')
    expect(add.find('[data-testid="threshold-0-label"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
    state.fail = 422
    await set('track-name', 'Kept')
    await add.trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="tracks-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    expect((add.get('[data-testid="track-name"]').element as HTMLInputElement).value).toBe('Kept')
  })
})
