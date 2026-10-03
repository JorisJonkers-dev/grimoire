import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { RuleHooks, RuleVariant } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const onOff = [{ value: 'off', label: 'Off' }, { value: 'on', label: 'On' }]
const variants: RuleVariant[] = [
  { slug: 'flanking', name: 'Flanking', description: 'A melee attack has Advantage when an ally stands across the target.', automated: true, value: 'on', options: onOff },
  {
    slug: 'rests', name: 'Rest lengths', description: 'How long a rest takes on the Game Clock.', automated: true, value: 'standard',
    options: [{ value: 'standard', label: 'Standard: 1 hour and 8 hours' }, { value: 'gritty', label: 'Gritty: 8 hours and 7 days' }],
  },
  { slug: 'morale', name: 'Morale', description: 'A creature down to half its hit points makes a save or flees.', automated: false, value: 'off', options: onOff },
  {
    slug: 'critical-hits', name: 'Critical hits', description: 'What a Critical Hit does to the damage dice.', automated: true, value: 'max-dice',
    options: [{ value: 'double-dice', label: 'Roll every damage die twice' }, { value: 'max-dice', label: 'Roll once and add the most' }],
  },
]
const FUMBLES = '0190c7a8-0000-7000-8000-0000000000f7'
const HOOK = '0190c7a8-0000-7000-8000-0000000000b1'
const hooks: RuleHooks = {
  dm: true,
  hooks: [
    { id: HOOK, name: 'Critical fumbles', hook: 'natural-1', rollTableId: FUMBLES, tableName: 'Fumbles' },
    { id: '0190c7a8-0000-7000-8000-0000000000b2', name: 'Winded', hook: 'drop-to-0', effect: 'exhaustion' },
    { id: '0190c7a8-0000-7000-8000-0000000000b3', name: 'Old injuries', hook: 'rest', rollTableId: '0190c7a8-0000-7000-8000-0000000000f9', tableName: '' },
  ],
  points: [
    { slug: 'natural-1', label: 'On a natural 1 on an attack roll' },
    { slug: 'critical', label: 'On a natural 20 on an attack roll' },
    { slug: 'drop-to-0', label: 'On dropping to 0 hit points' },
    { slug: 'rest', label: 'On finishing a rest' },
    { slug: 'cast', label: 'On casting a spell' },
  ],
  tables: [{ id: FUMBLES, name: 'Fumbles' }],
}
const member = (role: 'dm' | 'player') => ({ id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role, joinedAt: '2026-09-30T20:00:00Z', isMe: true })
const campaign = (as: 'dm' | 'player') => ({ id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 1, createdAt: '2026-09-30T20:00:00Z', me: member(as), members: [member(as)] })

afterEach(() => { unmountAll() })

function backend(as: 'dm' | 'player') {
  const calls: string[] = []
  const state = { fail: 0 }
  const routes = {
    [`/api/v1/campaigns/${ID}/rule-variants`]: async (_u: URL, req: Request) => {
      if (req.method === 'GET') return variants
      calls.push(`PUT ${await req.text()}`)
      if (state.fail) return jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
      return new Response(null, { status: 204 })
    },
    [`/api/v1/campaigns/${ID}/rule-hooks/`]: (url: URL, req: Request) => {
      calls.push(`${req.method} ${url.pathname.split('/').slice(5).join('/')}`)
      if (state.fail) return jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
      return new Response(null, { status: 204 })
    },
    [`/api/v1/campaigns/${ID}/rule-hooks`]: async (_u: URL, req: Request) => {
      if (req.method === 'GET') return as === 'dm' ? hooks : { ...hooks, dm: false, tables: [] }
      calls.push(`POST ${await req.text()}`)
      if (state.fail) return jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
      return jsonResponse(hooks.hooks[0], 201)
    },
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  }
  return { calls, routes, state }
}

