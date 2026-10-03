import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { RuleVariant } from '@/infrastructure/api/types.gen'
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

  it('says when the Campaign is not there', async () => {
    const missing = () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404)
    const { wrapper } = await mountApp(`/campaigns/${ID}/rules`, { [`/api/v1/campaigns/${ID}/rule-variants`]: missing, [`/api/v1/campaigns/${ID}`]: missing })
    await flushPromises()
    expect(wrapper.get('[data-testid="variants-missing"]').text()).toBe('That Campaign is not available.')
    expect(wrapper.find('[data-testid^="variant-"]').exists()).toBe(false)
  })
})
