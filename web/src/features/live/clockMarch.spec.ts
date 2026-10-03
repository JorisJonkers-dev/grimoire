import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'
import { clockText, nextDawn } from './travel'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const ARIA = '0190c7a8-0000-7000-8000-000000000031'
const BROM = '0190c7a8-0000-7000-8000-000000000032'
const CYRA = '0190c7a8-0000-7000-8000-000000000033'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const order = [{ characterId: BROM, name: 'Brom', place: 1 }, { characterId: ARIA, name: 'Aria', place: 2 }, { characterId: CYRA, name: 'Cyra' }]
const view = (extra: Record<string, unknown> = {}) => ({ tokens: [], fog: false, visible: [], remembered: [], gameDay: 3, gameMinute: 14 * 60 + 5, marchingOrder: order, ...extra })

async function open(as: 'dm' | 'player') {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/maps`]: () => [],
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: view(), session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  await mounted.wrapper.get('[data-testid="scope-world"]').setValue(true)
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('the Game Clock', () => {
  it('says the day and the time, and when the next dawn is', () => {
    expect(clockText(3, 14 * 60 + 5)).toBe('Day 3, 14:05')
    expect(clockText(0, 0)).toBe('Day 0, 00:00')
    expect(clockText(12, 23 * 60 + 59)).toBe('Day 12, 23:59')
    expect(nextDawn(3, 14 * 60)).toEqual({ gameDay: 4, gameMinute: 360 })
    expect(nextDawn(3, 5 * 60 + 59)).toEqual({ gameDay: 3, gameMinute: 360 })
    expect(nextDawn(3, 6 * 60)).toEqual({ gameDay: 4, gameMinute: 360 })
  })

  it('shows a player the time, and nothing to change it with', async () => {
    const { wrapper } = await open('player')
    expect(wrapper.get('[data-testid="game-clock"]').text()).toBe('Day 3, 14:05')
    expect(wrapper.find('[data-testid="clock-tools"]').exists()).toBe(false)
  })

  it('lets the DM move the clock on or set it', async () => {
    const { wrapper, s } = await open('dm')
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    expect(wrapper.get('[data-testid="game-clock"]').text()).toBe('Day 3, 14:05')
    await wrapper.get('[data-testid="clock-hour"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'set_clock', gameDay: 3, gameMinute: 15 * 60 + 5 })
    await wrapper.get('[data-testid="clock-dawn"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'set_clock', gameDay: 4, gameMinute: 360 })
    // An hour before midnight becomes the next day.
    s.receive({ kind: 'view', seq: 2, view: view({ gameDay: 3, gameMinute: 23 * 60 + 30 }) })
    await flushPromises()
    expect(wrapper.get('[data-testid="game-clock"]').text()).toBe('Day 3, 23:30')
    await wrapper.get('[data-testid="clock-hour"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'set_clock', gameDay: 4, gameMinute: 30 })
    // The day to set starts from the day it is.
    s.receive({ kind: 'view', seq: 3, view: view({ gameDay: 5, gameMinute: 30 }) })
    await flushPromises()
    expect((wrapper.get('[data-testid="clock-day"]').element as HTMLInputElement).value).toBe('5')
    await wrapper.get('[data-testid="clock-day"]').setValue(9)
    await wrapper.get('[data-testid="clock-time"]').setValue('07:45')
    await wrapper.get('[data-testid="clock-set"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'set_clock', gameDay: 9, gameMinute: 7 * 60 + 45 })
    await expectAccessible(wrapper.element as Element)
  })
})

describe('the Marching Order', () => {
  it('shows the order to everyone and lets anyone rearrange it', async () => {
    const { wrapper, s } = await open('player')
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    const rows = () => wrapper.get('[data-testid="marching-order"]').findAll('li').map((li) => li.get('[data-testid="marcher"]').text())
    expect(rows()).toEqual(['1. Brom', '2. Aria', 'Cyra, not placed'])
    // The front cannot move up, nor the last of all down.
    expect(wrapper.get(`[data-testid="march-up-${BROM}"]`).attributes('disabled')).toBeDefined()
    expect(wrapper.get(`[data-testid="march-down-${CYRA}"]`).attributes('disabled')).toBeDefined()
    await wrapper.get(`[data-testid="march-up-${ARIA}"]`).trigger('click')
    expect(sent()).toEqual(expect.objectContaining({ kind: 'set_marching_order', characterIds: [ARIA, BROM, CYRA] }))
    await wrapper.get(`[data-testid="march-down-${BROM}"]`).trigger('click')
    expect(sent()).toEqual(expect.objectContaining({ kind: 'set_marching_order', characterIds: [ARIA, BROM, CYRA] }))
    await wrapper.get(`[data-testid="march-up-${CYRA}"]`).trigger('click')
    expect(sent()).toEqual(expect.objectContaining({ kind: 'set_marching_order', characterIds: [BROM, CYRA, ARIA] }))
    s.receive({ kind: 'view', seq: 2, view: view({ marchingOrder: [] }) })
    await flushPromises()
    expect(wrapper.get('[data-testid="marching-order"]').text()).toContain('No Characters in this Campaign yet.')
    await expectAccessible(wrapper.element as Element)
  })
})