describe('rule variants', () => {
  it('shows a Player how the table plays, with nothing to switch', async () => {
    const { routes } = backend('player')
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, routes)
    await flushPromises()
    const flanking = wrapper.get('[data-testid="variant-flanking"]')
    expect(flanking.get('h2').text()).toBe('Flanking')
    expect(flanking.get('[data-testid="variant-description"]').text()).toBe('A melee attack has Advantage when an ally stands across the target.')
    expect(flanking.get('[data-testid="variant-value"]').text()).toBe('On')
    expect(flanking.get('[data-testid="variant-applied"]').text()).toBe('Grimoire applies this in play.')
    expect(wrapper.get('[data-testid="variant-rests"] [data-testid="variant-value"]').text()).toBe('Standard: 1 hour and 8 hours')
    expect(wrapper.get('[data-testid="variant-morale"] [data-testid="variant-applied"]').text()).toBe('The DM applies this by hand.')
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid^="variant-"][data-on]').map((v) => v.attributes('data-on'))).toEqual(['true', 'false', 'false', 'true'])
    await expectAccessible(wrapper.element as Element)
  })

  it('lets the DM switch each variant, one at a time', async () => {
    const { calls, routes, state } = backend('dm')
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, routes)
    await flushPromises()
    const pick = (slug: string) => wrapper.get<HTMLSelectElement>(`[data-testid="variant-${slug}"] select`)
    expect(pick('flanking').element.value).toBe('on')
    expect(pick('rests').findAll('option').map((o) => o.text())).toEqual(['Standard: 1 hour and 8 hours', 'Gritty: 8 hours and 7 days'])
    expect(wrapper.find('[data-testid="variant-value"]').exists()).toBe(false)
    await pick('rests').setValue('gritty')
    await flushPromises()
    expect(calls).toEqual(['PUT {"choices":[{"slug":"rests","value":"gritty"}]}'])
    expect(wrapper.get('[data-testid="variants-saved"]').text()).toBe('Rest lengths is switched.')
    await pick('flanking').setValue('off')
    await flushPromises()
    expect(calls.at(-1)).toBe('PUT {"choices":[{"slug":"flanking","value":"off"}]}')
    expect(wrapper.get('[data-testid="variants-saved"]').text()).toBe('Flanking is switched.')
    await expectAccessible(wrapper.element as Element)
    // A switch that fails says so, and takes the last one's news away; the next that works clears it.
    state.fail = 422
    await pick('morale').setValue('on')
    await flushPromises()
    expect(wrapper.get('[data-testid="variants-problem"]').text()).toBe('That could not be switched. Try again.')
    expect(wrapper.find('[data-testid="variants-saved"]').exists()).toBe(false)
    state.fail = 0
    await pick('morale').setValue('on')
    await flushPromises()
    expect(wrapper.find('[data-testid="variants-problem"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="variants-saved"]').text()).toBe('Morale is switched.')
  })

  it('shows a Player the Campaign\'s own Rule Variants, with nothing to author', async () => {
    const { routes } = backend('player')
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, routes)
    await flushPromises()
    expect(wrapper.findAll('[data-testid^="hook-0190"]').map((h) => h.text())).toEqual([
      'Critical fumbles: On a natural 1 on an attack roll, roll on Fumbles.',
      'Winded: On dropping to 0 hit points, exhaustion applies.',
      'Old injuries: On finishing a rest, roll on a Roll Table this Campaign no longer sees.',
    ])
    expect(wrapper.find('[data-testid="hook-add"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="hook-remove"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('lets the DM author a Rule Variant from a hook point, to a Roll Table or an Effect, and remove one', async () => {
    const { calls, routes, state } = backend('dm')
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, routes)
    await flushPromises()
    const add = wrapper.get('[data-testid="hook-add"]')
    const set = (id: string, value: string) => add.get(`[data-testid="${id}"]`).setValue(value)
    expect(add.get('button').attributes('disabled')).toBeDefined()
    expect(add.findAll('[data-testid="hook-point"] option').map((o) => o.text())).toHaveLength(5)
    // It needs a name, and with no Roll Table chosen an Effect.
    await set('hook-effect', 'prone')
    expect(add.get('button').attributes('disabled')).toBeDefined()
    await set('hook-effect', '')
    await set('hook-name', '  Critical fumbles ')
    expect(add.get('button').attributes('disabled')).toBeDefined()
    await set('hook-effect', ' prone ')
    expect(add.get('button').attributes('disabled')).toBeUndefined()
    await set('hook-point', 'critical')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST {"name":"Critical fumbles","hook":"critical","effect":"prone"}')
    expect((add.get('[data-testid="hook-name"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-testid="variants-saved"]').text()).toBe('Critical fumbles is added.')
    // A Roll Table takes the place of the Effect.
    await set('hook-name', 'Fumbles')
    await set('hook-effect', 'prone')
    await set('hook-table', FUMBLES)
    expect(add.find('[data-testid="hook-effect"]').exists()).toBe(false)
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST {"name":"Fumbles","hook":"natural-1","rollTableId":"${FUMBLES}"}`)
    await wrapper.get(`[data-testid="hook-${HOOK}"] [data-testid="hook-remove"]`).trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE rule-hooks/${HOOK}`)
    expect(wrapper.get('[data-testid="variants-saved"]').text()).toBe('Critical fumbles is removed.')
    await expectAccessible(wrapper.element as Element)
    // A failure says so and keeps what was typed.
    state.fail = 422
    await set('hook-name', 'Kept')
    await set('hook-effect', 'prone')
    await add.trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="variants-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    expect((add.get('[data-testid="hook-name"]').element as HTMLInputElement).value).toBe('Kept')
    await wrapper.get(`[data-testid="hook-${HOOK}"] [data-testid="hook-remove"]`).trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="variants-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    expect(wrapper.find('[data-testid="variants-saved"]').exists()).toBe(false)
  })

  it('says when the Campaign has none of its own, and when it sees no Roll Table', async () => {
    const { routes } = backend('dm')
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, { ...routes, [`/api/v1/campaigns/${ID}/rule-hooks`]: () => ({ ...hooks, hooks: [], tables: [] }) })
    await flushPromises()
    expect(wrapper.get('[data-testid="no-hooks"]').text()).toBe('This Campaign has no Rule Variants of its own yet.')
    expect(wrapper.find('[data-testid="hook-table"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="no-tables"]').text()).toContain('Link a Roll Table from your Library')
  })

  it('says when the Campaign is not there', async () => {
    const missing = () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404)
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, { [`/api/v1/campaigns/${ID}/rule-variants`]: missing, [`/api/v1/campaigns/${ID}`]: missing })
    await flushPromises()
    expect(wrapper.get('[data-testid="variants-missing"]').text()).toBe('That Campaign is not available.')
    expect(wrapper.find('[data-testid^="variant-"]').exists()).toBe(false)
  })
})
