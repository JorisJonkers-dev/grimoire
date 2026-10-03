import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Faction } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const WATCH = '0190c7a8-0000-7000-8000-0000000000f1'
const SEAL = '0190c7a8-0000-7000-8000-0000000000c1'
const KILL = '0190c7a8-0000-7000-8000-0000000000c2'
const TAMSIN = '0190c7a8-0000-7000-8000-0000000000a1'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [{ ...dm, isMe: as === 'dm' }, { ...player, isMe: as === 'player' }],
})
const archetypes = [
  { slug: 'city-watch', name: 'City watch', description: 'The sworn keepers of order in a town or city.', goals: 'Keep the peace.' },
  { slug: 'cult', name: 'Cult', description: 'A hidden circle bound to a forbidden power.', goals: 'Grow in secret.' },
]
const at = '2026-10-03T10:00:00Z'
const seenByPlayer: Faction = {
  id: WATCH, name: 'The Lantern Watch', archetype: 'city-watch', tier: 'unfriendly',
  personal: [{ characterId: TAMSIN, character: 'Tamsin', tier: 'friendly' }],
  changes: [
    { id: SEAL, rose: true, reason: 'Returned the stolen seal.', status: 'confirmed', createdAt: at },
    { id: KILL, rose: false, reason: '', status: 'confirmed', createdAt: at },
  ],
}
const seenByDM: Faction = {
  ...seenByPlayer,
  dm: { goals: 'Keep the peace.', territory: 'Oakford', notes: 'The captain takes bribes.', score: -20 },
  personal: [{ characterId: TAMSIN, character: 'Tamsin', tier: 'friendly', score: 25 }],
  changes: [
    { id: KILL, rose: false, reason: 'Killed a watchman.', status: 'pending', createdAt: at, dm: { delta: -70, shareReason: false, origin: 'mcp', client: 'Claude' } },
    { id: SEAL, rose: true, reason: 'Returned the stolen seal.', status: 'confirmed', createdAt: at, dm: { delta: 30, shareReason: true, origin: 'ui', client: '' } },
    { id: '0190c7a8-0000-7000-8000-0000000000c3', rose: true, reason: 'A rumour.', status: 'dismissed', createdAt: at, dm: { delta: 40, shareReason: true, origin: 'ui', client: '' } },
    { id: '0190c7a8-0000-7000-8000-0000000000c4', rose: true, reason: 'A favour.', status: 'pending', createdAt: at, dm: { delta: 5, shareReason: false, origin: 'ui', client: '' } },
    { id: '0190c7a8-0000-7000-8000-0000000000c5', rose: false, reason: 'A member died.', status: 'pending', createdAt: at, dm: { delta: -10, shareReason: false, origin: 'system', client: '' } },
    { id: '0190c7a8-0000-7000-8000-0000000000c6', rose: false, reason: 'A slight.', status: 'confirmed', createdAt: at, dm: { delta: -5, shareReason: false, origin: 'ui', client: '' } },
  ],
}
const bare: Faction = { id: '0190c7a8-0000-7000-8000-0000000000f2', name: 'Grey Hands', archetype: '', tier: 'allied', dm: { goals: '', territory: '', notes: '', score: 80 }, personal: [], changes: [] }

afterEach(() => { unmountAll() })

