import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { board, hexes, initials } from './board'
import { cellsFor, key, layoutOf } from './geometry'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const member = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (myRole = 'dm') => {
  const me = myRole === 'dm' ? member : { ...player, isMe: true }
  return { id: ID, name: 'Strahd', ruleset: 'srd-2024', myRole, memberCount: 2, createdAt: '2026-09-30T20:00:00Z', me, members: [member, player] }
}
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin Boss', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0 }
const lurker: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000c', label: 'Lurker', kind: 'enemy', q: -1, r: 0, hidden: true, darkvisionFt: 60 }
const snapshot = (tokens: unknown[], audience = 'dm', extra: object = {}) => ({
  kind: 'snapshot', seq: 1, view: { tokens, fog: false, visible: [], remembered: [], ...extra }, session: { id: SID, number: 3, gridRadius: 2, audience },
})
const MID = '0190c7a8-0000-7000-8000-00000000000d'
const liveMap = { id: MID, name: 'Crypt', imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image?v=0`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, imageVersion: 0 }
const localMap = { id: MID, name: 'Crypt', imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, ambient: 'dark' }

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})
afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('board', () => {
  it('lays out hexes and paints tokens', () => {
    expect(hexes(2)).toHaveLength(19)
    expect(initials('Goblin  Boss Prime')).toBe('GB')
    const cells = board(1, [goblin, { ...lurker, q: 0, r: 0 }, { ...goblin, id: 'p', kind: 'party', q: 0, r: 1, label: 'Ireena' }], goblin.id)
    expect(cells.find((c) => c.q === 1 && c.r === 0)).toMatchObject({ tone: 'selected', mark: 'GB' })
    expect(cells.find((c) => c.q === 0 && c.r === 0)).toMatchObject({ tone: 'hidden', label: 'Lurker (hidden)' })
    expect(cells.find((c) => c.q === 0 && c.r === 1)).toMatchObject({ tone: 'ally' })
  })
})

describe('live session page', () => {
  it('lets the DM place, select, move, hide and remove tokens', async () => {
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/sessions/${SID}/end`]: () => ({ id: SID, number: 3, status: 'ended', seq: 4, gridRadius: 2, startedAt: '2026-09-30T20:00:00Z', endedAt: '2026-09-30T21:00:00Z' }),
      [`/api/v1/campaigns/${ID}/sessions`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    expect(s.url).toContain('audience=dm')
    s.open()
    s.receive(snapshot([goblin, lurker]))
    await flushPromises()
    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Live')
    expect(wrapper.get('h1').text()).toBe('Session 3')
    await wrapper.get('[data-testid="token-label"]').setValue('Ireena')
    await wrapper.get('[data-testid="token-kind"]').setValue('party')
    await wrapper.get('[data-testid="token-controller"]').setValue(player.id)
    await wrapper.get('[data-testid="token-hidden"]').setValue(true)
    await wrapper.get('[data-testid="token-darkvision"]').setValue(60)
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', label: 'Ireena', tokenKind: 'party', q: 0, r: 1, hidden: true, darkvisionFt: 60, controllerId: player.id })
    await wrapper.get('[data-hex="0,-1"]').trigger('click')
    expect(s.sent).toHaveLength(1)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(wrapper.get('[data-testid="selected-token"]').text()).toContain('Goblin Boss')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', tokenId: goblin.id, q: 2, r: 0 })
    s.receive({ kind: 'path', seq: 1, path: { tokenId: goblin.id, hexes: [{ q: 1, r: 0 }, { q: 2, r: 0 }], costFt: 5 } })
    await flushPromises()
    expect(wrapper.get('[data-testid="walk-preview"]').text()).toContain('Walk 5 ft')
    expect(wrapper.get('[data-hex="2,0"]').attributes('aria-label')).toContain('on the path')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'walk', tokenId: goblin.id, q: 2, r: 0 })
    await wrapper.get('[data-testid="toggle-hidden"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_token_hidden', tokenId: goblin.id, hidden: true })
    await wrapper.get('[data-hex="-1,0"]').trigger('click')
    expect(wrapper.get('[data-testid="toggle-hidden"]').text()).toBe('Reveal')
    await wrapper.get('[data-hex="-1,0"]').trigger('click')
    expect(wrapper.find('[data-testid="selected-token"]').exists()).toBe(false)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    await wrapper.get('[data-testid="remove-token"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'remove_token', tokenId: goblin.id })
    s.receive({ kind: 'rejected', seq: 1, reason: 'Only the DM can change tokens.' })
    await flushPromises()
    expect(wrapper.get('[data-testid="rejection"]').text()).toContain('Only the DM')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="end-session"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('campaign') }, { timeout: 5000 })
  })

  it('shows players the party view without controls', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Connecting…')
    const s = FakeSocket.last()
    expect(s.url).toContain('audience=party')
    s.receive(snapshot([goblin], 'party'))
    await flushPromises()
    expect(wrapper.find('[data-testid="dm-controls"]').exists()).toBe(false)
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(s.sent).toHaveLength(0)
    s.drop()
    await flushPromises()
    expect(wrapper.get('[data-testid="connection"]').text()).toBe('Reconnecting…')
  })

  it('refuses outsiders', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 404 }, 404),
    })
    expect(wrapper.find('[data-testid="live-missing"]').exists()).toBe(true)
  })
})

