import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CompendiumGuides } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const guides: CompendiumGuides = {
  spellsByLevel: [
    { level: 0, spells: [{ slug: 'fire-bolt', name: 'Fire Bolt', school: 'evocation' }] },
    { level: 1, spells: [{ slug: 'alarm', name: 'Alarm', school: 'abjuration' }, { slug: 'shield', name: 'Shield', school: 'abjuration' }] },
    { level: 2, spells: [] },
    { level: 3, spells: [{ slug: 'fireball', name: 'Fireball', school: 'evocation' }] },
  ],
  attacksByChallenge: [
    { challenge: '0', monsters: 1, attacks: 0, toHitLow: 0, toHit: 0, toHitHigh: 0, damage: 0 },
    { challenge: '1/4', monsters: 2, attacks: 3, toHitLow: 4, toHit: 4, toHitHigh: 5, damage: 5 },
    { challenge: '2', monsters: 1, attacks: 1, toHitLow: 6, toHit: 6, toHitHigh: 6, damage: 13 },
  ],
  lootTiers: [
    { tier: 1, fromLevel: 1, toLevel: 4, rarities: ['common'] },
    { tier: 2, fromLevel: 5, toLevel: 10, rarities: ['common', 'uncommon', 'rare'] },
    { tier: 3, fromLevel: 11, toLevel: 16, rarities: ['common', 'uncommon', 'rare', 'very-rare'] },
  ],
  lootByRarity: [
    { rarity: 'common', firstTier: 1, items: [{ slug: 'potion-of-healing', name: 'Potion of Healing' }] },
    { rarity: 'uncommon', firstTier: 1, items: [] },
    { rarity: 'very-rare', firstTier: 3, items: [{ slug: 'staff-of-power', name: 'Staff of Power' }, { slug: 'amulet', name: 'Amulet' }] },
  ],
}
const goblin = {
  kind: 'monster', slug: 'goblin', name: 'Goblin', subtitle: 'CR 1/4 · Humanoid', ruleset: 'srd-2024',
  facts: [], sections: [], mentions: [],
}
const fireBolt = {
  slug: 'fire-bolt', name: 'Fire Bolt', level: 0, school: 'evocation', ruleset: 'srd-2014', ritual: false, concentration: false,
  castingTime: 'action', rangeText: '120 feet', rangeFeet: 120, verbal: true, somatic: true, material: false,
  duration: 'instantaneous', description: 'A mote of fire.', classes: ['wizard'], damageTypes: ['fire'], attackRoll: true, scaling: [], mentions: [],
}

afterEach(() => {
  unmountAll()
  vi.unstubAllGlobals()
})

