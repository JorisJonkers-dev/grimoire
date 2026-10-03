import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
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
const goblin = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 60, ac: 15, hp: 5, hpMax: 7, attacks: [] }
const checkpoint = (n: number, name: string, kind: 'named' | 'round', round = 0) => ({
  id: `0190c7a8-0000-7000-8000-0000000000c${String(n)}`, name, kind, round, actionSeq: 40 + n, at: `2026-10-03T20:0${String(n)}:00Z`,
})
const before = checkpoint(1, 'Before the ambush', 'named')
const round = checkpoint(2, 'Round 2', 'round', 2)
const log = [{ seq: 9, kind: 'hp_adjusted', actor: 'Joris', origin: 'ui', label: 'Goblin', tokenId: goblin.id, undoable: true, createdAt: '2026-10-03T20:00:00Z' }]
const view = (extra: Record<string, unknown> = {}) => ({ tokens: [goblin], fog: false, visible: [], remembered: [], roster: [], gameDay: 0, ...extra })

async function open(as: 'dm' | 'player', v: Record<string, unknown>) {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/sessions/${SID}/log`]: () => log,
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: view(v), session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('checkpoints', () => {
  it('lets the DM keep a named checkpoint, and rewind to one only after saying so twice', async () => {
    const { wrapper, s } = await open('dm', { checkpoints: [before, round] })
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    const panel = wrapper.get('[data-testid="checkpoints"]')
    // The newest first; a round's own checkpoint says what it is.
    expect(panel.findAll('li[data-testid^="checkpoint-"]').map((li) => li.text().replace(/\s+/g, ' '))).toEqual([
      'Round 2 Start of the roundRewind here', 'Before the ambushRewind here',
    ])
    expect(panel.get('[data-testid="checkpoints-scope"]').text()).toContain('Items, coins and rests already finished stay as they are')

    // A name is needed, and is sent trimmed.
    const keep = panel.get('[data-testid="checkpoint-keep"]')
    expect((keep.element as HTMLButtonElement).disabled).toBe(true)
    await panel.get('[data-testid="checkpoint-name"]').setValue('  The bridge  ')
    await panel.get('[data-testid="checkpoint-form"]').trigger('submit')
    expect(sent()).toMatchObject({ kind: 'checkpoint', name: 'The bridge' })
    expect((panel.get('[data-testid="checkpoint-name"]').element as HTMLInputElement).value).toBe('')

    // Rewinding asks first: it takes back everything since.
    const sends = s.sent.length
    await panel.get(`[data-testid="rewind-${before.id}"]`).trigger('click')
    expect(s.sent).toHaveLength(sends)
    const sure = panel.get('[data-testid="rewind-confirm"]')
    expect(sure.text()).toContain('Rewind to Before the ambush? Everything done since is taken back.')
    await expectAccessible(wrapper.element as Element)
    await sure.get('[data-testid="rewind-no"]').trigger('click')
    expect(panel.find('[data-testid="rewind-confirm"]').exists()).toBe(false)
    expect(s.sent).toHaveLength(sends)
    await panel.get(`[data-testid="rewind-${round.id}"]`).trigger('click')
    await panel.get('[data-testid="rewind-yes"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'rewind', checkpointId: round.id })
    expect(panel.find('[data-testid="rewind-confirm"]').exists()).toBe(false)

    // The rewound Session arrives as a fresh snapshot: the checkpoint after it is gone from the list.
    s.receive({ kind: 'snapshot', seq: 5, view: view({ checkpoints: [before] }), session: { id: SID, number: 3, gridRadius: 2, audience: 'dm' } })
    await flushPromises()
    expect(panel.findAll('li[data-testid^="checkpoint-"]')).toHaveLength(1)
    // Undo is on offer beside it, as ever.
    expect(wrapper.find('[data-testid="undo-9"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="undo-damage"]').exists()).toBe(true)
  })

  it('says so when there is nothing to rewind to yet', async () => {
    const { wrapper } = await open('dm', {})
    expect(wrapper.get('[data-testid="checkpoints-none"]').text()).toContain('None yet')
    expect(wrapper.find('[data-testid^="rewind-"]').exists()).toBe(false)
  })

  it('offers no undo, checkpoint or rewind in a Campaign played without undo', async () => {
    const { wrapper } = await open('dm', { noUndo: true })
    expect(wrapper.get('[data-testid="no-undo"]').text()).toBe('This Campaign is played without undo: nothing is taken back.')
    expect(wrapper.find('[data-testid="checkpoint-form"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="undo-9"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="undo-damage"]').exists()).toBe(false)
    // The log itself still shows what happened.
    expect(wrapper.get('[data-testid="log-9"]').text()).toContain('hp adjusted')
    await expectAccessible(wrapper.element as Element)
  })

  it('shows a player none of it', async () => {
    const { wrapper } = await open('player', {})
    expect(wrapper.find('[data-testid="checkpoints"]').exists()).toBe(false)
  })
})