describe('live session on a map', () => {
  it('lets the DM choose a map and paint fog, walls, lights and ambient', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/maps`]: () => [localMap],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.open()
    s.receive(snapshot([], 'dm'))
    await flushPromises()
    await wrapper.get('[data-testid="use-map"]').trigger('click')
    expect(s.sent.at(-1)).toEqual({ kind: 'set_map', q: 0, r: 0, hidden: false, nonce: '1' })
    await wrapper.get('[data-testid="map-choice"]').setValue(MID)
    await wrapper.get('[data-testid="use-map"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_map', mapId: MID })
    const light = { id: '0190c7a8-0000-7000-8000-00000000000e', q: 1, r: 1, brightFt: 20, dimFt: 40 }
    s.receive({ kind: 'view', seq: 2, view: { tokens: [goblin], map: liveMap, fog: true, visible: [{ q: 0, r: 0 }], remembered: [{ q: 1, r: 0 }], walls: [{ q: 2, r: 0 }], lights: [light], ambient: 'dark' } })
    await flushPromises()
    expect(wrapper.get('[data-testid="map-board"] image').attributes('href')).toBe(liveMap.imageUrl)
    expect(wrapper.get('[data-hex="0,0"]').classes()).toContain('cell--lit')
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toBe('Hex 1, 0: remembered: Goblin Boss')
    expect(wrapper.get('[data-hex="2,0"]').classes()).toEqual(expect.arrayContaining(['cell--unseen', 'cell--wall', 'cell--dm']))
    expect(wrapper.get('[data-hex="1,1"]').attributes('aria-label')).toContain('light')
    expect(wrapper.get('[data-testid="ambient-dark"]').text()).toBe('dark ✓')
    const tap = async (tool: string, hex: string) => {
      await wrapper.get(`[data-testid="tool-${tool}"]`).setValue(true)
      await wrapper.get(`[data-hex="${hex}"]`).trigger('click')
    }
    await tap('reveal', '2,0')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'reveal_hexes', hexes: [{ q: 2, r: 0 }], on: true })
    await tap('conceal', '2,0')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'reveal_hexes', on: false })
    await tap('wall', '0,1')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_walls', hexes: [{ q: 0, r: 1 }], on: true })
    await tap('unwall', '2,0')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_walls', on: false })
    await tap('light', '1,1')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'remove_light', lightId: light.id })
    const [bright, dim] = wrapper.findAll('input[type="number"]')
    await bright?.setValue(10)
    await dim?.setValue(30)
    await wrapper.get('[data-hex="0,1"]').trigger('keydown', { key: 'Enter' })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_light', q: 0, r: 1, brightFt: 10, dimFt: 30 })
    await wrapper.get('[data-testid="ambient-dim"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_ambient', ambient: 'dim' })
    await tap('tokens', '1,0')
    expect(wrapper.get('[data-testid="selected-token"]').text()).toContain('Goblin Boss')
    await expectAccessible(wrapper.element as Element)
  })

  it('shows players only what the party sees', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([{ ...goblin, kind: 'npc' }, { ...lurker, kind: 'object', hidden: false, q: 0, r: 0 }], 'party', { map: liveMap, fog: true, visible: [{ q: 0, r: 0 }, { q: 1, r: 0 }], remembered: [] }))
    await flushPromises()
    expect(wrapper.get('[data-hex="2,0"]').classes()).toEqual(expect.arrayContaining(['cell--unseen']))
    expect(wrapper.get('[data-hex="2,0"]').classes()).not.toContain('cell--dm')
    expect(wrapper.get('[data-hex="2,0"]').attributes('aria-label')).toBe('Hex 2, 0: never seen')
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(s.sent).toHaveLength(0)
  })
})

describe('exploration', () => {
  it('lets a player preview and walk their own tokens, played back hex by hex', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id }
    const brom: LiveToken = { ...aria, id: '0190c7a8-0000-7000-8000-00000000000f', label: 'Brom', q: 0, r: 1 }
    s.receive(snapshot([aria, brom, goblin], 'party'))
    await flushPromises()
    expect(wrapper.get('[data-testid="walker"]').text()).toBe('Tap a hex to walk Aria there.')
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent).toHaveLength(0)
    expect(wrapper.get('[data-testid="walker"]').text()).toContain('Brom')
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', tokenId: brom.id, q: 1, r: 0 })
    s.receive({ kind: 'path', seq: 1, path: { tokenId: brom.id, hexes: [{ q: 0, r: 1 }, { q: 1, r: 0 }], costFt: 5 } })
    await flushPromises()
    await wrapper.get('[data-hex="2,-1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', q: 2, r: -1 })
    s.receive({ kind: 'rejected', seq: 1, reason: 'There is no way there.' })
    await flushPromises()
    expect(wrapper.find('[data-testid="walk-preview"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="rejection"]').text()).toBe('There is no way there.')
    s.receive({ kind: 'path', seq: 1, path: { tokenId: brom.id, hexes: [{ q: 0, r: 1 }, { q: 1, r: 1 }, { q: 2, r: 0 }], costFt: 10 } })
    await flushPromises()
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'walk', tokenId: brom.id, q: 2, r: 0 })
    const at = (q: number, r: number) => ({ tokens: [aria, { ...brom, q, r }], fog: false, visible: [], remembered: [] })
    s.receive({ kind: 'view', seq: 2, steps: [at(1, 1)], view: at(2, 0) })
    await flushPromises()
    expect(wrapper.get('[data-hex="1,1"]').attributes('aria-label')).toContain('Brom')
    await vi.advanceTimersByTimeAsync(250)
    expect(wrapper.get('[data-hex="2,0"]').attributes('aria-label')).toContain('Brom')
    expect(wrapper.find('[data-testid="walk-preview"]').exists()).toBe(false)
    vi.useRealTimers()
  })
})

describe('combat', () => {
  const fighter = (label: string, extra: Record<string, unknown> = {}) => ({
    id: `0190c7a8-0000-7000-8000-0000000001${label.length.toString().padStart(2, '0')}`, tokenId: goblin.id, label, kind: 'enemy',
    rollId: `0190c7a8-0000-7000-8000-0000000002${label.length.toString().padStart(2, '0')}`, acting: false, done: false,
    action: true, bonusAction: true, reaction: true, movementFt: 30, speedFt: 30, ...extra,
  })
  const initiativeRoll = (id: string) => ({
    id, purpose: 'Initiative for Goblin Boss', notation: '1d20', requestedBy: 'Joris', roller: { id: member.id, name: 'Joris' }, mine: true, canRoll: true,
    status: 'pending', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }], dice: [{ no: 0, group: 0, faces: 20, kept: false }],
    modifiers: [{ label: 'Initiative', value: 2 }], createdAt: '2026-09-30T20:00:00Z',
  })

  it('lets the DM start a fight, roll for the monsters and run turns', async () => {
    const rolled: string[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u, req) => {
        const id = u.pathname.split('/')[6] ?? ''
        if (req.method === 'POST') rolled.push(id)
        if (req.method === 'POST' && rolled.length === 1) return jsonResponse({ type: 'about:blank', title: 'x', status: 500 }, 500)
        if (rolled.length === 3) return { ...initiativeRoll(id), dice: [{ no: 0, group: 0, faces: 20, kept: false, value: 14 }] }
        return initiativeRoll(id)
      },
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const chest: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000f', label: 'Chest', kind: 'object', q: 2, r: 0 }
    s.receive(snapshot([goblin, chest, lurker]))
    await flushPromises()
    await wrapper.get('[data-testid="choose-combatants"]').trigger('click')
    expect(wrapper.get('[data-testid="fights-Chest"]').element).toHaveProperty('checked', false)
    await wrapper.get('[data-testid="fights-Chest"]').setValue(true)
    await wrapper.get('[data-testid="fights-Chest"]').setValue(false)
    await wrapper.get('[data-testid="bonus-Goblin Boss"]').setValue(2)
    await wrapper.get('[data-testid="speed-Lurker"]').setValue(40)
    await wrapper.get('[data-testid="fights-Goblin Boss"]').setValue(false)
    await wrapper.get('[data-testid="fights-Lurker"]').setValue(false)
    await wrapper.get('[data-testid="start-combat"]').trigger('submit')
    expect(s.sent).toHaveLength(0)
    await wrapper.get('[data-testid="fights-Goblin Boss"]').setValue(true)
    await wrapper.get('[data-testid="fights-Lurker"]').setValue(true)
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="start-combat"]').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({
      kind: 'start_combat',
      combatants: [{ tokenId: goblin.id, initiativeBonus: 2, speedFt: 30 }, { tokenId: lurker.id, initiativeBonus: 0, speedFt: 40 }],
    })
    expect(wrapper.find('[data-testid="start-combat"]').exists()).toBe(false)
    const rolling = { status: 'rolling', round: 0, combatants: [fighter('Goblin Boss'), fighter('Lurker'), fighter('Aria', { kind: 'party', controllerId: player.id })] }
    s.receive({ kind: 'view', seq: 2, view: { tokens: [goblin, lurker], fog: false, visible: [], remembered: [], combat: rolling } })
    await flushPromises()
    expect(wrapper.get('[data-testid="initiative-rail"]').text()).toContain('Rolling initiative')
    expect(wrapper.get('[data-testid="rail-Lurker"]').text()).toContain('rolling…')
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(2)
    await wrapper.get('[data-testid="roll-all"]').trigger('click')
    await flushPromises()
    expect(rolled).toHaveLength(2)
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    await wrapper.get('[data-testid="auto-0"]').trigger('click')
    await vi.waitFor(() => { expect(wrapper.get('[data-testid="die-0"]').text()).toContain('14') }, { timeout: 5000 })
    const active = {
      status: 'active', round: 1,
      combatants: [
        fighter('Goblin Boss', { initiative: 17, rank: 1, acting: true, action: false, movementFt: 10 }),
        fighter('Aria', { kind: 'party', controllerId: player.id, initiative: 17, rank: 1, acting: true }),
        fighter('Lurker', { initiative: 5, rank: 3, done: true }),
      ],
    }
    s.receive({ kind: 'view', seq: 3, view: { tokens: [goblin, lurker], fog: false, visible: [], remembered: [], combat: active } })
    await flushPromises()
    expect(wrapper.get('[data-testid="initiative-rail"]').text()).toContain('Round 1')
    expect(wrapper.get('[data-testid="rail-Goblin Boss"]').text()).toContain('17 · tied')
    expect(wrapper.get('[data-testid="rail-Goblin Boss"]').attributes('aria-current')).toBe('step')
    expect(wrapper.get('[data-testid="rail-Lurker"]').text()).not.toContain('tied')
    expect(wrapper.findAll('[data-testid^="turn-"]')).toHaveLength(2)
    const boss = wrapper.get('[data-testid="turn-Goblin Boss"]')
    expect(boss.get('[data-testid="movement"]').text()).toBe('10 / 30 ft')
    expect(boss.get('[data-testid="spend-action"]').attributes('disabled')).toBeDefined()
    await boss.get('[data-testid="spend-bonus_action"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'spend', combatantId: fighter('Goblin Boss').id, resource: 'bonus_action' })
    await boss.get('[data-testid="end-turn"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'end_turn', combatantId: fighter('Goblin Boss').id })
    expect(wrapper.find('[data-testid="your-turn"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="end-combat"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'end_combat' })
  })

  it('shows a player their turn, their initiative roll and the rail on the table', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => initiativeRoll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    const rolling = { status: 'rolling', round: 0, combatants: [fighter('Aria', { kind: 'party', controllerId: player.id }), fighter('Goblin Boss')] }
    s.receive(snapshot([goblin], 'party', { combat: rolling }))
    await flushPromises()
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="roll-all"]').exists()).toBe(false)
    const active = { status: 'active', round: 2, combatants: [fighter('Aria', { kind: 'party', controllerId: player.id, initiative: 12, rank: 1, acting: true }), fighter('Goblin Boss', { initiative: 3, rank: 2 })] }
    s.receive({ kind: 'view', seq: 2, view: { tokens: [goblin], fog: false, visible: [], remembered: [], combat: active } })
    await flushPromises()
    expect(wrapper.get('[data-testid="your-turn"]').text()).toBe('Your turn')
    expect(wrapper.findAll('[data-testid^="turn-"]')).toHaveLength(1)
    await wrapper.get('[data-testid="spend-reaction"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'spend', resource: 'reaction' })

    const table = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, {})
    FakeSocket.last().receive(snapshot([goblin], 'table', { combat: active }))
    await flushPromises()
    expect(table.wrapper.get('[data-testid="initiative-rail"]').text()).toContain('Round 2')
    expect(table.wrapper.findAll('[data-testid="table-display"] polygon')).toHaveLength(19)
  })
})

describe('map geometry', () => {
  it('lists the hexes whose centres fall inside the picture', () => {
    const l = layoutOf(localMap)
    const cells = cellsFor(l, localMap.width, localMap.height).map(key)
    expect(cells).toContain('0,0')
    expect(cells).not.toContain('-1,0')
    expect(cellsFor(l, 1, 1)).toEqual([])
  })
})

describe('maps pages', () => {
  it('uploads a map and calibrates its grid', async () => {
    const writes: string[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/maps`, {
      [`/api/v1/campaigns/${ID}/maps/${MID}`]: async (_u, req) => {
        if (req.method === 'PUT') writes.push(`PUT ${JSON.stringify(await req.json())}`)
        return localMap
      },
      [`/api/v1/campaigns/${ID}/maps`]: async (u, req) => {
        if (req.method === 'POST') {
          writes.push(`POST ${String(u.searchParams.get('name'))} ${String((await req.arrayBuffer()).byteLength)}`)
          return localMap
        }
        return [localMap]
      },
    })
    expect(wrapper.get('[data-testid="map-list"]').text()).toContain('Crypt')
    expect(wrapper.get('[data-testid="map-list"]').text()).toContain('200 × 160 px')
    const form = wrapper.get('[data-testid="map-upload"]')
    expect(form.get('button').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="map-name"]').setValue('Crypt')
    const input = wrapper.get('[data-testid="map-file"]')
    Object.defineProperty(input.element, 'files', { value: [new File([new Uint8Array(5)], 'crypt.png', { type: 'image/png' })], configurable: true })
    await input.trigger('change')
    await expectAccessible(wrapper.element as Element)
    await form.trigger('submit')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('map') }, { timeout: 5000 })
    expect(writes).toEqual(['POST Crypt 5'])
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Crypt')
    expect(wrapper.findAll('[data-testid="map-board"] g').length).toBeGreaterThan(0)
    await wrapper.get('[data-testid="hex-size"]').setValue(50)
    await wrapper.get('[data-testid="origin-x"]').setValue(20)
    const numbers = wrapper.findAll('[data-testid="map-calibrate"] input')
    await numbers[0]?.setValue('Crypt of Night')
    await numbers[3]?.setValue(25)
    await wrapper.get('[data-testid="map-calibrate"] select').setValue('dim')
    await wrapper.get('[data-testid="map-calibrate"]').trigger('submit')
    await flushPromises()
    expect(writes[1]).toBe('PUT {"name":"Crypt of Night","hexSizePx":50,"originX":20,"originY":25,"ambient":"dim"}')
    expect(wrapper.get('[data-testid="calibration-saved"]').text()).toBe('Saved.')
    await expectAccessible(wrapper.element as Element)
  })

  it('explains failed uploads and saves, and refuses players', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/maps`, {
      [`/api/v1/campaigns/${ID}/maps`]: (_u, req) => (req.method === 'POST' ? jsonResponse({ type: 'about:blank', title: 'x', status: 422 }, 422) : []),
    })
    await wrapper.get('[data-testid="map-name"]').setValue('Crypt')
    const input = wrapper.get('[data-testid="map-file"]')
    Object.defineProperty(input.element, 'files', { value: [new File([new Uint8Array(5)], 'x.gif')], configurable: true })
    await input.trigger('change')
    await wrapper.get('[data-testid="map-upload"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="map-upload"] [role="alert"]').text()).toContain('could not be uploaded')
    Object.defineProperty(input.element, 'files', { value: [], configurable: true })
    await input.trigger('change')
    await wrapper.get('[data-testid="map-upload"]').trigger('submit')
    await flushPromises()

    const denied = await mountApp(`/campaigns/${ID}/maps`, {
      [`/api/v1/campaigns/${ID}/maps`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 403 }, 403),
    })
    expect(denied.wrapper.find('[data-testid="maps-refused"]').exists()).toBe(true)

    const broken = await mountApp(`/campaigns/${ID}/maps/${MID}`, {
      [`/api/v1/campaigns/${ID}/maps/${MID}`]: (_u, req) => (req.method === 'PUT' ? jsonResponse({ type: 'about:blank', title: 'x', status: 422 }, 422) : localMap),
    })
    await broken.wrapper.get('[data-testid="map-calibrate"]').trigger('submit')
    await flushPromises()
    expect(broken.wrapper.get('[data-testid="map-calibrate"] [role="alert"]').text()).toContain('could not be saved')
    const missing = await mountApp(`/campaigns/${ID}/maps/${MID}`, {})
    expect(missing.wrapper.find('[data-testid="map-missing"]').exists()).toBe(true)
  })
})

describe('table display', () => {
  it('renders the party view full-screen and follows the session to its end', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, {})
    expect(wrapper.find('header').exists()).toBe(false)
    expect(wrapper.find('footer').exists()).toBe(false)
    expect(wrapper.text()).toContain('Waiting for the table')
    const s = FakeSocket.last()
    expect(s.url).toContain('audience=table')
    s.receive(snapshot([goblin], 'table'))
    await flushPromises()
    expect(wrapper.findAll('[data-testid="table-display"] polygon')).toHaveLength(19)
    await expectAccessible(wrapper.element as Element)
    s.receive({ kind: 'view', seq: 2, view: { tokens: [], map: liveMap, fog: true, visible: [], remembered: [{ q: 0, r: 0 }] } })
    await flushPromises()
    expect(wrapper.get('[data-hex="0,0"]').classes()).toContain('cell--remembered')
    s.receive({ kind: 'ended', seq: 1 })
    await flushPromises()
    expect(wrapper.text()).toContain('The session has ended')
  })
})
