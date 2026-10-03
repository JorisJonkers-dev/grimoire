import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const FANG = '0190c7a8-0000-7000-8000-0000000000d1'
const BORS = '0190c7a8-0000-7000-8000-0000000000d2'
const companions = [
  { id: FANG, name: 'Fang', kind: 'companion', monsterSlug: 'wolf', controllerId: player.id, sharesXp: true, notes: '', updatedAt: '2026-10-03T10:00:00Z' },
  { id: BORS, name: 'Bors', kind: 'hireling', monsterSlug: 'goblin', sharesXp: false, notes: '', updatedAt: '2026-10-03T10:00:00Z' },
]
const fang = { id: '0190c7a8-0000-7000-8000-0000000000e1', label: 'Fang', kind: 'party', q: 1, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id, companionId: FANG, ac: 13, hp: 11, hpMax: 11, attacks: [] }
const aria = { id: '0190c7a8-0000-7000-8000-0000000000e2', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id, ac: 16, hp: 12, hpMax: 12, attacks: [] }
const entry = (t: { id: string; label: string; kind: string }, companion = false) => ({ tokenId: t.id, label: t.label, kind: t.kind, health: 'unhurt', hidden: false, acting: false, effects: [], ...(companion ? { companion: true } : {}) })
const view = { tokens: [aria, fang], fog: false, visible: [], remembered: [], roster: [entry(aria), entry(fang, true)], gameDay: 0 }

async function open(as: 'dm' | 'player') {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/sessions/`]: () => [],
    [`/api/v1/campaigns/${ID}/companions`]: () => companions,
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view, session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('Companions in live play', () => {
  it('lets the DM put a Companion on the map and hand it from Player to DM and back', async () => {
    const { wrapper, s } = await open('dm')
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    // Only a Companion that is not on the map yet can be placed.
    expect(wrapper.findAll('[data-testid="token-companion"] option').map((o) => o.text())).toEqual(['None', 'Bors'])
    await wrapper.get('[data-testid="token-companion"]').setValue(BORS)
    await wrapper.get('[data-hex="-1,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'place_token', companionId: BORS, q: -1, r: 0 })
    expect(sent().monsterSlug).toBeUndefined()
    expect((wrapper.get('[data-testid="token-companion"]').element as HTMLSelectElement).value).toBe('')

    // The roster marks who is a Companion.
    expect(wrapper.get('[data-testid="rail-Fang"]').find('[data-testid="companion-Fang"]').text()).toBe('Companion')
    expect(wrapper.get('[data-testid="rail-Aria"]').find('[data-testid="companion-Aria"]').exists()).toBe(false)

    // Its token says who runs it, and the DM changes that; another token has no such choice.
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    const hand = wrapper.get('[data-testid="token-hand"]')
    expect((hand.element as HTMLSelectElement).value).toBe(player.id)
    expect(hand.findAll('option').map((o) => o.text())).toEqual(['The DM', 'Joris', 'Aria'])
    await hand.setValue('')
    expect(sent()).toEqual(expect.objectContaining({ kind: 'assign_control', tokenId: fang.id }))
    expect(sent().controllerId).toBeUndefined()
    await hand.setValue(player.id)
    expect(sent()).toMatchObject({ kind: 'assign_control', tokenId: fang.id, controllerId: player.id })
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(wrapper.find('[data-testid="token-hand"]').exists()).toBe(false)
  })

  it('shows a Player the Companions in the roster, and asks for no list of them', async () => {
    const { wrapper, calls } = await open('player')
    expect(wrapper.get('[data-testid="rail-Fang"]').find('[data-testid="companion-Fang"]').text()).toBe('Companion')
    expect(wrapper.find('[data-testid="token-companion"]').exists()).toBe(false)
    expect(calls.some((u) => u.pathname.endsWith('/companions'))).toBe(false)
  })
})
