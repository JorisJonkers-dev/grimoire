import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const AWAY = '0190c7a8-0000-7000-8000-00000000000c'
const TOWER = '0190c7a8-0000-7000-8000-0000000000a1'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const token = (n: number, label: string, kind: string, q: number) => ({ id: `0190c7a8-0000-7000-8000-0000000000e${String(n)}`, label, kind, q, r: 0, hidden: false, darkvisionFt: 0 })
const aria = token(1, 'Aria', 'party', 0)
const brom = token(2, 'Brom', 'party', 1)
const goblin = token(3, 'Goblin', 'enemy', 2)
const maps = [
  { id: '0190c7a8-0000-7000-8000-0000000000a0', name: 'Crypt', kind: 'local', width: 400, height: 300, hexSizePx: 40, originX: 35, originY: 40, imageUrl: '/api/v1/x', ambient: 'bright', gridKind: 'hexes', gridStrength: 20, scaleMiles: 6 },
  { id: TOWER, name: 'Tower', kind: 'local', width: 400, height: 300, hexSizePx: 40, originX: 35, originY: 40, imageUrl: '/api/v1/y', ambient: 'bright', gridKind: 'hexes', gridStrength: 20, scaleMiles: 6 },
  { id: '0190c7a8-0000-7000-8000-0000000000a2', name: 'Realm', kind: 'world', width: 400, height: 300, hexSizePx: 40, originX: 35, originY: 40, imageUrl: '/api/v1/z', ambient: 'bright', gridKind: 'hexes', gridStrength: 20, scaleMiles: 6 },
]
const group = (sessionId: string, name: string, extra: Record<string, unknown>) => ({ sessionId, number: 1, name, home: false, here: false, table: false, tokens: [], ...extra })
const view = (extra: Record<string, unknown> = {}) => ({ tokens: [aria, brom, goblin], fog: false, visible: [], remembered: [], roster: [], gameDay: 0, ...extra })
const snapshot = (sid: string, audience: string, v: Record<string, unknown> = {}) => ({ kind: 'snapshot', seq: 1, view: view(v), session: { id: sid, number: 3, gridRadius: 2, audience } })

