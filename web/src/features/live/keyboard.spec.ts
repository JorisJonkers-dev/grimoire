import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp, unmountAll } from '@/test/mountApp'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = { id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: 'player', memberCount: 2, createdAt: '2026-09-30T20:00:00Z', me: player, members: [dm, player] }
const aria: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', controllerId: player.id, q: -1, r: 0, hidden: false, darkvisionFt: 0 }
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin Boss', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0 }
const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
  rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
  reaction: true, movementFt: 30, speedFt: 30, ...extra,
})
const key = (init: KeyboardEventInit, on: EventTarget = document.body) => {
  const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })
  on.dispatchEvent(e)
  return e
}
const hex = () => document.activeElement?.getAttribute('data-hex')

async function open(extra: object = {}) {
  const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign })
  const s = FakeSocket.last()
  const combat = { status: 'active', round: 1, combatants: [fighter(aria, { acting: true }), fighter(goblin, { rank: 2 })] }
  s.receive({ kind: 'snapshot', seq: 1, view: { tokens: [aria, goblin], fog: false, visible: [], remembered: [], combat, ...extra }, session: { id: SID, number: 3, gridRadius: 2, audience: 'party' } })
  await flushPromises()
  return { wrapper, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})
afterEach(() => {
  unmountAll()
  vi.unstubAllGlobals()
})

describe('playing a turn from the keyboard', () => {
  it('finds my Character on the map with M, walks the hexes with the arrows and chooses one with Enter', async () => {
    const { wrapper, s } = await open()
    expect(wrapper.get('[data-hex="-1,0"]').attributes('aria-keyshortcuts')).toBe('M')
    for (const other of ['0,0', '1,0']) expect(wrapper.get(`[data-hex="${other}"]`).attributes('aria-keyshortcuts')).toBeUndefined()
    expect(key({ key: 'm' }).defaultPrevented).toBe(true)
    expect(hex()).toBe('-1,0')
    // M only finds the hex: nothing is chosen or walked until Enter.
    expect(s.sent).toEqual([])
    expect(key({ key: 'ArrowRight' }, document.activeElement ?? document.body).defaultPrevented).toBe(true)
    expect(hex()).toBe('0,0')
    key({ key: 'ArrowRight' }, document.activeElement ?? document.body)
    expect(hex()).toBe('1,0')
    key({ key: 'ArrowLeft' }, document.activeElement ?? document.body)
    key({ key: 'ArrowUp' }, document.activeElement ?? document.body)
    expect(hex()).toBe('0,-1')
    await wrapper.get('[data-hex="0,-1"]').trigger('keydown.enter')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', tokenId: aria.id, q: 0, r: -1 })
    // At the edge of the map an arrow that leads nowhere is left to the browser.
    ;(wrapper.get('[data-hex="2,0"]').element as HTMLElement).focus()
    expect(key({ key: 'ArrowRight' }, document.activeElement ?? document.body).defaultPrevented).toBe(false)
    expect(hex()).toBe('2,0')
  })

  it('finds my Character on a drawn map as well', async () => {
    const MID = '0190c7a8-0000-7000-8000-00000000000d'
    const map = { id: MID, name: 'Crypt', imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image?v=0`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, imageVersion: 0, gridKind: 'hexes', gridStrength: 20 }
    // The picture's hexes start at its corner, so the two stand on it here.
    const { wrapper } = await open({ map, tokens: [{ ...aria, q: 1, r: 0 }, { ...goblin, q: 0, r: 0 }] })
    expect(wrapper.find('[data-testid="map-board"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-hex][aria-keyshortcuts]').map((c) => c.attributes('data-hex'))).toEqual(['1,0'])
    key({ key: 'm' })
    expect(hex()).toBe('1,0')
  })

  it('ends my turn with E, and not while I am typing', async () => {
    const { wrapper, s } = await open()
    const end = wrapper.get('[data-testid="end-turn"]')
    expect(end.attributes('aria-keyshortcuts')).toBe('E')
    const field = document.createElement('input')
    document.body.append(field)
    key({ key: 'e' }, field)
    expect(s.sent).toEqual([])
    key({ key: 'e' })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'end_turn' })
    // The keys list says so.
    key({ key: '?', shiftKey: true })
    await flushPromises()
    const rows = wrapper.findAll('[data-testid="key-row"]').map((r) => `${r.get('dt').text()} ${r.get('dd').text()}`)
    expect(rows).toContain('E End turn')
    expect(rows.some((r) => r.startsWith('M '))).toBe(true)
  })
})

describe('the controls a turn is played with', () => {
  // Each names its key where it is drawn, which is what pressing the key and the list of keys go by.
  it.each([
    ['live/TurnPanel.vue', 'end-turn', 'E'],
    ['live/WalkPlan.vue', 'confirm-walk', 'C'],
    ['live/WalkPlan.vue', 'cancel-walk', 'Escape'],
    ['live/AttackPreview.vue', 'confirm-attack', 'C'],
    ['live/AttackPreview.vue', 'cancel-attack', 'Escape'],
    ['live/AreaPreviewCard.vue', 'confirm-area', 'C'],
    ['live/AreaPreviewCard.vue', 'cancel-area', 'Escape'],
    ['live/ReactionPrompt.vue', 'use-reaction', 'Y'],
    ['live/ReactionPrompt.vue', 'decline-reaction', 'N'],
    ['rolls/RollCard.vue', 'roll-rest', 'R'],
    ['rolls/RollCard.vue', 'keep-roll', 'K'],
  ])('%s: %s answers to %s', (file, testId, keyName) => {
    const source = readFileSync(resolve(__dirname, '..', file), 'utf8')
    const line = source.split('\n').find((l) => l.includes(`data-testid="${testId}"`)) ?? ''
    expect(line).toContain(`aria-keyshortcuts="${keyName}"`)
  })
})