function backend(as: 'dm' | 'player', list: Faction[], fail = 0) {
  const calls: string[] = []
  const write = async (url: URL, req: Request) => {
    calls.push(`${req.method} ${url.pathname.split('/').slice(5).join('/')} ${req.method === 'DELETE' ? '' : await req.text()}`.trim())
    if (fail) return jsonResponse({ type: 'about:blank', title: 'No', status: fail }, fail)
    return new Response(null, { status: 204 })
  }
  const routes = {
    '/api/v1/faction-archetypes': () => archetypes,
    [`/api/v1/campaigns/${ID}/standing-changes/`]: write,
    [`/api/v1/campaigns/${ID}/factions/`]: async (url: URL, req: Request) => {
      if (req.method !== 'POST') return write(url, req)
      calls.push(`POST ${url.pathname.split('/').slice(5).join('/')} ${await req.text()}`)
      if (fail) return jsonResponse({ type: 'about:blank', title: 'No', status: fail }, fail)
      return jsonResponse({ id: KILL, rose: true, reason: 'x', status: 'pending', createdAt: at }, 201)
    },
    [`/api/v1/campaigns/${ID}/factions`]: async (_u: URL, req: Request) => {
      if (req.method === 'GET') return list
      calls.push(`POST factions ${await req.text()}`)
      if (fail) return jsonResponse({ type: 'about:blank', title: 'No', status: fail }, fail)
      return jsonResponse({ ...seenByDM, id: '0190c7a8-0000-7000-8000-0000000000f9', name: 'New', changes: [], personal: [] }, 201)
    },
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  }
  return { calls, routes }
}

