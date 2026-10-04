import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { still, vibrating } from '@/test/haptics'
import { mountApp, unmountAll } from '@/test/mountApp'
import { rollLine, turnLine } from './announce'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = (myRole: string) => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: myRole === 'dm' ? { ...dm, isMe: true } : player, members: [dm, player],
})
const aria: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', controllerId: player.id, q: 0, r: 0, hidden: false, darkvisionFt: 0 }
const wolf: LiveToken = { ...aria, id: '0190c7a8-0000-7000-8000-00000000000f', label: 'Wolf', q: 0, r: 1 }
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin Boss', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0 }
const UNSEEN = '0190c7a8-0000-7000-8000-00000000000c'
const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
  rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
  reaction: true, movementFt: 30, speedFt: 30, ...extra,
})
const combat = (round: number, acting: string[]) => ({ status: 'active', round, combatants: [goblin, aria, wolf].map((t, i) => fighter(t, { rank: i + 1, acting: acting.includes(t.id) })) })
const view = (round: number, acting: string[]) => ({ tokens: [aria, wolf, goblin], fog: false, visible: [], remembered: [], combat: combat(round, acting) })
const RID = '0190c7a8-0000-7000-8000-0000000000d1'
const athletics = { id: RID, roller: 'Aria', purpose: 'Athletics', dice: [{ faces: 20, value: 14, kept: true }, { faces: 20, value: 3, kept: false }], modifier: 3, total: 17 }

async function open(role: string) {
  const buzzed = vibrating()
  const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign(role) })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: view(1, [goblin.id]), session: { id: SID, number: 3, gridRadius: 2, audience: role === 'dm' ? 'dm' : 'party' }, roll: athletics })
  await flushPromises()
  let seq = 1
  const turn = async (round: number, tokenIds: string[]) => {
    s.receive({ kind: 'view', seq: ++seq, view: view(round, tokenIds), turn: { round, tokenIds } })
    await flushPromises()
  }
  const roll = async (r: object) => {
    s.receive({ kind: 'roll', seq, roll: r })
    await flushPromises()
  }
  const said = () => wrapper.findAll('[data-testid="announcer"] p').map((line) => line.text())
  return { wrapper, turn, roll, said, buzzed }
}

beforeEach(() => {
  localStorage.clear()
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})
afterEach(() => {
  unmountAll()
  still()
  vi.unstubAllGlobals()
  localStorage.clear()
})

describe('what a screen reader is told in live play', () => {
  it('says whose turn starts and what was rolled, in a log a screen reader reads as it grows', async () => {
    const { wrapper, turn, roll, said } = await open('player')
    const log = wrapper.get('[data-testid="announcer"]')
    expect(log.attributes()).toMatchObject({ role: 'log', 'aria-live': 'polite', 'aria-label': 'What is happening' })
    expect(log.classes()).toContain('sr-only')
    // The roll that was already on the table when the page opened is old news.
    expect(said()).toEqual([])

    await turn(1, [goblin.id])
    await turn(1, [aria.id])
    await roll(athletics)
    await roll({ ...athletics, id: RID.replace('d1', 'd2'), purpose: 'Longsword attack against Goblin Boss', dice: [{ faces: 20, value: 20, kept: true }], modifier: 5, total: 25 })
    await turn(1, [UNSEEN])
    await turn(2, [aria.id, wolf.id])
    expect(said()).toEqual([
      'Round 1. Goblin Boss acts.',
      'Round 1. Your turn: Aria.',
      'Aria rolled 17 for Athletics.',
      'Aria rolled 25 for Longsword attack against Goblin Boss, a natural 20.',
      'Round 1. A creature acts.',
      'Round 2. Your turn: Aria and Wolf.',
    ])
    // The words on screen for a turn are for the eye: the log says them once.
    expect(wrapper.get('[data-testid="your-turn"]').attributes('role')).toBeUndefined()
    await expectAccessible(wrapper.element as Element)
  })

  it('keeps only the last few lines', async () => {
    const { turn, said } = await open('player')
    for (let round = 1; round <= 8; round++) await turn(round, [goblin.id])
    expect(said()).toEqual([3, 4, 5, 6, 7, 8].map((round) => `Round ${String(round)}. Goblin Boss acts.`))
  })

  it('tells the DM whose turn it is by name, since every creature is theirs to run', async () => {
    const { turn, said } = await open('dm')
    await turn(1, [goblin.id])
    await turn(1, [aria.id, wolf.id])
    expect(said()).toEqual(['Round 1. Goblin Boss acts.', 'Round 1. Aria and Wolf act.'])
  })

  it('leaves out turns or rolls when they are turned off', async () => {
    localStorage.setItem('grimoire.accessibility', JSON.stringify({ announceTurns: false }))
    const quiet = await open('player')
    await quiet.turn(1, [aria.id])
    await quiet.roll(athletics)
    expect(quiet.said()).toEqual(['Aria rolled 17 for Athletics.'])
    unmountAll()
    localStorage.setItem('grimoire.accessibility', JSON.stringify({ announceRolls: false }))
    const mute = await open('player')
    await mute.turn(1, [aria.id])
    await mute.roll(athletics)
    expect(mute.said()).toEqual(['Round 1. Your turn: Aria.'])
  })

  it('buzzes as my turn starts, and not for anybody else\'s', async () => {
    const { turn, buzzed } = await open('player')
    await turn(1, [goblin.id])
    expect(buzzed).toEqual([])
    await turn(1, [aria.id])
    expect(buzzed).toEqual([[120, 60, 120]])
    unmountAll()
    localStorage.setItem('grimoire.accessibility', JSON.stringify({ haptics: false }))
    const off = await open('player')
    await off.turn(1, [aria.id])
    expect(off.buzzed).toEqual([])
    unmountAll()
    // The DM runs every turn, so none of them is news.
    localStorage.clear()
    const master = await open('dm')
    await master.turn(1, [aria.id])
    expect(master.buzzed).toEqual([])
  })
})

describe('the lines', () => {
  it('name a turn and a roll', () => {
    expect(turnLine(3, ['Goblin'], [])).toBe('Round 3. Goblin acts.')
    expect(turnLine(3, ['Goblin', 'Wolf', 'Bat'], [])).toBe('Round 3. Goblin, Wolf and Bat act.')
    expect(turnLine(3, ['Aria', 'Goblin'], ['Aria'])).toBe('Round 3. Your turn: Aria.')
    expect(turnLine(3, [], [])).toBe('Round 3. A creature acts.')
    expect(rollLine({ ...athletics, dice: [{ faces: 20, value: 1, kept: true }], total: 4 })).toBe('Aria rolled 4 for Athletics, a natural 1.')
    // A dropped 20 is no critical.
    expect(rollLine({ ...athletics, dice: [{ faces: 20, value: 20, kept: false }, { faces: 20, value: 9, kept: true }], total: 12 })).toBe('Aria rolled 12 for Athletics.')
  })
})
