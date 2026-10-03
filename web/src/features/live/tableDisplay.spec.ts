import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { fakeClock, mountApp } from '@/test/mountApp'
import { BANNER_MS } from './motion'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = { id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: 'dm', memberCount: 2, createdAt: '2026-09-30T20:00:00Z', me: dm, members: [dm, player] }
const aria: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id, hp: 12, hpMax: 12 }
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0, health: 'unhurt' }
const table = (extra: object = {}) => ({ camera: 'follow_turn', q: 0, r: 0, zoomPct: 100, scene: 'local', blackout: false, ...extra })
const fighter = (t: LiveToken, acting: boolean) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`,
  initiative: 10, rank: 1, acting, done: false, action: true, bonusAction: true, reaction: true, movementFt: 30, speedFt: 30,
})
const view = (extra: object = {}) => ({ tokens: [aria, goblin], fog: false, visible: [], remembered: [], table: table(), ...extra })
const session = { id: SID, number: 3, gridRadius: 2, audience: 'table' }
const athletics = { roller: 'Aria', purpose: 'Athletics', dice: [{ faces: 20, value: 14, kept: true }, { faces: 20, value: 3, kept: false }], modifier: 3, total: 17 }

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('table display', () => {
  it('shows the DM\'s caption, the last roll with its dice, and whose turn it is', async () => {
    fakeClock()
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, {})
    const s = FakeSocket.last()
    s.receive({ kind: 'snapshot', seq: 1, view: view(), session })
    await flushPromises()
    for (const id of ['table-caption', 'table-roll', 'table-turn']) expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)

    s.receive({ kind: 'view', seq: 2, view: view({ table: table({ caption: 'The gate creaks open.' }) }) })
    await flushPromises()
    expect(wrapper.get('[data-testid="table-caption"]').text()).toBe('The gate creaks open.')

    // The last roll: who, what for, every die (the dropped one marked), the modifier and the total.
    s.receive({ kind: 'roll', seq: 2, roll: athletics })
    await flushPromises()
    const roll = wrapper.get('[data-testid="table-roll"]')
    expect(roll.text()).toBe('Aria · Athletics143+ 3= 17')
    expect(roll.findAll('.die').map((d) => [d.attributes('aria-label'), d.classes().includes('die--dropped')])).toEqual([['d20: 14', false], ['d20: 3, dropped', true]])
    s.receive({ kind: 'roll', seq: 2, roll: { roller: 'Aria', purpose: 'Longsword damage', dice: [{ faces: 8, value: 6, kept: true }], modifier: 0, total: 6 } })
    await flushPromises()
    expect(wrapper.get('[data-testid="table-roll"]').text()).toBe('Aria · Longsword damage6= 6')
    s.receive({ kind: 'roll', seq: 2, roll: { roller: 'Aria', purpose: 'Bane', dice: [{ faces: 4, value: 2, kept: true }], modifier: -1, total: 1 } })
    await flushPromises()
    expect(wrapper.get('[data-testid="table-roll"]').text()).toBe('Aria · Bane2− 1= 1')

    // Whose turn it is, as each turn starts; then it leaves.
    const fight = { status: 'active', round: 1, combatants: [fighter(goblin, true), fighter(aria, false)] }
    s.receive({ kind: 'view', seq: 3, view: view({ combat: fight }), turn: { round: 1, tokenIds: [goblin.id] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="table-turn"]').text()).toBe("Goblin's turn")
    await vi.advanceTimersByTimeAsync(BANNER_MS)
    expect(wrapper.find('[data-testid="table-turn"]').exists()).toBe(false)
    s.receive({ kind: 'view', seq: 4, view: view({ combat: { ...fight, combatants: [fighter(goblin, true), fighter(aria, true)] } }), turn: { round: 2, tokenIds: [aria.id, goblin.id, '0190c7a8-0000-7000-8000-0000000000ff'] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="table-turn"]').text()).toBe("Aria and Goblin's turn")
    vi.useRealTimers()
    await expectAccessible(wrapper.element as Element)

    // In the dark nothing shows; a late screen gets the last roll with its snapshot.
    s.receive({ kind: 'view', seq: 5, view: view({ table: table({ caption: 'The gate creaks open.', blackout: true }) }) })
    await flushPromises()
    for (const id of ['table-caption', 'table-roll', 'table-turn']) expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    const late = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, {})
    FakeSocket.last().receive({ kind: 'snapshot', seq: 9, view: view(), session, roll: athletics })
    await flushPromises()
    expect(late.wrapper.get('[data-testid="table-roll"]').text()).toContain('Athletics')
  })

  it('lets the DM put a caption on the table and take it off', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign })
    const s = FakeSocket.last()
    s.receive({ kind: 'snapshot', seq: 1, view: view({ table: table({ caption: 'Old words' }) }), session: { ...session, audience: 'dm' } })
    await flushPromises()
    const text = wrapper.get('[data-testid="table-caption-text"]')
    expect((text.element as HTMLInputElement).value).toBe('Old words')
    await text.setValue('The gate creaks open.')
    await wrapper.get('[data-testid="table-caption-form"]').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_caption', caption: 'The gate creaks open.' })
    await wrapper.get('[data-testid="clear-caption"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_caption', caption: '' })
    expect((text.element as HTMLInputElement).value).toBe('')
  })
})
