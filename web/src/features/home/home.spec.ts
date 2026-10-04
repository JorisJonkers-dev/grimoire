import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Dashboard, SearchResults } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { fakeClock, mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const CID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const CID2 = '0190c7a8-0000-7000-8000-000000000002'
const SID2 = '0190c7a8-0000-7000-8000-00000000000c'
const busy: Dashboard = {
  live: [
    { campaignId: CID, campaign: 'Morvain', sessionId: SID, number: 3, dm: true },
    { campaignId: CID2, campaign: 'Fire Reach', sessionId: SID2, number: 12, dm: false },
  ],
  needs: [
    { kind: 'level_up', campaign: 'Fire Reach', title: 'Kara can level up', path: `/campaigns/${CID2}/characters/x/level-up` },
    { kind: 'proposals', campaign: 'Morvain', title: '2 Proposals to review', path: `/campaigns/${CID}/proposals` },
    { kind: 'friend_requests', title: '1 Friend Request to answer', path: '/friends' },
  ],
}
const quiet: Dashboard = { live: [], needs: [] }
const found: SearchResults = {
  hits: [
    { group: 'compendium', kind: 'spell', title: 'Fireball', preview: 'Level 3 evocation spell', path: '/compendium/spells/fireball' },
    { group: 'compendium', kind: 'monster', title: 'Fire Elemental', preview: 'Large elemental · CR 5', path: '/compendium/monster/fire-elemental' },
    { group: 'library', kind: 'creature', title: 'Fire Drake', preview: 'Your Library · creature', path: '/library/abc' },
    { group: 'campaigns', kind: 'campaign', title: 'Fire Reach', preview: 'You play in this Campaign', path: `/campaigns/${CID2}` },
    { group: 'people', kind: 'friend', title: 'Firebeard', preview: 'Friend · @bram', path: '/friends' },
  ],
}
const denied = () => jsonResponse({ type: 'about:blank', title: 'Unauthorized', status: 401 }, 401)

afterEach(() => {
  unmountAll()
  vi.useRealTimers()
})

describe('the Dashboard', () => {
  it('greets me, counts what needs me, and lists the Sessions under way with a way in', async () => {
    const who = { id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'aria@example.com', admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false }
    const mine = { items: [
      { id: CID, name: 'Morvain', ruleset: 'srd-2024', myRole: 'dm', memberCount: 3, createdAt: '2026-09-30T20:00:00Z' },
      { id: CID2, name: 'Fire Reach', ruleset: 'srd-2024', myRole: 'player', memberCount: 4, createdAt: '2026-09-30T20:00:00Z' },
    ] }
    const { wrapper } = await mountApp('/', { '/api/v1/dashboard': () => busy, '/api/v1/account': () => who, '/api/v1/campaigns': () => mine })
    expect(wrapper.get('[data-testid="greeting"]').text()).toMatch(/^Good (morning|afternoon|evening), Aria\.$/)
    expect(wrapper.get('h1').text()).toBe('Three things need you')
    const live = wrapper.findAll('[data-testid="live-session"]')
    expect(live.map((row) => row.text())).toEqual(['Morvain is playing now · Session 3 · you are the DMJoin', 'Fire Reach is playing now · Session 12Join'])
    expect(live.map((row) => row.get('a').attributes('href'))).toEqual([`/campaigns/${CID}/sessions/${SID}`, `/campaigns/${CID2}/sessions/${SID2}`])
    expect(live.map((row) => row.get('a').attributes('aria-label'))).toEqual(['Join Session 3 of Morvain', 'Join Session 12 of Fire Reach'])
    // Each thing that needs me: what it is, the word for dealing with it, and where it belongs.
    const needs = wrapper.findAll('[data-testid="need"]')
    expect(needs.map((row) => row.findAll('span').map((part) => part.text()))).toEqual([
      ['Kara can level up', 'Fire Reach · Advancement'], ['2 Proposals to review', 'Morvain · Proposal'], ['1 Friend Request to answer', 'Friends'],
    ])
    expect(needs.map((row) => row.get('a').text())).toEqual(['Level up', 'Review', 'Answer'])
    expect(needs.map((row) => row.get('a').attributes('href'))).toEqual([`/campaigns/${CID2}/characters/x/level-up`, `/campaigns/${CID}/proposals`, '/friends'])
    expect(needs.map((row) => row.get('a').attributes('aria-label'))).toEqual(['Level up: Kara can level up', 'Review: 2 Proposals to review', 'Answer: 1 Friend Request to answer'])
    expect(wrapper.find('[data-testid="nothing-needed"]').exists()).toBe(false)
    // My Campaigns stand at the side, each with what I am in it.
    const campaigns = wrapper.findAll('[data-testid="dash-campaign"]')
    expect(campaigns.map((c) => c.findAll('span').map((part) => part.text()))).toEqual([['Morvain', 'Dungeon Master'], ['Fire Reach', 'Player']])
    expect(campaigns.map((c) => c.attributes('href'))).toEqual([`/campaigns/${CID}`, `/campaigns/${CID2}`])
    expect(wrapper.get('[data-testid="dash-new-campaign"]').text()).toBe('All Campaigns')
    await expectAccessible(wrapper.element as Element)
  })

  it('says so when nothing is under way and nothing needs me', async () => {
    const { wrapper } = await mountApp('/', { '/api/v1/dashboard': () => quiet, '/api/v1/campaigns': () => ({ items: [] }) })
    expect(wrapper.find('[data-testid="dashboard"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="greeting"]').text()).toMatch(/^Good (morning|afternoon|evening)\.$/)
    expect(wrapper.get('h1').text()).toBe('Nothing needs you')
    expect(wrapper.find('[data-testid="live-sessions"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="live-session"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="nothing-needed"]').text()).toBe('Nothing needs you before the next Session.')
    // With no Campaign yet, the side offers to start one.
    expect(wrapper.find('[data-testid="dash-campaign"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="dash-new-campaign"]').text()).toBe('New Campaign')
    expect(wrapper.get('[data-testid="dash-new-campaign"]').attributes('href')).toBe('/campaigns')
  })

  it('counts one thing as one thing, and more than nine in figures', async () => {
    const one = await mountApp('/', { '/api/v1/dashboard': () => ({ live: [], needs: busy.needs.slice(0, 1) }) })
    expect(one.wrapper.get('h1').text()).toBe('One thing needs you')
    unmountAll()
    const many = await mountApp('/', { '/api/v1/dashboard': () => ({ live: [], needs: Array.from({ length: 11 }, () => busy.needs[0]) }) })
    expect(many.wrapper.get('h1').text()).toBe('11 things need you')
  })

  it('shows nothing of it to somebody signed out', async () => {
    const { wrapper } = await mountApp('/', { '/api/v1/dashboard': denied })
    expect(wrapper.find('[data-testid="dashboard"]').exists()).toBe(false)
    expect(wrapper.find('h1').text()).toBe('Grimoire')
  })
})

describe('the search', () => {
  it('finds as I type, once I pause, and shows each result with its preview under its group', async () => {
    fakeClock()
    const asked: string[] = []
    const { wrapper } = await mountApp('/search', {
      '/api/v1/search': (url: URL) => {
        asked.push(url.search)
        return url.searchParams.get('q') === 'fire' ? found : { hits: [] }
      },
    })
    const box = wrapper.get('[data-testid="search-input"]')
    expect(wrapper.get('[data-testid="search-hint"]').text()).toBe('Type at least 2 characters.')
    // One letter is not looked for.
    await box.setValue('f')
    vi.advanceTimersByTime(400)
    await flushPromises()
    expect(asked).toEqual([])
    expect(wrapper.find('[data-testid="search-hint"]').exists()).toBe(true)
    // Typing on does not ask for every letter: only what stands when the typing pauses.
    await box.setValue('fi')
    vi.advanceTimersByTime(100)
    await box.setValue('fir')
    vi.advanceTimersByTime(100)
    await box.setValue(' fire ')
    vi.advanceTimersByTime(249)
    await flushPromises()
    expect(asked).toEqual([])
    vi.advanceTimersByTime(1)
    await flushPromises()
    expect(asked).toEqual(['?q=fire'])
    expect(wrapper.find('[data-testid="search-hint"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="search-group"] h2').map((h) => h.text())).toEqual(['Compendium', 'Library', 'Campaigns', 'People'])
    const hits = wrapper.findAll('[data-testid="search-hit"]')
    expect(hits.map((li) => li.text())).toEqual([
      'FireballLevel 3 evocation spell', 'Fire ElementalLarge elemental · CR 5', 'Fire DrakeYour Library · creature', 'Fire ReachYou play in this Campaign', 'FirebeardFriend · @bram',
    ])
    expect(hits.map((li) => li.get('a').attributes('href'))).toEqual(['/compendium/spells/fireball', '/compendium/monster/fire-elemental', '/library/abc', `/campaigns/${CID2}`, '/friends'])
    expect(wrapper.findAll('[data-testid="search-group"]')[0]?.findAll('[data-testid="search-hit"]')).toHaveLength(2)
    expect(wrapper.get('[data-testid="search-count"]').text()).toBe('5 results for “fire”.')
    vi.useRealTimers()
    await expectAccessible(wrapper.element as Element)
    fakeClock()

    // Nothing found says so, by what was looked for.
    await box.setValue('zzz')
    vi.advanceTimersByTime(250)
    await flushPromises()
    expect(asked.at(-1)).toBe('?q=zzz')
    expect(wrapper.get('[data-testid="search-count"]').text()).toBe('Nothing found for “zzz”.')
    expect(wrapper.find('[data-testid="search-group"]').exists()).toBe(false)
  })

  it('searches at once for what the address already asks for, and keeps the address in step', async () => {
    fakeClock()
    const { wrapper, router, calls } = await mountApp('/search?q=fire', { '/api/v1/search': () => found })
    expect(calls.filter((u) => u.pathname === '/api/v1/search').map((u) => u.search)).toEqual(['?q=fire'])
    expect((wrapper.get('[data-testid="search-input"]').element as HTMLInputElement).value).toBe('fire')
    expect(wrapper.findAll('[data-testid="search-hit"]')).toHaveLength(5)
    expect(wrapper.get('[data-testid="search-count"]').text()).toBe('5 results for “fire”.')
    await wrapper.get('[data-testid="search-input"]').setValue('drake')
    vi.advanceTimersByTime(250)
    await flushPromises()
    expect(router.currentRoute.value.query.q).toBe('drake')
    expect(calls.at(-1)?.search).toBe('?q=drake')
    // Two characters are enough to look for.
    await wrapper.get('[data-testid="search-input"]').setValue('fi')
    vi.advanceTimersByTime(250)
    await flushPromises()
    expect(calls.at(-1)?.search).toBe('?q=fi')
    expect(wrapper.find('[data-testid="search-hint"]').exists()).toBe(false)
  })

  it('narrows to one place to look, and previews the result in hand beside the list', async () => {
    const { wrapper } = await mountApp('/search?q=fire', { '/api/v1/search': () => found })
    const scopes = () => wrapper.findAll('[data-testid="search-scopes"] button')
    expect(scopes().map((b) => b.text())).toEqual(['Everything · 5', 'Compendium · 2', 'Library · 1', 'Campaigns · 1', 'People · 1'])
    expect(scopes().map((b) => b.attributes('aria-pressed'))).toEqual(['true', 'false', 'false', 'false', 'false'])
    const preview = wrapper.get('[data-testid="search-preview"]')
    expect(preview.get('.g-eyebrow').text()).toBe('Compendium · spell')
    expect(preview.get('h2').text()).toBe('Fireball')
    expect(preview.get('p').text()).toBe('Level 3 evocation spell')
    expect(preview.get('a').attributes('href')).toBe('/compendium/spells/fireball')
    expect(preview.get('a').text()).toBe('Open')
    // The result the keyboard or the pointer is on is the one previewed.
    await wrapper.findAll('[data-testid="search-hit"] a')[2]?.trigger('focus')
    expect(preview.get('h2').text()).toBe('Fire Drake')
    await wrapper.findAll('[data-testid="search-hit"] a')[4]?.trigger('mouseenter')
    expect(preview.get('h2').text()).toBe('Firebeard')
    await scopes()[1]?.trigger('click')
    expect(scopes().map((b) => b.attributes('aria-pressed'))).toEqual(['false', 'true', 'false', 'false', 'false'])
    expect(wrapper.findAll('[data-testid="search-hit"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="search-group"] h2').map((h) => h.text())).toEqual(['Compendium'])
    // What was in hand is no longer listed, so the first that is takes its place.
    expect(preview.get('h2').text()).toBe('Fireball')
    expect(wrapper.get('[data-testid="search-count"]').text()).toBe('5 results for “fire”.')
    await scopes()[0]?.trigger('click')
    expect(wrapper.findAll('[data-testid="search-hit"]')).toHaveLength(5)
    await expectAccessible(wrapper.element as Element)
  })

  it('says so when the search cannot be made', async () => {
    const { wrapper } = await mountApp('/search?q=fire', { '/api/v1/search': denied })
    expect(wrapper.get('[data-testid="search-problem"]').text()).toBe('Search is not available just now. Sign in and try again.')
    expect(wrapper.find('[data-testid="search-count"]').exists()).toBe(false)
  })

  it('is reached from the header, with what was typed there', async () => {
    const { wrapper, router } = await mountApp('/', { '/api/v1/dashboard': () => quiet, '/api/v1/search': () => ({ hits: [found.hits[0]] }) })
    await wrapper.get('[data-testid="header-search-input"]').setValue('  fireball ')
    await wrapper.get('[data-testid="header-search"]').trigger('submit')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/search?q=fireball')
    expect(wrapper.get('[data-testid="search-count"]').text()).toBe('1 result for “fireball”.')
    // The header box is not shown on the search page itself, which has its own.
    expect(wrapper.find('[data-testid="header-search"]').exists()).toBe(false)
    // Nothing typed goes to the page to type there.
    await router.push('/')
    await flushPromises()
    await wrapper.get('[data-testid="header-search"]').trigger('submit')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/search')
  })
})