describe('factions', () => {
  it('shows a Player tiers and the reasons the DM shared, and nothing to change them with', async () => {
    const { routes } = backend('player', [seenByPlayer])
    const { wrapper } = await mountApp(`/campaigns/${ID}/factions`, routes)
    await flushPromises()
    const card = wrapper.get(`[data-testid="faction-${WATCH}"]`)
    expect(card.get('h2').text()).toBe('The Lantern Watch')
    expect(card.get('[data-testid="tier"]').text()).toBe('Unfriendly')
    expect(card.get('[data-testid="archetype"]').text()).toBe('City watch')
    expect(card.get('[data-testid="personal"]').text()).toBe('Tamsin: Friendly')
    expect(card.findAll('[data-testid^="change-"]').map((c) => c.text())).toEqual(['Standing rose: Returned the stolen seal.', 'Standing fell.'])
    for (const id of ['faction-add', 'suggest', 'faction-secrets', 'faction-remove', 'decide']) expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    expect(card.text()).not.toMatch(/-?\d/)
    await expectAccessible(wrapper.element as Element)
  })

  it('says so when there are none', async () => {
    const { routes } = backend('player', [])
    const { wrapper } = await mountApp(`/campaigns/${ID}/factions`, routes)
    await flushPromises()
    expect(wrapper.get('[data-testid="no-factions"]').text()).toBe('No Factions in this Campaign yet.')
  })

  it('lets the DM add a Faction from the catalogue, suggest a change and decide the ones that wait', async () => {
    const { calls, routes } = backend('dm', [seenByDM, bare])
    const { wrapper } = await mountApp(`/campaigns/${ID}/factions`, routes)
    await flushPromises()
    const card = wrapper.get(`[data-testid="faction-${WATCH}"]`)
    expect(card.get('[data-testid="tier"]').text()).toBe('Unfriendly')
    expect(card.get('[data-testid="faction-secrets"]').text()).toContain('Score -20')
    expect(card.get('[data-testid="faction-secrets"]').text()).toContain('The captain takes bribes.')
    expect(card.get('[data-testid="personal"]').text()).toBe('Tamsin: Friendly (25)')
    expect(card.get(`[data-testid="change-${SEAL}"]`).text()).toBe('Standing rose by 30: Returned the stolen seal. Shared with the Players.')
    expect(card.get('[data-testid="change-0190c7a8-0000-7000-8000-0000000000c3"]').text()).toBe('Dismissed: rose by 40. A rumour.')
    expect(card.get('[data-testid="change-0190c7a8-0000-7000-8000-0000000000c4"]').text()).toContain('Suggested by you: rose by 5. A favour.')
    expect(card.get('[data-testid="change-0190c7a8-0000-7000-8000-0000000000c5"]').text()).toContain('Suggested by system: fell by 10. A member died.')
    expect(card.get('[data-testid="change-0190c7a8-0000-7000-8000-0000000000c6"]').text()).toBe('Standing fell by 5: A slight. Not shared.')
    // A Faction made from nothing has no archetype, and nothing more for the DM than its score.
    const other = wrapper.get(`[data-testid="faction-${bare.id}"]`)
    expect(other.find('[data-testid="archetype"]').exists()).toBe(false)
    expect(other.get('[data-testid="faction-secrets"]').text()).toBe('For the DMScore 80')
    expect(other.get('[data-testid="tier"]').classes()).toContain('tier--allied')
    // The change that waits says where it came from, and takes the DM's word.
    const waiting = card.get(`[data-testid="change-${KILL}"]`)
    expect(waiting.text()).toContain('Suggested by Claude: fell by 70. Killed a watchman.')
    await waiting.get('[data-testid="decide-delta"]').setValue(-50)
    await waiting.get('[data-testid="decide-reason"]').setValue('A watchman died in the brawl.')
    await waiting.get('[data-testid="decide-share"]').setValue(true)
    await waiting.get('[data-testid="decide-confirm"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST standing-changes/${KILL}/decision {"confirm":true,"delta":-50,"reason":"A watchman died in the brawl.","shareReason":true}`)
    await waiting.get('[data-testid="decide-dismiss"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST standing-changes/${KILL}/decision {"confirm":false}`)

    // A suggestion, for the party or for one Character with a Personal Standing.
    const suggest = card.get('[data-testid="suggest"]')
    expect(suggest.get('button').attributes('disabled')).toBeDefined()
    await suggest.get('[data-testid="suggest-delta"]').setValue(15)
    await suggest.get('[data-testid="suggest-reason"]').setValue('Paid the fine.')
    await suggest.get('[data-testid="suggest-share"]').setValue(true)
    await suggest.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST factions/${WATCH}/standing-changes {"delta":15,"reason":"Paid the fine.","shareReason":true}`)
    await suggest.get('[data-testid="suggest-delta"]').setValue(-5)
    await suggest.get('[data-testid="suggest-reason"]').setValue('Tamsin was rude.')
    await suggest.get('[data-testid="suggest-who"]').setValue(TAMSIN)
    await suggest.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe(`POST factions/${WATCH}/standing-changes {"delta":-5,"reason":"Tamsin was rude.","shareReason":false,"characterId":"${TAMSIN}"}`)

    // An archetype fills in a name and goals to start from.
    const add = wrapper.get('[data-testid="faction-add"]')
    await add.get('[data-testid="faction-archetype"]').setValue('cult')
    expect((add.get('[data-testid="faction-name"]').element as HTMLInputElement).value).toBe('Cult')
    expect((add.get('[data-testid="faction-goals"]').element as HTMLTextAreaElement).value).toBe('Grow in secret.')
    await add.get('[data-testid="faction-name"]').setValue('The Ashen Hand')
    await add.get('[data-testid="faction-notes"]').setValue('They meet under the mill.')
    await add.trigger('submit')
    await flushPromises()
    expect(calls.at(-1)).toBe('POST factions {"name":"The Ashen Hand","archetype":"cult","goals":"Grow in secret.","territory":"","notes":"They meet under the mill."}')
    expect((add.get('[data-testid="faction-name"]').element as HTMLInputElement).value).toBe('')
    await card.get('[data-testid="faction-remove"]').trigger('click')
    await flushPromises()
    expect(calls.at(-1)).toBe(`DELETE factions/${WATCH}`)
    await expectAccessible(wrapper.element as Element)
  })

  it('says when something could not be done', async () => {
    const { routes } = backend('dm', [seenByDM], 422)
    const { wrapper } = await mountApp(`/campaigns/${ID}/factions`, routes)
    await flushPromises()
    await wrapper.get('[data-testid="faction-name"]').setValue('X')
    await wrapper.get('[data-testid="faction-add"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="faction-problem"]').text()).toBe('That could not be done. Check what you entered and try again.')
    const denied = await mountApp(`/campaigns/${ID}/factions`, { [`/api/v1/campaigns/${ID}/factions`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404), [`/api/v1/campaigns/${ID}`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404) })
    await flushPromises()
    expect(denied.wrapper.get('[data-testid="factions-missing"]').text()).toBe('That Campaign is not available.')
  })
})
