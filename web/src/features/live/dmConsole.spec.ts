import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
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
const scimitar = { name: 'Scimitar', toHit: 4, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d6', damageBonus: 2, damageType: 'slashing' }
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 60, ac: 15, hp: 5, hpMax: 7, attacks: [scimitar] }
const hob: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000c', label: 'Hob', q: 2, r: -1, ac: 18, hp: 11, hpMax: 11 }
const crate: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000d', label: 'Crate', kind: 'object', q: -1, r: 0, hidden: false, darkvisionFt: 0 }
const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id, ac: 16, hp: 12, hpMax: 12 }
const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
  rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
  reaction: true, movementFt: 30, speedFt: 30, ...extra,
})
const suggestion = { attackNo: 0, targetId: aria.id, reason: 'Simple: Aria is the nearest enemy, 5 ft away.' }
const view = (forDM: boolean, acting: LiveToken = goblin) => ({
  tokens: [aria, goblin, hob, crate], fog: false, visible: [], remembered: [],
  combat: {
    status: 'active', round: 1,
    combatants: [goblin, hob, aria].map((t) => fighter(t, { acting: t.id === acting.id, ...(forDM && t.kind === 'enemy' ? { tactics: 'auto' } : {}), ...(forDM && t.id === acting.id && t.kind === 'enemy' ? { suggestion } : {}) })),
  },
})
const at = '2026-10-03T20:00:00Z'
const log = [
  { seq: 9, kind: 'hp_adjusted', actor: 'Joris', origin: 'mcp', client: 'Claude', label: 'Goblin', tokenId: goblin.id, undoable: true, createdAt: at },
  { seq: 8, kind: 'hp_adjusted', actor: 'Joris', origin: 'ui', label: 'Goblin', tokenId: goblin.id, undoable: true, createdAt: at },
  { seq: 7, kind: 'effect_applied', actor: 'Joris', origin: 'mcp', client: 'Claude', label: 'Hob', tokenId: hob.id, undoable: false, createdAt: at },
  { seq: 6, kind: 'combat_started', actor: 'Joris', origin: 'mcp', client: 'Claude', label: '', undoable: false, createdAt: at },
]
async function open(as: 'dm' | 'player') {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/sessions/${SID}/log`]: () => log,
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: view(as === 'dm'), session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('DM console', () => {
  it('follows the acting creature, switches to another, and shows each one\'s panel with Tactics, its Suggested Action and what agents did to it', async () => {
    const { wrapper, s } = await open('dm')
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    const pressed = (label: string) => wrapper.get(`[data-testid="control-${label}"]`).attributes('aria-pressed')
    // The creatures the DM runs: not a player's Character, not a crate.
    expect(wrapper.findAll('[data-testid="control-switcher"] [data-testid^="control-"]').map((b) => b.attributes('data-testid'))).toEqual(['control-Goblin', 'control-Hob', 'control-several'])
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['true', 'false'])
    expect(wrapper.get('[data-testid="control-Goblin"]').text()).toContain('acting')

    const panel = wrapper.get('[data-testid="creature-Goblin"]')
    expect(panel.text()).toContain('5 / 7 hit points')
    expect(panel.text()).toContain('AC 15')
    expect(panel.get('[data-testid="creature-suggestion"]').text()).toBe('Suggested: Scimitar against Aria. Simple: Aria is the nearest enemy, 5 ft away.')
    expect((panel.get('[data-testid="creature-tactics"]').element as HTMLSelectElement).value).toBe('auto')
    await panel.get('[data-testid="creature-tactics"]').setValue('cunning')
    expect(sent()).toMatchObject({ kind: 'set_tactics', tokenId: goblin.id, tactics: 'cunning' })
    // Agent notes: what an agent did to this creature, with Undo while it can be undone. A DM's own change is not one.
    const notes = panel.get('[data-testid="agent-notes"]')
    expect(notes.findAll('li').map((li) => li.text())).toEqual(['#9 hp adjusted · via ClaudeUndo'])
    await notes.get('[data-testid="note-undo-9"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'undo', seq: 9 })

    // The Suggested Action is written under the token, and in what the hex says.
    expect(wrapper.get(`[data-testid="caption-${goblin.id}"]`).text()).toBe('Scimitar → Aria')
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('suggested: Scimitar against Aria')
    expect(wrapper.find(`[data-testid="caption-${hob.id}"]`).exists()).toBe(false)
    expect(wrapper.find('[data-testid="hotbar-Goblin"]').exists()).toBe(true)
    await expectAccessible(wrapper.element as Element)

    // Switching shows the other creature; the one acting keeps no hotbar while another is in hand.
    await wrapper.get('[data-testid="control-Hob"]').trigger('click')
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['false', 'true'])
    expect(wrapper.find('[data-testid="creature-Goblin"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="hotbar-Goblin"]').exists()).toBe(false)
    const other = wrapper.get('[data-testid="creature-Hob"]')
    expect(other.find('[data-testid="creature-suggestion"]').exists()).toBe(false)
    expect(other.get('[data-testid="agent-notes"]').findAll('li').map((li) => li.text())).toEqual(['#7 effect applied · via Claude'])

    // When the turn passes, the console follows it.
    s.receive({ kind: 'view', seq: 2, view: view(true, hob) })
    await flushPromises()
    expect(pressed('Hob')).toBe('true')
    s.receive({ kind: 'view', seq: 3, view: view(true, goblin) })
    await flushPromises()
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['true', 'false'])
    // A creature with nobody's notes says so; on a player's turn the console keeps what it had.
    s.receive({ kind: 'view', seq: 4, view: view(true, aria) })
    await flushPromises()
    expect(pressed('Goblin')).toBe('true')
    expect(wrapper.find(`[data-testid="caption-${goblin.id}"]`).exists()).toBe(false)
  })

  it('takes several creatures in hand to set their Tactics or change their hit points together', async () => {
    const { wrapper, s } = await open('dm')
    const pressed = (label: string) => wrapper.get(`[data-testid="control-${label}"]`).attributes('aria-pressed')
    expect(wrapper.find('[data-testid="bulk"]').exists()).toBe(false)
    await wrapper.get('[data-testid="control-several"]').setValue(true)
    await wrapper.get('[data-testid="control-Hob"]').trigger('click')
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['true', 'true'])
    expect(wrapper.findAll('[data-testid^="creature-G"], [data-testid^="creature-H"]')).toHaveLength(2)
    const before = s.sent.length
    await wrapper.get('[data-testid="bulk-tactics"]').setValue('off')
    expect(s.sent.slice(before)).toMatchObject([{ kind: 'set_tactics', tokenId: goblin.id, tactics: 'off' }, { kind: 'set_tactics', tokenId: hob.id, tactics: 'off' }])
    await wrapper.get('[data-testid="bulk-hp"]').setValue(3)
    await wrapper.get('[data-testid="bulk-hurt"]').trigger('click')
    expect(s.sent.slice(-2)).toMatchObject([{ kind: 'adjust_hp', tokenId: goblin.id, hpDelta: -3 }, { kind: 'adjust_hp', tokenId: hob.id, hpDelta: -3 }])
    await wrapper.get('[data-testid="bulk-heal"]').trigger('click')
    expect(s.sent.slice(-2)).toMatchObject([{ kind: 'adjust_hp', tokenId: goblin.id, hpDelta: 3 }, { kind: 'adjust_hp', tokenId: hob.id, hpDelta: 3 }])
    // Tapping a creature in hand lets it go; the last one stays. With several in hand the turn does not take over.
    await wrapper.get('[data-testid="control-Goblin"]').trigger('click')
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['false', 'true'])
    await wrapper.get('[data-testid="control-Hob"]').trigger('click')
    expect(pressed('Hob')).toBe('true')
    s.receive({ kind: 'view', seq: 2, view: view(true, goblin) })
    await flushPromises()
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['false', 'true'])
    await wrapper.get('[data-testid="control-Goblin"]').trigger('click')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="control-several"]').setValue(false)
    expect([pressed('Goblin'), pressed('Hob')]).toEqual(['true', 'false'])
    expect(wrapper.find('[data-testid="bulk"]').exists()).toBe(false)
  })

  it('shows a player none of it', async () => {
    const { wrapper, calls } = await open('player')
    expect(wrapper.find('[data-testid="control-switcher"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid^="creature-"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid^="caption-"]').exists()).toBe(false)
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).not.toContain('suggested')
    expect(calls.some((u) => u.pathname.endsWith('/log'))).toBe(false)
  })
})