async function open(as: 'dm' | 'player', path = `/campaigns/${ID}/sessions/${SID}`) {
  const mounted = await mountApp(path, {
    [`/api/v1/campaigns/${ID}/sessions/`]: () => [],
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}/maps`]: () => maps,
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  return { ...mounted, s: FakeSocket.last() }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('a split party', () => {
  it('lets the DM send party tokens off as a group to another map', async () => {
    const { wrapper, s } = await open('dm')
    s.receive(snapshot(SID, 'dm'))
    await flushPromises()
    const panel = wrapper.get('[data-testid="groups"]')
    // Only party tokens can go, and only to a local map.
    expect(panel.findAll('[data-testid^="goes-"]').map((c) => c.attributes('data-testid'))).toEqual(['goes-Aria', 'goes-Brom'])
    expect(panel.findAll('[data-testid="group-map"] option').map((o) => o.text())).toEqual(['Choose a map', 'Crypt', 'Tower'])
    const go = panel.get('[data-testid="group-split"]')
    expect((go.element as HTMLButtonElement).disabled).toBe(true)
    await panel.get('[data-testid="group-name"]').setValue('  The tower ')
    await panel.get('[data-testid="goes-Aria"]').setValue(true)
    expect((go.element as HTMLButtonElement).disabled).toBe(true)
    await panel.get('[data-testid="group-map"]').setValue(TOWER)
    expect((go.element as HTMLButtonElement).disabled).toBe(false)
    await expectAccessible(wrapper.element as Element)
    await panel.get('[data-testid="group-form"]').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'split_party', name: 'The tower', tokenIds: [aria.id], mapId: TOWER, q: 0, r: 0 })
    expect((panel.get('[data-testid="group-name"]').element as HTMLInputElement).value).toBe('')
  })

  it('shows the DM every group, has the Table Display follow one, and brings a group back', async () => {
    const { wrapper, s } = await open('dm')
    const groups = [group(SID, '', { home: true, here: true, table: true, tokens: ['Brom'] }), group(AWAY, 'The tower', { number: 2, tokens: ['Aria'] })]
    s.receive(snapshot(SID, 'dm', { tokens: [brom, goblin], groups }))
    await flushPromises()
    const panel = wrapper.get('[data-testid="groups"]')
    const rows = panel.findAll('[data-testid^="group-0"]')
    expect(rows).toHaveLength(2)
    for (const piece of ['The party', 'Brom', 'You are here']) expect(rows[0]?.text()).toContain(piece)
    for (const piece of ['The tower', 'Aria', 'Go to this group', 'Bring back']) expect(rows[1]?.text()).toContain(piece)
    expect(rows[0]?.text()).not.toContain('Bring back')
    expect(panel.get(`[data-testid="group-go-${AWAY}"]`).attributes('href')).toBe(`/campaigns/${ID}/sessions/${AWAY}`)
    const follows = (id: string) => (panel.get(`[data-testid="group-table-${id}"]`).element as HTMLInputElement).checked
    expect([follows(SID), follows(AWAY)]).toEqual([true, false])
    expect(panel.get('[data-testid="groups-table"]').text()).toContain('opens for you, and for the Players of that group')
    await expectAccessible(wrapper.element as Element)

    await panel.get(`[data-testid="group-table-${AWAY}"]`).setValue(true)
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_follow', sessionId: AWAY })
    await panel.get(`[data-testid="group-table-${SID}"]`).setValue(true)
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_follow' })
    expect((s.sent.at(-1) as Record<string, unknown>).sessionId).toBeUndefined()
    // The group comes back beside the party tokens that stayed.
    await panel.get(`[data-testid="group-back-${AWAY}"]`).trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'rejoin_party', sessionId: AWAY, q: 1, r: 0 })
    // A party that is split keeps no Checkpoints.
    expect(wrapper.get('[data-testid="checkpoints-split"]').text()).toContain('Bring the party back together')
    expect(wrapper.find('[data-testid="checkpoint-form"]').exists()).toBe(false)
  })

  it('shows a group that left where the party is, and leaves bringing it back to the party\'s Session', async () => {
    const { wrapper, s } = await open('dm', `/campaigns/${ID}/sessions/${AWAY}`)
    const groups = [group(SID, '', { home: true, table: true, tokens: ['Brom'] }), group(AWAY, 'The tower', { number: 2, here: true, tokens: ['Aria'] })]
    s.receive(snapshot(AWAY, 'dm', { tokens: [aria], groups }))
    await flushPromises()
    const panel = wrapper.get('[data-testid="groups"]')
    expect(panel.get(`[data-testid="group-go-${SID}"]`).attributes('href')).toBe(`/campaigns/${ID}/sessions/${SID}`)
    expect(panel.find('[data-testid^="group-back-"]').exists()).toBe(false)
    expect(panel.find('[data-testid^="group-table-"]').exists()).toBe(false)
    expect(panel.find('[data-testid="group-form"]').exists()).toBe(false)
    expect(panel.get('[data-testid="groups-away"]').text()).toContain('from the party')
  })

  it('takes a player to their own group when the party splits, and shows them no groups', async () => {
    const { wrapper, router, s } = await open('player')
    s.receive(snapshot(SID, 'party'))
    await flushPromises()
    expect(wrapper.find('[data-testid="groups"]').exists()).toBe(false)
    s.receive({ kind: 'regroup', seq: 2, group: group(AWAY, '', {}) })
    s.drop()
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe(`/campaigns/${ID}/sessions/${AWAY}`)
    // The page is the other Session's now: a socket of its own, and nothing of the Session it left.
    const next = FakeSocket.last()
    expect(next).not.toBe(s)
    expect(next.url).toContain(`/sessions/${AWAY}/live?audience=party`)
    next.receive(snapshot(AWAY, 'party', { tokens: [aria] }))
    await flushPromises()
    expect(wrapper.get('[data-testid="tokens"]').text()).toContain('Aria')
    expect(wrapper.get('[data-testid="tokens"]').text()).not.toContain('Goblin')
  })

  it('takes the Table Display to the group it is to follow', async () => {
    const { router, s } = await open('player', `/campaigns/${ID}/sessions/${SID}/table`)
    s.receive(snapshot(SID, 'table'))
    await flushPromises()
    s.receive({ kind: 'regroup', seq: 2, group: group(AWAY, '', {}) })
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe(`/campaigns/${ID}/sessions/${AWAY}/table`)
    expect(FakeSocket.last().url).toContain(`/sessions/${AWAY}/live?audience=table`)
  })
})
