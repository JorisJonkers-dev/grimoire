import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const WATCH = '0190c7a8-0000-7000-8000-0000000000f1'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const factions = [{ id: WATCH, name: 'The Lantern Watch', archetype: 'city-watch', tier: 'allied', personal: [], changes: [], dm: { goals: '', territory: '', notes: '', score: 80 } }]
const ARIA = '0190c7a8-0000-7000-8000-0000000000a1'
const BROM = '0190c7a8-0000-7000-8000-0000000000a2'
const watchman: LiveToken = {
  id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Watchman', kind: 'npc', q: 1, r: 0, hidden: false, darkvisionFt: 0, factionId: WATCH, firstReaction: 'friendly',
  attitudes: [{ characterId: ARIA, attitude: 'friendly' }, { characterId: BROM, attitude: 'hostile' }],
}
const stray: LiveToken = { ...watchman, id: '0190c7a8-0000-7000-8000-00000000000c', label: 'Stray', q: 0, r: 1, factionId: undefined, firstReaction: undefined, attitudes: undefined }

async function open(as: 'dm' | 'player', tokens: LiveToken[]) {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/factions`]: () => factions,
    [`/api/v1/campaigns/${ID}/maps`]: () => [],
    [`/api/v1/campaigns/${ID}/characters`]: () => [{ id: ARIA, name: 'Aria', ownerName: 'T', mine: false, species: 'human', class: 'fighter', level: 1, hpCurrent: 1, hpMax: 1 }],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: { tokens, fog: false, visible: [], remembered: [] }, session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('a creature of a Faction', () => {
  it('is placed by the DM as one of a Faction, and says how it first takes to the party', async () => {
    const { wrapper, s } = await open('dm', [watchman, stray])
    expect(wrapper.get('[data-testid="token-faction"]').findAll('option').map((o) => o.text())).toEqual(['No Faction', 'The Lantern Watch'])
    await wrapper.get('[data-testid="token-label"]').setValue('Sergeant')
    await wrapper.get('[data-testid="token-faction"]').setValue(WATCH)
    await wrapper.get('[data-hex="-1,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', label: 'Sergeant', factionId: WATCH, q: -1, r: 0 })
    // With no Faction chosen, none is sent.
    await wrapper.get('[data-testid="token-faction"]').setValue('')
    await wrapper.get('[data-testid="token-label"]').setValue('Beggar')
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(s.sent.at(-1)).not.toHaveProperty('factionId')

    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(wrapper.get('[data-testid="token-faction-line"]').text()).toBe('Of The Lantern Watch. First reaction: friendly.')
    // How it takes to each Character an Influence check has moved it towards; one the list does not know is still told.
    expect(wrapper.findAll('[data-testid="token-attitude"]').map((li) => li.text())).toEqual(['Friendly towards Aria', 'Hostile towards a Character'])
    // A swaying check is aimed at whoever is chosen: the Standing with its Faction shapes the roll.
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(wrapper.find('[data-testid="token-faction-line"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="token-attitude"]').exists()).toBe(false)
  })

  it('lets a Player aim an Influence check at a creature', async () => {
    const aria: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id, ac: 16, hp: 12, hpMax: 12, attacks: [{ name: 'Longsword', toHit: 5, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d8', damageBonus: 3, damageType: 'slashing' }] }
    const { wrapper, s } = await open('player', [aria, { ...watchman, firstReaction: undefined }])
    const fighter = (t: LiveToken, acting: boolean) => ({
      id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId, rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`,
      initiative: 10, rank: 1, acting, done: false, action: true, bonusAction: true, reaction: true, movementFt: 30, speedFt: 30,
    })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, { ...watchman, firstReaction: undefined }, stray], fog: false, visible: [], remembered: [], combat: { status: 'active', round: 1, combatants: [fighter(aria, true), fighter(watchman, false)] } } })
    await flushPromises()
    // A creature of a Faction is openly one: whoever aims at it sees whose it is.
    expect(wrapper.get('[data-testid="influence-target"]').findAll('option').map((o) => o.text())).toEqual(['Nobody in particular', 'Watchman (The Lantern Watch)', 'Stray'])
    await wrapper.get('[data-testid="action-influence"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'take_action', action: 'influence' })
    expect(s.sent.at(-1)).not.toHaveProperty('targetId')
    await wrapper.get('[data-testid="influence-target"]').setValue(watchman.id)
    await wrapper.get('[data-testid="action-influence"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'take_action', tokenId: aria.id, action: 'influence', targetId: watchman.id })
    // Another action is not aimed at anyone.
    await wrapper.get('[data-testid="action-search"]').trigger('click')
    expect(s.sent.at(-1)).not.toHaveProperty('targetId')
  })

  it('gives a Player no way to place a creature of a Faction', async () => {
    const { wrapper } = await open('player', [{ ...watchman, firstReaction: undefined }])
    expect(wrapper.find('[data-testid="token-faction"]').exists()).toBe(false)
  })
})
