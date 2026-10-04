import { afterEach, describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const me = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 1, createdAt: '2026-09-30T20:00:00Z', me: { ...me, role: as }, members: [{ ...me, role: as }],
})
const journal = { dm: false, entries: [], sessions: [], secrets: [] }
const tabs = (w: Awaited<ReturnType<typeof mountApp>>['wrapper']) => w.findAll('nav[aria-label="Campaign"] a')

afterEach(() => { unmountAll() })

describe('the frame around a Campaign\'s pages', () => {
  it('says where the page is and offers the DM every page of the Campaign as a tab', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, { [`/api/v1/campaigns/${ID}/journal`]: () => journal, [`/api/v1/campaigns/${ID}`]: () => campaign('dm') })
    const crumbs = wrapper.get('nav[aria-label="Breadcrumb"]')
    expect(crumbs.text().replace(/\s*›\s*/g, ' › ')).toBe('Campaigns › Morvain › Journal')
    expect(crumbs.findAll('a').map((a) => a.attributes('href'))).toEqual(['/campaigns', `/campaigns/${ID}`])
    expect(wrapper.get('[data-testid="to-campaign"]').text()).toBe('Morvain')
    expect(tabs(wrapper).map((a) => a.text())).toEqual([
      'Overview', 'Journal', 'Maps', 'NPCs', 'Factions', 'Encounters', 'Loot', 'Shops', 'Library', 'Proposals', 'Tracks', 'Downtime', 'Vehicles', 'Rule Variants', 'Dice', 'AI activity',
    ])
    expect(tabs(wrapper).filter((a) => a.attributes('aria-current') === 'page').map((a) => a.text())).toEqual(['Journal'])
    expect(tabs(wrapper)[0]?.attributes('href')).toBe(`/campaigns/${ID}`)
    // The page's own name comes between where it is and where else to go.
    const order = [...wrapper.get('main').element.children].map((c) => c.tagName === 'NAV' ? c.getAttribute('aria-label') : c.tagName)
    expect(order.slice(0, 3)).toEqual(['Breadcrumb', 'HEADER', 'Campaign'])
    await expectAccessible(wrapper.element as Element)
  })

  it('leaves the DM\'s pages out for a player, and manages without the Campaign\'s name', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/journal`, { [`/api/v1/campaigns/${ID}/journal`]: () => journal, [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    expect(tabs(wrapper).map((a) => a.text())).toEqual(['Overview', 'Journal', 'Factions', 'Proposals', 'Tracks', 'Downtime', 'Vehicles', 'Rule Variants', 'Dice'])
    unmountAll()
    const unnamed = await mountApp(`/campaigns/${ID}/journal`, { [`/api/v1/campaigns/${ID}/journal`]: () => journal })
    expect(unnamed.wrapper.get('[data-testid="to-campaign"]').text()).toBe('Campaign')
  })
})
