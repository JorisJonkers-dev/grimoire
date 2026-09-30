import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { fakeClock, mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { isEntryKind, kindLabel } from './kinds'

const summary = (slug: string, name: string, extra = {}) => ({
  kind: 'monster', slug, name, subtitle: 'CR 1/4 · Humanoid', ruleset: 'srd-2024', ...extra,
})

const goblin = {
  kind: 'monster', slug: 'goblin', name: 'Goblin', subtitle: 'CR 0.25 · Humanoid', ruleset: 'srd-2024',
  facts: [{ label: 'Armor Class', value: '15 (leather)' }, { label: 'Hit Points', value: '7 (2d6)' }],
  sections: [{ title: 'Scimitar', text: 'Slash.\nThe target is knocked Prone.' }],
  mentions: [{ slug: 'prone', name: 'Prone', description: 'You are on the ground.' }],
}

const problem = () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503)

afterEach(() => {
  document.body.innerHTML = ''
  vi.useRealTimers()
})

describe('entry kinds', () => {
  it('knows every kind by label', () => {
    expect(kindLabel('magic-item')).toBe('Magic items')
    expect(isEntryKind('monster')).toBe(true)
    expect(isEntryKind('spell')).toBe(false)
  })
})

describe('entry list', () => {
  it('lists a kind, filters by ruleset and loads more', async () => {
    const { wrapper, calls } = await mountApp('/compendium/monster', {
      '/api/v1/compendium/entries': (url) =>
        url.searchParams.get('cursor')
          ? { items: [summary('orc', 'Orc', { ruleset: 'srd-2014' })] }
          : { items: [summary('goblin', 'Goblin')], nextCursor: 'abc' },
    })
    expect(wrapper.get('h1').text()).toBe('Monsters')
    expect(wrapper.get('[data-testid="entry-list"]').text()).toContain('CR 1/4 · Humanoid')
    expect(wrapper.get('[aria-current="page"]').text()).toBe('Monsters')
    await wrapper.get('button.g-button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="entry-list"]').text()).toContain('Orc')
    expect(wrapper.get('[data-testid="entry-list"]').text()).toContain('2014')
    await wrapper.get('[data-testid="entry-ruleset"]').setValue('srd-2014')
    await flushPromises()
    expect(calls.at(-1)?.searchParams.get('ruleset')).toBe('srd-2014')
    expect(calls.at(-1)?.searchParams.get('kind')).toBe('monster')
    expect(wrapper.get('[data-testid="entry-list"] a').attributes('href')).toContain('ruleset=srd-2014')
    await expectAccessible(wrapper.element as Element)
  })

  it('debounces the search and resets it when the kind changes', async () => {
    const { wrapper, calls, router } = await mountApp('/compendium/feat', {
      '/api/v1/compendium/entries': () => ({ items: [] }),
    })
    fakeClock()
    expect(wrapper.find('[data-testid="entry-empty"]').exists()).toBe(true)
    await wrapper.get('[data-testid="entry-search"]').setValue('gr')
    await wrapper.get('[data-testid="entry-search"]').setValue('grap')
    vi.advanceTimersByTime(300)
    await flushPromises()
    expect(calls.filter((c) => c.searchParams.get('q') === 'grap')).toHaveLength(1)
    expect(calls.some((c) => c.searchParams.get('q') === 'gr')).toBe(false)
    await router.push('/compendium/weapon')
    await flushPromises()
    expect((wrapper.get('[data-testid="entry-search"]').element as HTMLInputElement).value).toBe('')
    expect(calls.at(-1)?.searchParams.get('kind')).toBe('weapon')
  })

  it('rejects unknown kinds without calling the API', async () => {
    const { wrapper, calls } = await mountApp('/compendium/dragons', {})
    expect(wrapper.find('[data-testid="entry-kind-missing"]').exists()).toBe(true)
    expect(calls).toHaveLength(0)
  })

  it('shows an error when the list fails', async () => {
    const { wrapper } = await mountApp('/compendium/armor', { '/api/v1/compendium/entries': problem })
    expect(wrapper.get('[role="alert"]').text()).toContain('could not be loaded')
  })

  it('redirects the compendium root to spells', async () => {
    const { router } = await mountApp('/compendium', { '/api/v1/compendium/spells': () => ({ items: [] }) })
    expect(router.currentRoute.value.name).toBe('spells')
  })
})

describe('entry detail', () => {
  it('shows facts and sections and opens condition tooltips', async () => {
    const { wrapper, calls } = await mountApp('/compendium/monster/goblin?ruleset=srd-2024', {
      '/api/v1/compendium/entries/monster/goblin': () => goblin,
    })
    const detail = wrapper.get('[data-testid="entry-detail"]')
    expect(detail.get('[data-testid="entry-facts"]').text()).toContain('15 (leather)')
    expect(detail.text()).toContain('Scimitar')
    expect(calls[0]?.searchParams.get('ruleset')).toBe('srd-2024')
    expect(wrapper.get('.back').text()).toContain('Monsters')
    await detail.get('button.term').trigger('click')
    expect(wrapper.get('[data-testid="condition-popover"]').text()).toContain('on the ground')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('.close').trigger('click')
    expect(wrapper.find('[data-testid="condition-popover"]').exists()).toBe(false)
  })

  it('hides an empty fact list', async () => {
    const { wrapper } = await mountApp('/compendium/condition/prone', {
      '/api/v1/compendium/entries/condition/prone': () => ({ ...goblin, kind: 'condition', facts: [], mentions: [] }),
    })
    expect(wrapper.find('[data-testid="entry-facts"]').exists()).toBe(false)
  })

  it('reports missing entries and unknown kinds', async () => {
    const missing = await mountApp('/compendium/monster/nothing', {})
    expect(missing.wrapper.find('[data-testid="entry-missing"]').exists()).toBe(true)
    document.body.innerHTML = ''
    const unknown = await mountApp('/compendium/dragons/red', {})
    expect(unknown.wrapper.find('[data-testid="entry-missing"]').exists()).toBe(true)
    expect(unknown.calls).toHaveLength(0)
    expect(unknown.wrapper.find('.back').exists()).toBe(false)
  })
})

describe('automation coverage', () => {
  it('lists counts per kind', async () => {
    const { wrapper } = await mountApp('/about/automation', {
      '/api/v1/compendium/automation': () => [
        { kind: 'spell', total: 658, full: 0, partial: 0, manual: 658 },
        { kind: 'monster', total: 656, full: 0, partial: 0, manual: 656 },
      ],
    })
    const table = wrapper.get('[data-testid="automation-table"]')
    expect(table.text()).toContain('Spells')
    expect(table.text()).toContain('Monsters')
    expect(table.text()).toContain('656')
    await expectAccessible(wrapper.element as Element)
  })

  it('shows an error when the report fails', async () => {
    const { wrapper } = await mountApp('/about/automation', { '/api/v1/compendium/automation': problem })
    expect(wrapper.get('[role="alert"]').text()).toContain('could not be loaded')
  })
})
