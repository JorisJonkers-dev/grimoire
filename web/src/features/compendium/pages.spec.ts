import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const summary = (slug: string, name: string, extra = {}) => ({
  slug, name, level: 3, school: 'evocation', ruleset: 'srd-2024', ritual: false, concentration: false, ...extra,
})

const fireball = {
  slug: 'fireball', name: 'Fireball', level: 3, school: 'evocation', ruleset: 'srd-2024', ritual: false, concentration: false,
  castingTime: 'action', rangeText: '150 feet', rangeFeet: 150, verbal: true, somatic: true, material: true, materialText: 'bat guano',
  duration: 'instantaneous', description: 'A bright streak.\nCreatures are knocked Prone.', higherLevel: '+1d6 per slot.',
  classes: ['sorcerer', 'wizard'], damageTypes: ['fire'], saveAbility: 'dexterity', attackRoll: false, damageRoll: '8d6',
  scaling: [{ kind: 'slot', level: 4, damageRoll: '9d6' }],
  mentions: [{ slug: 'prone', name: 'Prone', description: 'You are on the ground.\nStand up with half your speed.' }],
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.useRealTimers()
})

describe('spell list', () => {
  it('lists spells, filters through the API and loads more', async () => {
    const { wrapper, calls } = await mountApp('/compendium/spells', {
      '/api/v1/compendium/spells': (url) =>
        url.searchParams.get('cursor')
          ? { items: [summary('light', 'Light', { level: 0, ruleset: 'srd-2014' })] }
          : { items: [summary('fireball', 'Fireball', { concentration: true, ritual: true })], nextCursor: 'abc' },
    })
    expect(wrapper.get('[data-testid="spell-list"]').text()).toContain('Fireball')
    expect(wrapper.text()).toContain('3rd level · Evocation')
    await wrapper.get('button.g-button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="spell-list"]').text()).toContain('Light')
    expect(wrapper.text()).toContain('2014')
    await wrapper.get('[data-testid="spell-level"]').setValue('0')
    await wrapper.get('[data-testid="spell-class"]').setValue('wizard')
    await wrapper.findAll('select')[1]?.setValue('evocation')
    await wrapper.findAll('select')[3]?.setValue('srd-2014')
    await flushPromises()
    const last = calls.at(-1)
    expect(last?.searchParams.get('level')).toBe('0')
    expect(last?.searchParams.get('class')).toBe('wizard')
    expect(last?.searchParams.get('school')).toBe('evocation')
    expect(last?.searchParams.get('ruleset')).toBe('srd-2014')
    await expectAccessible(wrapper.element as Element)
  })

  it('debounces the name search', async () => {
    vi.useFakeTimers()
    const { wrapper, calls } = await mountApp('/compendium/spells', {
      '/api/v1/compendium/spells': () => ({ items: [] }),
    })
    expect(wrapper.find('[data-testid="spell-empty"]').exists()).toBe(true)
    await wrapper.get('[data-testid="spell-search"]').setValue('fire')
    await wrapper.get('[data-testid="spell-search"]').setValue('fireb')
    vi.advanceTimersByTime(300)
    await flushPromises()
    expect(calls.filter((c) => c.searchParams.get('q') === 'fireb')).toHaveLength(1)
    expect(calls.some((c) => c.searchParams.get('q') === 'fire')).toBe(false)
  })

  it('shows an error when the list fails', async () => {
    const { wrapper } = await mountApp('/compendium/spells', {
      '/api/v1/compendium/spells': () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
    })
    expect(wrapper.get('[role="alert"]').text()).toContain('could not be loaded')
  })
})

describe('spell detail', () => {
  it('shows the rules and opens condition tooltips', async () => {
    const { wrapper, calls } = await mountApp('/compendium/spells/fireball?ruleset=srd-2024', {
      '/api/v1/compendium/spells/fireball': () => fireball,
    })
    const detail = wrapper.get('[data-testid="spell-detail"]')
    expect(detail.text()).toContain('V, S, M (bat guano)')
    expect(detail.text()).toContain('Dexterity')
    expect(detail.text()).toContain('8d6 Fire')
    expect(detail.text()).toContain('At higher levels')
    expect(calls[0]?.searchParams.get('ruleset')).toBe('srd-2024')
    const term = wrapper.get('button.term')
    expect(term.text()).toBe('Prone')
    await term.trigger('click')
    expect(wrapper.get('[data-testid="condition-popover"]').text()).toContain('Stand up with half your speed.')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('button.close').trigger('click')
    expect(wrapper.find('[data-testid="condition-popover"]').exists()).toBe(false)
  })

  it('handles a minimal spell and a missing spell', async () => {
    const minimal = { ...fireball, material: false, materialText: undefined, saveAbility: undefined, damageRoll: undefined,
      higherLevel: undefined, ritual: true, concentration: true, ruleset: 'srd-2014', mentions: [] }
    const { wrapper } = await mountApp('/compendium/spells/fireball', { '/api/v1/compendium/spells/fireball': () => minimal })
    expect(wrapper.text()).toContain('V, S')
    expect(wrapper.text()).toContain('ritual')
    expect(wrapper.text()).toContain('Concentration, instantaneous')
    expect(wrapper.find('button.term').exists()).toBe(false)
    const missing = await mountApp('/compendium/spells/nothing', {})
    expect(missing.wrapper.find('[data-testid="spell-missing"]').exists()).toBe(true)
  })
})

describe('attribution', () => {
  it('credits the author and every source', async () => {
    const { wrapper } = await mountApp('/about/attribution', {
      '/api/v1/compendium/sources': () => [
        { key: 'srd-2024', title: 'SRD 5.2', rulesetYear: 2024, license: 'CC-BY-4.0', attribution: 'By WotC.', url: 'https://x' },
      ],
    })
    expect(wrapper.text()).toContain('Joris Jonkers')
    expect(wrapper.get('[data-testid="source"]').text()).toContain('By WotC.')
    const failing = await mountApp('/about/attribution', {})
    expect(failing.wrapper.get('[role="alert"]').text()).toContain('could not be loaded')
  })
})