describe('the compendium guides', () => {
  it('lists the spells of each level, what the attacks of each Challenge Rating look like, and the loot that suits each tier', async () => {
    const { wrapper, calls } = await mountApp('/compendium/guides', { '/api/v1/compendium/guides': () => guides })
    expect(wrapper.get('h1').text()).toBe('Guides')
    expect(wrapper.get('nav[aria-label="Compendium"] [aria-current="page"]').text()).toBe('Guides')
    expect(calls.at(-1)?.search).toBe('')

    const levels = wrapper.findAll('[data-testid="spell-level"]')
    expect(levels.map((d) => d.get('summary').text())).toEqual(['Cantrips (1)', 'Level 1 (2)', 'Level 3 (1)'])
    expect(levels[1]?.findAll('li').map((li) => li.text())).toEqual(['Alarm · abjuration', 'Shield · abjuration'])
    expect(levels[1]?.findAll('a').map((a) => a.attributes('href'))).toEqual(['/compendium/spells/alarm', '/compendium/spells/shield'])

    const rows = wrapper.findAll('[data-testid="challenge-row"]')
    expect(rows.map((r) => r.findAll('th, td').map((c) => c.text()))).toEqual([
      ['0', '1', '0', '—', '—'],
      ['1/4', '2', '3', '+4 (+4 to +5)', '5'],
      ['2', '1', '1', '+6', '13'],
    ])

    const tiers = wrapper.findAll('[data-testid="loot-tier"]')
    expect(tiers.map((li) => li.text())).toEqual([
      'Tier 1, levels 1 to 4: common', 'Tier 2, levels 5 to 10: common, uncommon and rare', 'Tier 3, levels 11 to 16: common, uncommon, rare and very rare',
    ])
    const rarities = wrapper.findAll('[data-testid="loot-rarity"]')
    // A rarity with nothing in it is left out.
    expect(rarities.map((d) => d.get('summary').text())).toEqual(['Common magic items (1), from level 1', 'Very rare magic items (2), from level 11'])
    expect(rarities[1]?.findAll('a').map((a) => `${a.text()} ${a.attributes('href') ?? ''}`)).toEqual(['Staff of Power /compendium/magic-item/staff-of-power', 'Amulet /compendium/magic-item/amulet'])
    await expectAccessible(wrapper.element as Element)
  })

  it('reads one ruleset when asked, and keeps links within it', async () => {
    const { wrapper, calls, router } = await mountApp('/compendium/guides?ruleset=srd-2014', { '/api/v1/compendium/guides': () => guides })
    expect(calls.at(-1)?.search).toBe('?ruleset=srd-2014')
    expect(wrapper.get('nav[aria-label="Ruleset"] [aria-current="page"]').text()).toBe('2014')
    expect(wrapper.get('[data-testid="spell-level"] a').attributes('href')).toBe('/compendium/spells/fire-bolt?ruleset=srd-2014')
    expect(wrapper.get('[data-testid="loot-rarity"] a').attributes('href')).toBe('/compendium/magic-item/potion-of-healing?ruleset=srd-2014')
    await wrapper.get('nav[aria-label="Ruleset"] a:first-child').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/compendium/guides')
    expect(wrapper.get('nav[aria-label="Ruleset"] [aria-current="page"]').text()).toBe('Newest')
    expect(calls.at(-1)?.search).toBe('')
  })

  it('says so when the guides cannot be read', async () => {
    const { wrapper } = await mountApp('/compendium/guides', { '/api/v1/compendium/guides': () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503) })
    expect(wrapper.get('[data-testid="guides-missing"]').text()).toBe('The guides are not available just now.')
  })
})

describe('copying a link to an entry', () => {
  it('copies the address of the entry in the rules it is shown in, and says so', async () => {
    const copied: string[] = []
    vi.stubGlobal('navigator', { clipboard: { writeText: (text: string) => { copied.push(text); return Promise.resolve() } } })
    const { wrapper } = await mountApp('/compendium/monster/goblin', { '/api/v1/compendium/entries/monster/goblin': () => goblin })
    expect(wrapper.find('[data-testid="link-copied"]').exists()).toBe(false)
    await wrapper.get('[data-testid="copy-link"]').trigger('click')
    await flushPromises()
    expect(copied).toEqual([`${window.location.origin}/compendium/monster/goblin?ruleset=srd-2024`])
    expect(wrapper.get('[data-testid="link-copied"]').text()).toBe('Link copied.')
    await expectAccessible(wrapper.element as Element)
  })

  it('copies a spell\'s address too', async () => {
    const copied: string[] = []
    vi.stubGlobal('navigator', { clipboard: { writeText: (text: string) => { copied.push(text); return Promise.resolve() } } })
    const { wrapper } = await mountApp('/compendium/spells/fire-bolt?ruleset=srd-2014', { '/api/v1/compendium/spells/fire-bolt': () => fireBolt })
    await wrapper.get('[data-testid="copy-link"]').trigger('click')
    await flushPromises()
    expect(copied).toEqual([`${window.location.origin}/compendium/spells/fire-bolt?ruleset=srd-2014`])
  })

  it('shows the address to copy by hand where the browser will not copy it', async () => {
    vi.stubGlobal('navigator', { clipboard: { writeText: () => Promise.reject(new Error('denied')) } })
    const { wrapper } = await mountApp('/compendium/monster/goblin', { '/api/v1/compendium/entries/monster/goblin': () => goblin })
    await wrapper.get('[data-testid="copy-link"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="link-copied"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="link-to-copy"]').element as HTMLInputElement).value).toBe(`${window.location.origin}/compendium/monster/goblin?ruleset=srd-2024`)
    await expectAccessible(wrapper.element as Element)
  })
})
