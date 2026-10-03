import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveCheck, LiveContainer, LiveShop, LiveTable, LiveToken, LiveWorld, LiveZone } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { fakeClock, mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { board, hexes, initials, zoneHexes } from './board'
import { focus } from './camera'
import { checkLine } from './checks'
import { canPut, canTake, instanceLabel, load } from './inventory'
import { cellsFor, key, layoutOf } from './geometry'
import { BANNER_MS, REVEAL_FADE_MS, REVEAL_HOLD_MS } from './motion'
import { duration, journey } from './travel'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const member = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (myRole = 'dm') => {
  const me = myRole === 'dm' ? member : { ...player, isMe: true }
  return { id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole, memberCount: 2, createdAt: '2026-09-30T20:00:00Z', me, members: [member, player] }
}
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin Boss', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0 }
const lurker: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000c', label: 'Lurker', kind: 'enemy', q: -1, r: 0, hidden: true, darkvisionFt: 60 }
const snapshot = (tokens: unknown[], audience = 'dm', extra: object = {}) => ({
  kind: 'snapshot', seq: 1, view: { tokens, fog: false, visible: [], remembered: [], ...extra }, session: { id: SID, number: 3, gridRadius: 2, audience },
})
const MID = '0190c7a8-0000-7000-8000-00000000000d'
const liveMap = { id: MID, name: 'Crypt', imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image?v=0`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, imageVersion: 0, gridKind: 'hexes' as const, gridStrength: 20 }
const localMap = { id: MID, name: 'Crypt', imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, ambient: 'dark', kind: 'local' as const, gridKind: 'hexes' as const, gridStrength: 20, scaleMiles: 6, found: false }
const WID = '0190c7a8-0000-7000-8000-000000000020'
const realmMap = { ...localMap, id: WID, name: 'Realm', kind: 'world' as const }
const OAK = '0190c7a8-0000-7000-8000-000000000022'
const MILL = '0190c7a8-0000-7000-8000-000000000023'
const ROAD = '0190c7a8-0000-7000-8000-000000000024'
const realm = (extra: Partial<LiveWorld> = {}): LiveWorld => ({
  map: { ...liveMap, id: WID, name: 'Realm' },
  found: false,
  revealed: [{ q: 0, r: 0 }, { q: 1, r: 0 }],
  nodes: [{ id: OAK, name: 'Oakford', q: 0, r: 0 }, { id: MILL, name: 'Mill', q: 2, r: 0 }],
  routes: [{
    id: ROAD, fromNodeId: OAK, toNodeId: MILL, distanceMi: 12,
    plans: [{ pace: 'slow', minutes: 360, days: 1 }, { pace: 'normal', minutes: 240, days: 1 }, { pace: 'fast', minutes: 180, days: 1 }],
  }],
  partyNodeId: OAK,
  legs: [{ from: 'Mill', to: 'Oakford', pace: 'normal', distanceMi: 12, minutes: 240, days: 1 }, { from: 'Oakford', to: 'Mill', pace: 'slow', distanceMi: 30, minutes: 900, days: 2 }],
  ...extra,
})

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
    const cells = board(1, [goblin, { ...lurker, q: 0, r: 0 }, { ...goblin, id: 'p', kind: 'party', q: 0, r: 1, label: 'Tamsin' }], goblin.id)
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
    await wrapper.get('[data-testid="token-label"]').setValue('Tamsin')
    await wrapper.get('[data-testid="token-kind"]').setValue('party')
    await wrapper.get('[data-testid="token-controller"]').setValue(player.id)
    await wrapper.get('[data-testid="token-hidden"]').setValue(true)
    await wrapper.get('[data-testid="token-darkvision"]').setValue(60)
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', label: 'Tamsin', tokenKind: 'party', q: 0, r: 1, hidden: true, darkvisionFt: 60, controllerId: player.id })
    await wrapper.get('[data-hex="0,-1"]').trigger('click')
    expect(s.sent).toHaveLength(1)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(wrapper.get('[data-testid="selected-token"]').text()).toContain('Goblin Boss')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', tokenId: goblin.id, q: 2, r: 0 })
    s.receive({ kind: 'path', seq: 1, path: { tokenId: goblin.id, hexes: [{ q: 1, r: 0 }, { q: 2, r: 0 }], costFt: 5, threats: [], sight: [] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="walk-preview"]').text()).toContain('Walk 5 ft')
    expect(wrapper.get('[data-hex="2,0"]').attributes('aria-label')).toContain('on the path')
    await wrapper.get('[data-testid="confirm-walk"]').trigger('click')
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
    expect(wrapper.get('[data-testid="map-image"]').attributes('href')).toBe(liveMap.imageUrl)
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

describe('leaving the session', () => {
  it('closes the socket and never reconnects once the page is gone', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([goblin], 'party'))
    await flushPromises()
    fakeClock()
    wrapper.unmount()
    expect(s.closed).toBe(true)
    s.drop()
    await vi.advanceTimersByTimeAsync(30_000)
    expect(FakeSocket.all).toHaveLength(1)
  })
})

describe('exploration', () => {
  it('lets a player preview and walk their own tokens, played back hex by hex', async () => {
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
    s.receive({ kind: 'path', seq: 1, path: { tokenId: brom.id, hexes: [{ q: 0, r: 1 }, { q: 1, r: 0 }], costFt: 5, threats: [], sight: [] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="walk-preview"]').text()).toContain('No opportunity attacks.')
    // Tapping the same hex again only plans again: nothing moves without Confirm.
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', q: 1, r: 0 })
    await wrapper.get('[data-testid="cancel-walk"]').trigger('click')
    expect(wrapper.find('[data-testid="walk-preview"]').exists()).toBe(false)
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).not.toContain('on the path')
    expect(s.sent.every((c) => (c as { kind: string }).kind !== 'walk')).toBe(true)
    await wrapper.get('[data-hex="2,-1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', q: 2, r: -1 })
    s.receive({ kind: 'rejected', seq: 1, reason: 'There is no way there.' })
    await flushPromises()
    expect(wrapper.find('[data-testid="walk-preview"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="rejection"]').text()).toBe('There is no way there.')
    s.receive({
      kind: 'path', seq: 1,
      path: {
        tokenId: brom.id, hexes: [{ q: 0, r: 1 }, { q: 1, r: 1 }, { q: 2, r: 0 }], costFt: 10,
        threats: [{ tokenId: goblin.id, label: 'Goblin Boss', q: 0, r: 1 }],
        sight: [
          { tokenId: goblin.id, label: 'Goblin Boss', visible: true, cover: 'half' },
          { tokenId: lurker.id, label: 'Archer', visible: false, cover: 'total' },
          { tokenId: '0190c7a8-0000-7000-8000-0000000000aa', label: 'Wolf', visible: true, cover: 'none' },
          { tokenId: '0190c7a8-0000-7000-8000-0000000000ab', label: 'Ogre', visible: true, cover: 'three_quarters' },
        ],
      },
    })
    await flushPromises()
    const plan = wrapper.get('[data-testid="walk-preview"]')
    expect(plan.text()).toContain('Walk 10 ft')
    expect(plan.get('[data-testid="walk-threats"]').text()).toBe('Leaving Goblin Boss\'s reach draws an opportunity attack.')
    expect(plan.findAll('[data-testid="walk-sight"] li').map((li) => li.text())).toEqual([
      'Goblin Boss sees Brom there, behind half cover.', 'Archer has no line to Brom there.', 'Wolf sees Brom there.', 'Ogre sees Brom there, behind three-quarters cover.',
    ])
    expect(wrapper.get('[data-hex="0,1"]').attributes('aria-label')).toContain('opportunity attack')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="confirm-walk"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'walk', tokenId: brom.id, q: 2, r: 0 })
    const at = (q: number, r: number) => ({ tokens: [aria, { ...brom, q, r }], fog: false, visible: [], remembered: [] })
    fakeClock()
    s.receive({ kind: 'view', seq: 2, steps: [at(1, 1)], view: at(2, 0) })
    await flushPromises()
    expect(wrapper.get('[data-hex="1,1"]').attributes('aria-label')).toContain('Brom')
    await vi.advanceTimersByTimeAsync(250)
    expect(wrapper.get('[data-hex="2,0"]').attributes('aria-label')).toContain('Brom')
    expect(wrapper.find('[data-testid="walk-preview"]').exists()).toBe(false)
  })
})

describe('combat', () => {
  const fighter = (label: string, extra: Record<string, unknown> = {}) => ({
    id: `0190c7a8-0000-7000-8000-0000000001${label.length.toString().padStart(2, '0')}`, tokenId: ({ Lurker: lurker.id } as Record<string, string>)[label] ?? goblin.id, label, kind: 'enemy',
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
    const tired = { ...goblin, effects: [{ id: '0190c7a8-0000-7000-8000-000000000301', slug: 'exhaustion', name: 'Exhaustion', concentration: false, level: 2 },
      { id: '0190c7a8-0000-7000-8000-000000000302', slug: 'homebrew-hex', name: 'Hex', concentration: false }] }
    s.receive({ kind: 'view', seq: 3, view: { tokens: [tired, lurker], fog: false, visible: [], remembered: [], combat: active } })
    await flushPromises()
    expect(wrapper.get('[data-testid="rail-Goblin Boss"] [data-testid="statuses"]').findAll('[role="img"]').map((i) => i.attributes('aria-label'))).toEqual(['Exhaustion 2', 'Hex'])
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

  it('reveals initiative before the faces slide into order, and raises a player\'s banner as their turn starts', async () => {
    const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', controllerId: player.id, q: 0, r: 0 }
    const ariaFights = (extra: Record<string, unknown> = {}) => fighter('Aria', { kind: 'party', controllerId: player.id, tokenId: aria.id, ...extra })
    const frame = (seq: number, combat: object, extra: object = {}) => ({ kind: 'view', seq, view: { tokens: [aria, goblin], fog: false, visible: [], remembered: [], combat }, ...extra })
    const order = [{ tokenId: goblin.id, label: 'Goblin Boss', kind: 'enemy', initiative: 18 }, { tokenId: aria.id, label: 'Aria', kind: 'party', initiative: 5 }]
    const goblinActs = { status: 'active', round: 1, combatants: [fighter('Goblin Boss', { initiative: 18, rank: 1, acting: true }), ariaFights({ initiative: 5, rank: 2 })] }
    const ariaActs = { status: 'active', round: 1, combatants: [fighter('Goblin Boss', { initiative: 18, rank: 1, done: true }), ariaFights({ initiative: 5, rank: 2, acting: true })] }
    const open = async (role: string, path = '') => {
      const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}${path}`, { [`/api/v1/campaigns/${ID}/rolls/`]: (u) => initiativeRoll(u.pathname.split('/')[6] ?? ''), [`/api/v1/campaigns/${ID}`]: () => campaign(role) })
      const s = FakeSocket.last()
      s.receive(snapshot([aria, goblin], path ? 'table' : role === 'dm' ? 'dm' : 'party', { combat: { status: 'rolling', round: 0, combatants: [ariaFights(), fighter('Goblin Boss')] } }))
      await flushPromises()
      const faces = () => wrapper.findAll('[data-testid="roster-strip"] li').map((li) => li.attributes('data-testid'))
      return { wrapper, s, faces }
    }
    fakeClock()
    const { wrapper, s, faces } = await open('player')
    s.receive(frame(2, goblinActs, { initiative: { order }, turn: { round: 1, tokenIds: [goblin.id] } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="roster-strip"]').classes()).toContain('roster--reveal')
    expect(wrapper.get('[data-testid="rolled-Aria"]').text()).toBe('5')
    expect(wrapper.get('[data-testid="rolled-Goblin Boss"]').text()).toBe('18')
    expect(faces()).toEqual(['rail-Aria', 'rail-Goblin Boss'])
    expect(wrapper.find('[data-testid="turn-banner"]').exists()).toBe(false)
    await vi.advanceTimersByTimeAsync(REVEAL_HOLD_MS)
    expect(faces()).toEqual(['rail-Goblin Boss', 'rail-Aria'])
    expect(wrapper.find('[data-testid="rolled-Aria"]').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(REVEAL_FADE_MS)
    expect(wrapper.find('[data-testid="rolled-Aria"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="roster-strip"]').classes()).not.toContain('roster--reveal')

    s.receive(frame(3, ariaActs, { turn: { round: 1, tokenIds: [aria.id] } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="turn-banner"]').text()).toBe("It's your turn Aria")
    s.receive(frame(4, ariaActs))
    await flushPromises()
    expect(wrapper.find('[data-testid="turn-banner"]').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(BANNER_MS)
    expect(wrapper.find('[data-testid="turn-banner"]').exists()).toBe(false)
    s.receive(frame(5, { ...ariaActs, round: 2 }, { turn: { round: 2, tokenIds: [aria.id] } }))
    await flushPromises()
    await wrapper.get('[data-testid="turn-banner"]').trigger('click')
    expect(wrapper.find('[data-testid="turn-banner"]').exists()).toBe(false)

    // A monster's turn is nobody's banner; the DM and the Table Display see the reveal too.
    const dm = await open('dm')
    dm.s.receive(frame(2, goblinActs, { initiative: { order }, turn: { round: 1, tokenIds: [goblin.id] } }))
    await flushPromises()
    expect(dm.wrapper.find('[data-testid="rolled-Aria"]').exists()).toBe(true)
    expect(dm.wrapper.find('[data-testid="turn-banner"]').exists()).toBe(false)
    const table = await open('player', '/table')
    table.s.receive(frame(2, goblinActs, { initiative: { order }, turn: { round: 1, tokenIds: [goblin.id] } }))
    await flushPromises()
    expect(table.wrapper.get('[data-testid="rolled-Goblin Boss"]').text()).toBe('18')

    // Reduced motion: straight into order, nothing to fade; the banner still says whose turn it is.
    unmountAll()
    document.body.innerHTML = ''
    vi.useRealTimers()
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const still = await open('player')
    still.s.receive(frame(2, goblinActs, { initiative: { order }, turn: { round: 1, tokenIds: [goblin.id] } }))
    await flushPromises()
    expect(still.faces()).toEqual(['rail-Goblin Boss', 'rail-Aria'])
    expect(still.wrapper.find('[data-testid="rolled-Aria"]').exists()).toBe(false)
    expect(still.wrapper.get('[data-testid="roster-strip"]').classes()).not.toContain('roster--reveal')
    still.s.receive(frame(3, ariaActs, { turn: { round: 1, tokenIds: [aria.id] } }))
    await flushPromises()
    expect(still.wrapper.get('[data-testid="turn-banner"]').text()).toContain('Aria')
    await expectAccessible(still.wrapper.element as Element)
    still.wrapper.unmount()
  })
})

describe('attacks', () => {
  const sword = { name: 'Scimitar', toHit: 4, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d6', damageBonus: 2, damageType: 'slashing' }
  const slam = { name: 'Slam', toHit: 4, reachFt: 0, rangeFt: 20, longRangeFt: 60, damageBonus: 3 }
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id, ac: 16, hp: 12, hpMax: 12, attacks: [sword] }
  const boss: LiveToken = { ...goblin, ac: 15, hp: 7, hpMax: 7, attacks: [sword, slam] }
  const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
    id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
    rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
    reaction: true, movementFt: 30, speedFt: 30, ...extra,
  })
  const pendingRoll = (id: string) => ({
    id, purpose: 'Scimitar attack against Aria', notation: '1d20', requestedBy: 'Joris', roller: { id: member.id, name: 'Joris' }, mine: true, canRoll: true,
    status: 'pending', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }], dice: [{ no: 0, group: 0, faces: 20, kept: false }],
    modifiers: [{ label: 'Scimitar', value: 4 }], createdAt: '2026-09-30T20:00:00Z',
  })
  const characterSummary = { id: '0190c7a8-0000-7000-8000-000000000031', name: 'Mira', ownerName: 'Aria', mine: false, species: 'human', class: 'fighter', level: 1, hpCurrent: 12, hpMax: 12 }

  it('lets the DM aim a creature, preview the odds, attack and undo', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => pendingRoll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}/characters`]: () => [characterSummary],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const active = (extra: Record<string, unknown> = {}) => ({ status: 'active', round: 1, combatants: [fighter(boss, { acting: true }), fighter(aria)], ...extra })
    s.receive(snapshot([aria, boss], 'dm', { combat: active() }))
    await flushPromises()
    expect(wrapper.text()).toContain('Aria (12/12 HP)')
    const bar = wrapper.get('[data-testid="hotbar-Goblin Boss"]')
    expect(bar.get('[data-testid="attack-0"]').text()).toContain('+4 · 1d6+2 · reach 5 ft')
    expect(bar.get('[data-testid="attack-1"]').text()).toContain('+4 · 3 · range 20/60 ft')
    await bar.get('[data-testid="action-dash"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'take_action', tokenId: boss.id, action: 'dash' })
    expect(bar.get('[data-testid="action-hide"]').attributes('title')).toContain('DC 15')
    await bar.get('[data-testid="ready"]').setValue('0')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'take_action', tokenId: boss.id, action: 'ready', trigger: 'enters_reach', attackNo: 0 })
    await bar.get('[data-testid="unarmed-grapple"]').trigger('click')
    expect(wrapper.get('[data-testid="grabbing"]').text()).toBe('Tap the creature to grapple or shove.')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'unarmed', tokenId: boss.id, targetId: aria.id, option: 'grapple' })
    expect(wrapper.find('[data-testid="grabbing"]').exists()).toBe(false)
    await bar.get('[data-testid="attack-0"]').trigger('click')
    await bar.get('[data-testid="attack-0"]').trigger('click')
    expect(wrapper.find('[data-testid="hotbar-Goblin Boss"] [role="status"]').exists()).toBe(false)
    await bar.get('[data-testid="attack-0"]').trigger('click')
    expect(bar.text()).toContain('Tap a creature to aim.')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'preview_attack', tokenId: boss.id, attackNo: 0, targetId: aria.id })
    const preview = { tokenId: boss.id, targetId: aria.id, attackNo: 0, name: 'Scimitar', hitChance: 45, mode: 'normal', damageMin: 3, damageMax: 8, critMax: 14, reasons: ['Scimitar: +4 to hit'] }
    s.receive({ kind: 'attack_preview', seq: 1, preview: { ...preview, attackNo: 1 } })
    await flushPromises()
    expect(wrapper.find('[data-testid="attack-preview"]').exists()).toBe(false)
    s.receive({ kind: 'attack_preview', seq: 1, preview })
    await flushPromises()
    const panel = wrapper.get('[data-testid="attack-preview"]')
    expect(panel.get('h2').text()).toBe('Scimitar against Aria')
    expect(panel.get('[data-testid="hit-chance"]').text()).toBe('45% to hit')
    expect(panel.get('[data-testid="damage-range"]').text()).toBe('3–8 damage')
    expect(panel.text()).toContain('critical up to 14')
    await expectAccessible(wrapper.element as Element)
    await panel.get('[data-testid="cancel-attack"]').trigger('click')
    expect(wrapper.find('[data-testid="attack-preview"]').exists()).toBe(false)
    await bar.get('[data-testid="attack-0"]').trigger('click')
    s.receive({ kind: 'attack_preview', seq: 1, preview: { ...preview, targetId: '0190c7a8-0000-7000-8000-000000000099' } })
    await flushPromises()
    expect(wrapper.get('[data-testid="attack-preview"] h2').text()).toBe('Scimitar against the target')
    s.receive({ kind: 'attack_preview', seq: 1, preview })
    await flushPromises()
    await wrapper.get('[data-testid="confirm-attack"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'attack', tokenId: boss.id, attackNo: 0, targetId: aria.id })
    const attack = { attackerId: boss.id, targetId: aria.id, name: 'Scimitar', stage: 'to_hit', rollId: '0190c7a8-0000-7000-8000-000000000041', critical: false }
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, boss], fog: false, visible: [], remembered: [], combat: active({ attack, combatants: [fighter(boss, { acting: true, action: false }), fighter(aria)] }) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="pending-attack"]').text()).toBe('Scimitar: waiting for the attack roll.')
    expect(wrapper.get('[data-testid="roll-card"]').text()).toContain('Scimitar attack against Aria')
    expect(wrapper.get('[data-testid="hotbar-blocked"]').text()).toBe('An attack is waiting on its roll.')
    s.receive({ kind: 'view', seq: 3, view: { tokens: [{ ...aria, hp: 5 }, boss], fog: false, visible: [], remembered: [], combat: active({ attack: { ...attack, stage: 'damage', critical: true }, combatants: [fighter(boss, { acting: true, action: false }), fighter(aria)] }) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="pending-attack"]').text()).toBe('Scimitar (critical): waiting for the damage roll.')
    expect(wrapper.get('[data-hex="0,0"]').attributes('aria-label')).toContain('Aria (5/12 HP)')
    s.receive({ kind: 'view', seq: 4, view: { tokens: [aria, boss], fog: false, visible: [], remembered: [], combat: active({ combatants: [fighter(boss, { acting: true, action: false }), fighter(aria)] }) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="hotbar-blocked"]').text()).toBe('The action is used this turn.')
    await wrapper.get('[data-testid="undo-damage"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'undo_damage' })
    await wrapper.get('[data-testid="token-monster"]').setValue('goblin')
    await wrapper.get('[data-hex="1,-1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', monsterSlug: 'goblin', label: '' })
    await wrapper.get('[data-testid="token-character"]').setValue(characterSummary.id)
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', characterId: characterSummary.id })
    expect(s.sent.at(-1)).not.toHaveProperty('monsterSlug')
  })

  it('offers the DM the suggested action of a creature and its tactics', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign() })
    const s = FakeSocket.last()
    const suggestion = { attackNo: 1, targetId: aria.id, reason: 'Cunning: it saw Aria deal 8 damage from range.' }
    const fight = (extra: Record<string, unknown>) => ({ status: 'active', round: 1, combatants: [fighter(boss, { acting: true, tactics: 'auto', ...extra }), fighter(aria)] })
    s.receive(snapshot([aria, boss], 'dm', { combat: fight({ suggestion }) }))
    await flushPromises()
    const hint = wrapper.get('[data-testid="suggestion"]')
    expect(hint.text()).toContain('Suggested: Slam against Aria. Cunning: it saw Aria deal 8 damage from range.')
    await hint.get('[data-testid="use-suggestion"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'attack', tokenId: boss.id, attackNo: 1, targetId: aria.id })
    await wrapper.get('[data-testid="tactics"]').setValue('off')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_tactics', tokenId: boss.id, tactics: 'off' })
    await expectAccessible(wrapper.element as Element)
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, boss], fog: false, visible: [], remembered: [], combat: fight({ suggestion: { targetId: '0190c7a8-0000-7000-8000-000000000098', reason: 'Simple: nothing reaches yet; close in on someone, 40 ft away.' } }) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="suggestion"]').text()).toContain('close in on someone')
    expect(wrapper.find('[data-testid="use-suggestion"]').exists()).toBe(false)
  })

  it('shows a player their hotbar, the roll for their attack and only the health of monsters', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => pendingRoll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    const orc: LiveToken = { ...goblin, label: 'Orc', health: 'bloodied' }
    const attack = { attackerId: aria.id, targetId: orc.id, name: 'Scimitar', stage: 'to_hit', rollId: '0190c7a8-0000-7000-8000-000000000042', critical: false }
    s.receive(snapshot([aria, orc], 'party', { combat: { status: 'active', round: 1, combatants: [fighter(aria, { acting: true }), fighter(orc)], attack } }))
    await flushPromises()
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('Orc (bloodied)')
    expect(wrapper.find('[data-testid="hotbar-Aria"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="roll-card"]').text()).toContain('Scimitar attack')
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, orc], fog: false, visible: [], remembered: [], combat: { status: 'active', round: 1, combatants: [fighter(aria), fighter(orc, { acting: true })], attack: { ...attack, attackerId: orc.id } } } })
    await flushPromises()
    expect(wrapper.find('[data-testid="roll-card"]').exists()).toBe(false)
    s.receive({ kind: 'view', seq: 3, view: { tokens: [aria], fog: false, visible: [], remembered: [], combat: { status: 'active', round: 1, combatants: [fighter(aria)], attack: { ...attack, attackerId: orc.id } } } })
    await flushPromises()
    expect(wrapper.find('[data-testid="roll-card"]').exists()).toBe(false)
  })
})

describe('reactions', () => {
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id, shield: true }
  const fighter = (t: LiveToken) => ({
    id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
    rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
    reaction: true, movementFt: 30, speedFt: 30,
  })
  const prompt = (extra: Record<string, unknown> = {}) => ({
    id: '0190c7a8-0000-7000-8000-000000000051', kind: 'shield', reactorId: aria.id, triggerId: goblin.id,
    effect: 'Shield: AC 16 → 21, so the attack (19) would miss.', secondsLeft: 3, ...extra,
  })
  const withPrompt = (p: unknown, stage = 'reaction') => ({
    combat: {
      status: 'active', round: 1, combatants: [fighter(aria), fighter(goblin)], prompt: p,
      attack: { attackerId: goblin.id, targetId: aria.id, name: 'Scimitar', stage, rollId: '0190c7a8-0000-7000-8000-000000000052', critical: false },
    },
  })

  it('asks the reactor to answer before the countdown runs out', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    fakeClock()
    s.receive(snapshot([aria, goblin], 'party', withPrompt(prompt())))
    await flushPromises()
    const card = wrapper.get('[data-testid="reaction-prompt"]')
    expect(card.get('h2').text()).toBe('Shield: Aria')
    expect(card.get('[data-testid="reaction-effect"]').text()).toContain('would miss')
    expect(wrapper.get('[data-testid="pending-attack"]').text()).toBe('Scimitar: waiting for a reaction.')
    expect(card.get('[data-testid="countdown"]').text()).toBe('3 s')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('[data-testid="countdown"]').text()).toBe('2 s')
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.get('[data-testid="countdown"]').text()).toBe('0 s')
    vi.useRealTimers()
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="use-reaction"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'react', use: true })
    await wrapper.get('[data-testid="decline-reaction"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'react', use: false })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, goblin], fog: false, visible: [], remembered: [], ...withPrompt(prompt({ id: '0190c7a8-0000-7000-8000-000000000053', kind: 'opportunity_attack', reactorId: goblin.id, secondsLeft: 9 })) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="reaction-prompt"] h2').text()).toBe('Opportunity attack: Goblin Boss')
    expect(wrapper.get('[data-testid="countdown"]').text()).toBe('9 s')
    expect(wrapper.find('[data-testid="use-reaction"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="reaction-prompt"]').text()).toContain("Waiting for Goblin Boss's reaction.")
    s.receive({ kind: 'view', seq: 3, view: { tokens: [aria], fog: false, visible: [], remembered: [], ...withPrompt(prompt({ reactorId: goblin.id }), 'to_hit') } })
    await flushPromises()
    expect(wrapper.get('[data-testid="reaction-prompt"] h2').text()).toBe('Shield: A creature')
    s.receive({ kind: 'view', seq: 4, view: { tokens: [aria, goblin], fog: false, visible: [], remembered: [] } })
    await flushPromises()
    expect(wrapper.find('[data-testid="reaction-prompt"]').exists()).toBe(false)
  })

  it('lets the DM answer any prompt and place a token that knows Shield', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([aria, goblin], 'dm', withPrompt(prompt())))
    await flushPromises()
    await wrapper.get('[data-testid="use-reaction"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'react', use: true })
    await wrapper.get('[data-testid="token-label"]').setValue('Mage')
    await wrapper.get('[data-testid="token-shield"]').setValue(true)
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', label: 'Mage', shield: true })
  })
})

describe('effects', () => {
  const aria: LiveToken = {
    ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id, hp: 12, hpMax: 12,
    effects: [{ id: '0190c7a8-0000-7000-8000-000000000061', slug: 'bless', name: 'Bless', sourceId: '0190c7a8-0000-7000-8000-00000000000e', concentration: true, roundsLeft: 9 }],
  }
  const saveRoll = (id: string) => ({
    id, purpose: 'Wisdom save to end Hold Person (DC 13)', notation: '1d20', requestedBy: 'Joris', roller: { id: member.id, name: 'Joris' }, mine: true,
    canRoll: true, status: 'pending', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }], dice: [{ no: 0, group: 0, faces: 20, kept: false }],
    modifiers: [], createdAt: '2026-09-30T20:00:00Z',
  })

  it('shows a homebrew condition with its own icon and offers the Campaign\'s conditions in the picker', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const frost = { id: '0190c7a8-0000-7000-8000-000000000071', slug: 'hb-frost', name: 'Frostbite', concentration: false, level: 2, icon: 'snow', color: '#7fa8dd' }
    s.receive(snapshot([{ ...aria, effects: [frost] }, goblin], 'dm', { conditions: [{ slug: 'hb-frost', name: 'Frostbite', icon: 'snow', color: '#7fa8dd' }] }))
    await flushPromises()
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    const panel = wrapper.get('[data-testid="effects-panel"]')
    const icon = panel.get('svg[aria-label="Frostbite 2"]')
    expect(icon.attributes('stroke')).toBe('#7fa8dd')
    expect(panel.findAll('#known-effects option').map((o) => o.attributes('value'))).toContain('hb-frost')
  })

  it('offers the DM a legendary creature\'s legendary and lair actions and its Legendary Resistance', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const legend = {
      uses: 3, left: 1, ready: true, resistLeft: 1, phase: 0, phases: 1, threshold: 5, lairReady: false,
      actions: [{ name: 'Tail Sweep', cost: 1, text: 'One Claw attack.' }, { name: 'Sink', cost: 2, text: 'It sinks.' }],
      lair: [{ name: 'Rising Water', cost: 0, text: 'The water rises.' }],
    }
    s.receive(snapshot([aria, { ...goblin, legend }], 'dm'))
    await flushPromises()
    await wrapper.get(`[data-hex="${String(goblin.q)},${String(goblin.r)}"]`).trigger('click')
    const panel = wrapper.get('[data-testid="legend-panel"]')
    expect(panel.get('[data-testid="legend-left"]').text()).toContain('1 of 3 legendary actions left · ready now')
    expect(panel.get('[data-testid="legend-phase"]').text()).toBe('Phase 1 of 2')
    expect(panel.get('[data-testid="legendary-Sink"]').attributes('disabled')).toBeDefined()
    expect(panel.get('[data-testid="lair-Rising Water"]').attributes('disabled')).toBeDefined()
    await panel.get('[data-testid="legendary-Tail Sweep"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'legendary_action', tokenId: goblin.id, legend: 'Tail Sweep' })
    await panel.get('[data-testid="legendary-resistance"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'legendary_resistance', tokenId: goblin.id })
    s.receive(snapshot([aria, { ...goblin, legend: { ...legend, ready: false, lairReady: true, resistLeft: 0 } }], 'dm'))
    await flushPromises()
    const again = wrapper.get('[data-testid="legend-panel"]')
    expect(again.get('[data-testid="legend-left"]').text()).toContain("after the next creature's turn")
    await again.get('[data-testid="lair-Rising Water"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'lair_action', tokenId: goblin.id, legend: 'Rising Water' })
  })

  it('lets the DM apply, end and resolve effects', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => saveRoll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const manual = [{ id: '0190c7a8-0000-7000-8000-000000000062', text: 'Goblin Boss: Resolve Hold Person by hand.' }]
    const saves = [{ rollId: '0190c7a8-0000-7000-8000-000000000063', tokenId: goblin.id, effect: 'Hold Person', dc: 13 }]
    const reduced = { id: '0190c7a8-0000-7000-8000-000000000066', slug: 'enlarge-reduce', name: 'Enlarge/Reduce', concentration: false, mode: 'Reduce' }
    s.receive(snapshot([{ ...aria, effects: [...(aria.effects ?? []), reduced], qualities: [{ quality: 'hidden', seenThrough: true }] }, goblin], 'dm', { manual, saves }))
    await flushPromises()
    expect(wrapper.get('[data-hex="0,0"]').attributes('aria-label')).toContain('Aria (12/12 HP) · Bless')
    expect(wrapper.get('[data-testid="manual"]').text()).toContain('Resolve Hold Person by hand.')
    expect(wrapper.get('[data-testid="roll-card"]').text()).toContain('Wisdom save to end Hold Person')
    await wrapper.get(`[data-testid="manual-done-${manual[0]?.id ?? ''}"]`).trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'resolve_manual', manualId: manual[0]?.id })
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    const panel = wrapper.get('[data-testid="effects-panel"]')
    expect(panel.text()).toContain('Bless · 9 rounds · concentration')
    expect(panel.text()).toContain('Enlarge/Reduce (Reduce)')
    await panel.get('[data-testid="end-effect-bless"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'end_effect', effectId: aria.effects?.[0]?.id })
    const sent = s.sent.length
    await panel.get('form').trigger('submit')
    expect(s.sent).toHaveLength(sent)
    await panel.get('[data-testid="effect-name"]').setValue('faerie-fire')
    await panel.get('[data-testid="effect-source"]').setValue(aria.id)
    expect(panel.get('[data-testid="concentration-warning"]').text()).toBe('Aria stops concentrating on Bless.')
    await panel.get('[data-testid="effect-name"]').setValue('hold-person')
    expect(panel.find('[data-testid="concentration-warning"]').exists()).toBe(false)
    await panel.get('[data-testid="effect-rounds"]').setValue(10)
    await panel.get('[data-testid="effect-save"]').setValue('wisdom')
    await panel.get('[data-testid="effect-dc"]').setValue(13)
    await expectAccessible(wrapper.element as Element)
    await panel.get('form').trigger('submit')
    expect(s.sent.at(-1)).toEqual({
      kind: 'apply_effect', targetId: aria.id, effect: 'hold-person', sourceId: aria.id, rounds: 10, saveAbility: 'wisdom', saveDc: 13,
      q: 0, r: 0, hidden: false, nonce: String(sent + 1),
    })
    await panel.get('[data-testid="effect-name"]').setValue('prone')
    await panel.get('[data-testid="effect-source"]').setValue('')
    await panel.get('[data-testid="effect-rounds"]').setValue(0)
    await panel.get('[data-testid="effect-save"]').setValue('')
    await panel.get('form').trigger('submit')
    expect(s.sent.at(-1)).toEqual({ kind: 'apply_effect', targetId: aria.id, effect: 'prone', q: 0, r: 0, hidden: false, nonce: String(sent + 2) })
    await panel.get('[data-testid="effect-name"]').setValue('enlarge-reduce')
    await panel.get('[data-testid="effect-mode"]').setValue(' Reduce ')
    await panel.get('form').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'apply_effect', effect: 'enlarge-reduce', effectMode: 'Reduce' })
    await panel.get('[data-testid="effect-name"]').setValue('wild-shape')
    await panel.get('[data-testid="effect-creature"]').setValue(' wolf ')
    await panel.get('[data-testid="effect-temp"]').setValue(4)
    await panel.get('form').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'apply_effect', effect: 'wild-shape', monsterSlug: 'wolf', tempHp: 4 })
    const vis = wrapper.get('[data-testid="visibility-panel"]')
    expect((vis.get('[data-testid="quality-hidden"]').element as HTMLInputElement).checked).toBe(true)
    expect((vis.get('[data-testid="seen-through-hidden"]').element as HTMLInputElement).checked).toBe(true)
    await vis.get('[data-testid="quality-hidden"]').setValue(false)
    await vis.get('[data-testid="quality-invisible"]').setValue(true)
    await vis.get('[data-testid="quality-disguised"]').setValue(true)
    await vis.get('[data-testid="disguise"]').setValue(' Old woman ')
    await vis.get('[data-testid="seen-through-disguised"]').setValue(true)
    await vis.get('[data-testid="save-visibility"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_visibility', tokenId: aria.id, qualities: ['invisible', 'disguised'], seenThrough: ['disguised'], disguise: 'Old woman' })
    await vis.get('[data-testid="quality-disguised"]').setValue(false)
    await vis.get('[data-testid="save-visibility"]').trigger('click')
    expect(s.sent.at(-1)).toEqual(expect.objectContaining({ qualities: ['invisible'], seenThrough: [] }))
    expect(s.sent.at(-1)).not.toHaveProperty('disguise')
    expect((panel.get('[data-testid="effect-mode"]').element as HTMLInputElement).value).toBe('')
  })

  it('tells players the DM is resolving and hands them their own saves', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => saveRoll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    const saves = [
      { rollId: '0190c7a8-0000-7000-8000-000000000064', tokenId: aria.id, effect: 'Hold Person', dc: 13 },
      { rollId: '0190c7a8-0000-7000-8000-000000000065', tokenId: goblin.id, effect: 'Hold Person', dc: 13 },
    ]
    s.receive(snapshot([aria, goblin], 'party', { resolving: true, saves }))
    await flushPromises()
    expect(wrapper.get('[data-testid="resolving"]').text()).toBe('The DM is resolving an effect.')
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="manual"]').exists()).toBe(false)
  })
})

describe('areas and terrain', () => {
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id, hp: 12, hpMax: 12, attacks: [] }
  const boss: LiveToken = { ...goblin, ac: 15, hp: 7, hpMax: 7, attacks: [{ name: 'Scimitar', toHit: 4, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d6', damageBonus: 2 }] }
  const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
    id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
    rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
    reaction: true, movementFt: 30, speedFt: 30, ...extra,
  })
  const pending = (id: string) => ({
    id, purpose: 'Fireball damage', notation: '8d6', requestedBy: 'Joris', roller: { id: member.id, name: 'Joris' }, mine: true, canRoll: true,
    status: 'pending', groups: [{ index: 0, count: 8, faces: 6, sign: 1 }], dice: Array.from({ length: 8 }, (_, no) => ({ no, group: 0, faces: 6, kept: false })),
    modifiers: [], createdAt: '2026-09-30T20:00:00Z',
  })
  const area = {
    casterId: boss.id, name: 'Fireball', hexes: [{ q: 1, r: 0 }, { q: 0, r: 0 }], damageRollId: '0190c7a8-0000-7000-8000-000000000071',
    saves: [{ tokenId: aria.id, rollId: '0190c7a8-0000-7000-8000-000000000072' }, { tokenId: boss.id, rollId: '0190c7a8-0000-7000-8000-000000000073' }],
  }

  it('teleports with Misty Step and shows temporary hit points and emanations that follow their bearer', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const combat = { status: 'active', round: 1, combatants: [fighter(boss, { acting: true }), fighter(aria)] }
    const guarded: LiveToken = { ...boss, effects: [{ id: '0190c7a8-0000-7000-8000-000000000091', slug: 'spirit-guardians', name: 'Spirit Guardians', concentration: true, hexes: [{ q: 1, r: 0 }] }] }
    s.receive(snapshot([{ ...aria, tempHp: 3, form: 'Wolf' }, guarded], 'dm', { combat }))
    await flushPromises()
    expect(wrapper.get('[data-hex="0,0"]').attributes('aria-label')).toContain('Aria as Wolf (12/12 HP +3 temp)')
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('in the area')
    await wrapper.get('[data-testid="hotbar-Goblin Boss"] [data-testid="misty-step"]').trigger('click')
    expect(wrapper.get('[data-testid="teleporting"]').text()).toBe('Tap a free hex within 30 feet.')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'teleport', tokenId: boss.id, effect: 'misty-step', q: 2, r: 0 })
    expect(wrapper.find('[data-testid="teleporting"]').exists()).toBe(false)
    const bar = '[data-testid="hotbar-Goblin Boss"]'
    await wrapper.get(`${bar} [data-testid="jump"]`).trigger('click')
    expect(wrapper.get('[data-testid="jumping"]').text()).toBe('Tap where to land.')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'jump', tokenId: boss.id, q: 2, r: 0 })
    await wrapper.get(`${bar} [data-testid="throw"]`).trigger('click')
    expect(wrapper.get('[data-testid="throwing"]').text()).toBe('Tap the creature or object to throw.')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(wrapper.get('[data-testid="throwing"]').text()).toBe('Tap the creature or object to throw.')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(wrapper.get('[data-testid="throwing"]').text()).toBe('Tap where it lands.')
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'throw', tokenId: boss.id, targetId: aria.id, q: 0, r: 1 })
    s.receive(snapshot([aria, guarded], 'dm', { combat, objects: [{ id: '0190c7a8-0000-7000-8000-0000000000b1', kind: 'barrel', name: 'Barrel', q: 1, r: 1, open: false, broken: false }] }))
    await flushPromises()
    await wrapper.get(`${bar} [data-testid="throw"]`).trigger('click')
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'throw', tokenId: boss.id, objectId: '0190c7a8-0000-7000-8000-0000000000b1', q: 0, r: 1 })
  })

  it('summons creatures, marks them in the roster and commands them', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const wolf: LiveToken = { ...boss, id: '0190c7a8-0000-7000-8000-000000000093', label: 'Wolf', q: 1, r: 1 }
    const owner = fighter(boss, { acting: true })
    const combat = { status: 'active', round: 1, combatants: [owner, fighter(aria), fighter(wolf, { acting: true, ownerId: owner.id, awaitingCommand: true })] }
    s.receive(snapshot([aria, boss, wolf], 'dm', { combat }))
    await flushPromises()
    expect(wrapper.get('[data-testid="summoned-Wolf"]').text()).toBe('Summoned by Goblin Boss · awaiting orders')
    await wrapper.get('[data-testid="hotbar-Goblin Boss"] [data-testid="command-Wolf"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'command', tokenId: boss.id, targetId: wolf.id })
    await wrapper.get('[data-testid="hotbar-Goblin Boss"] [data-testid="summon"]').setValue('animate-dead')
    expect(wrapper.get('[data-testid="summoning"]').text()).toBe('Tap where they appear.')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'summon', tokenId: boss.id, effect: 'animate-dead', q: 2, r: 0 })
    expect(wrapper.find('[data-testid="summoning"]').exists()).toBe(false)
    await wrapper.get('[data-testid="hotbar-Goblin Boss"] [data-testid="summon"]').setValue('')
    expect(wrapper.find('[data-testid="summoning"]').exists()).toBe(false)
    s.receive(snapshot([aria, boss, wolf], 'dm', { combat: { ...combat, combatants: [owner, fighter(aria), fighter(wolf, { ownerId: '0190c7a8-0000-7000-8000-000000000000' })] } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="summoned-Wolf"]').text()).toBe('Summoned by someone')
  })

  it('lets the DM aim an area spell, see who it catches and cast it', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => pending(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const combat = { status: 'active', round: 1, combatants: [fighter(boss, { acting: true }), fighter(aria)] }
    s.receive(snapshot([aria, boss], 'dm', { combat, surfaces: [{ q: 0, r: 1, kind: 'fire', roundsLeft: 2 }] }))
    await flushPromises()
    expect(wrapper.get('[data-hex="0,1"]').attributes('aria-label')).toBe('Hex 0, 1: fire')
    await wrapper.get('[data-testid="area-spell"]').setValue('fireball')
    expect(wrapper.get('[data-testid="area-aiming"]').text()).toBe('Tap where the spell goes.')
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'preview_area', tokenId: boss.id, effect: 'fireball', q: 1, r: 0 })
    const preview = { tokenId: boss.id, effect: 'fireball', name: 'Fireball', dc: 13, hexes: area.hexes, targets: [{ tokenId: aria.id, ally: false, pushedTo: { q: -1, r: 0 } }, { tokenId: boss.id, ally: true }, { tokenId: '0190c7a8-0000-7000-8000-000000000099', ally: false }], allies: 1, ends: ['Bless'] }
    s.receive({ kind: 'area_preview', seq: 1, area: { ...preview, effect: 'shatter' } })
    await flushPromises()
    expect(wrapper.find('[data-testid="area-preview"]').exists()).toBe(false)
    s.receive({ kind: 'area_preview', seq: 1, area: preview })
    await flushPromises()
    const card = wrapper.get('[data-testid="area-preview"]')
    expect(card.get('h2').text()).toBe('Fireball · DC 13')
    expect(card.get('[data-testid="ally-warning"]').text()).toBe('This catches 1 ally.')
    expect(card.get('[data-testid="concentration-warning"]').text()).toBe('Casting this ends your concentration on Bless.')
    expect(card.text()).toContain('Goblin Boss (ally)')
    expect(card.text()).toContain('Aria · pushed to -1, 0 on a failed save')
    expect(card.text()).toContain('Someone')
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('in the area')
    await expectAccessible(wrapper.element as Element)
    await card.get('[data-testid="cancel-area"]').trigger('click')
    expect(wrapper.find('[data-testid="area-preview"]').exists()).toBe(false)
    await wrapper.get('[data-testid="area-spell"]').setValue('')
    expect(wrapper.find('[data-testid="area-aiming"]').exists()).toBe(false)
    await wrapper.get('[data-testid="area-slot"]').setValue(5)
    await wrapper.get('[data-testid="area-spell"]').setValue('fireball')
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'preview_area', effect: 'fireball', slot: 5 })
    s.receive({ kind: 'area_preview', seq: 1, area: { ...preview, allies: 2, targets: [] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="ally-warning"]').text()).toBe('This catches 2 allies.')
    expect(wrapper.find('[data-testid="area-empty"]').exists()).toBe(true)
    await wrapper.get('[data-testid="confirm-area"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'cast_area', tokenId: boss.id, effect: 'fireball', q: 1, r: 0, slot: 5 })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, boss], fog: false, visible: [], remembered: [], combat, area } })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(2)

    await wrapper.get('[data-testid="tool-surface"]').setValue(true)
    await wrapper.get('[data-testid="surface-kind"]').setValue('grease')
    await wrapper.get('[data-testid="surface-rounds"]').setValue(3)
    s.receive({ kind: 'view', seq: 3, view: { tokens: [aria, boss], fog: false, visible: [], remembered: [], combat } })
    await flushPromises()
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'paint_surface', hexes: [{ q: -1, r: 1 }], surface: 'grease', rounds: 3 })
    await wrapper.get('[data-testid="surface-kind"]').setValue('')
    await wrapper.get('[data-testid="surface-rounds"]').setValue(0)
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(s.sent.at(-1)).toEqual({ kind: 'paint_surface', hexes: [{ q: -1, r: 1 }], q: 0, r: 0, hidden: false, nonce: String(s.sent.length) })
    await wrapper.get('[data-testid="tool-elevation"]').setValue(true)
    await wrapper.get('[data-testid="elevation-ft"]').setValue(15)
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_elevation', hexes: [{ q: -1, r: 1 }], elevationFt: 15 })
  })

  it('shows height and surfaces on a map and hands players their own area rolls', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => pending(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([aria, boss], 'party', {
      map: liveMap, fog: false, visible: [], remembered: [], area,
      surfaces: [{ q: 1, r: 1, kind: 'ice' }], elevation: [{ q: 1, r: 1, elevationFt: 10 }],
    }))
    await flushPromises()
    expect(wrapper.get('[data-hex="1,1"]').attributes('aria-label')).toBe('Hex 1, 1: ice: 10 ft high')
    expect(wrapper.get('[data-hex="1,1"]').classes()).toContain('cell--surface-ice')
    expect(wrapper.get('[data-hex="1,0"]').classes()).toContain('cell--area')
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(1)
  })
})

describe('map objects', () => {
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id }
  const door = { id: '0190c7a8-0000-7000-8000-0000000000a1', kind: 'door' as const, name: 'Door', q: 1, r: 0, open: false, broken: false, ac: 15, hp: 18, hpMax: 18 }
  const lever = { id: '0190c7a8-0000-7000-8000-0000000000a2', kind: 'lever' as const, name: 'Lever', q: 0, r: 1, open: true, broken: false, secret: true, ac: 19, hp: 5, hpMax: 5 }
  const barrel = { id: '0190c7a8-0000-7000-8000-0000000000a3', kind: 'barrel' as const, name: 'Barrel', q: 1, r: 1, open: false, broken: true }

  it('lets the DM place, find, damage and remove objects, and anyone next to one use it', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([aria], 'dm', { map: liveMap, objects: [door, lever, barrel], surfaceKinds: [{ kind: 'stinking-cloud', name: 'Stinking cloud' }], elevation: [{ q: 0, r: 1, elevationFt: -10 }, { q: 1, r: 1, elevationFt: 20 }] }))
    await flushPromises()
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('Door (closed)')
    expect(wrapper.find('[data-hex="0,1"] [data-height="-10"]').exists()).toBe(true)
    expect(wrapper.get('[data-hex="1,1"] [data-height="20"]').attributes('style')).toContain('opacity: 0.52')
    await wrapper.get('[data-testid="tool-surface"]').setValue(true)
    expect(wrapper.get('[data-testid="surface-kind"]').text()).toContain('Stinking cloud')
    await wrapper.get('[data-testid="tool-tokens"]').setValue(true)
    expect(wrapper.get('[data-hex="0,1"]').attributes('aria-label')).toContain('Lever (pulled)')
    expect(wrapper.get('[data-testid="object-Door"]').text()).toContain('Door · closed · 18/18 HP, AC 15')
    expect(wrapper.get('[data-testid="object-Lever"]').text()).toContain('secret')
    expect(wrapper.get('[data-testid="object-Barrel"]').text()).toContain('broken')
    expect(wrapper.find('[data-testid="use-Door"]').exists()).toBe(false)
    await wrapper.get('[data-testid="find-Lever"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'find_object', objectId: lever.id })
    await wrapper.get('[data-testid="hit-Door"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'find_object' })
    await wrapper.get('[data-testid="damage-Door"]').setValue(5)
    await wrapper.get('[data-testid="hit-Door"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'damage_object', objectId: door.id, hpDelta: -5 })
    await wrapper.get('[data-testid="remove-Barrel"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'remove_object', objectId: barrel.id })
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    await wrapper.get('[data-testid="use-Door"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'use_object', tokenId: aria.id, objectId: door.id })
    expect(wrapper.get('[data-testid="use-Lever"]').text()).toBe('Pull')
    s.receive(snapshot([aria], 'dm', { map: liveMap, objects: [{ ...door, open: true }] }))
    await flushPromises()
    expect(wrapper.get('[data-testid="use-Door"]').text()).toBe('Close')
    await wrapper.get('[data-testid="tool-object"]').setValue(true)
    await wrapper.get('[data-testid="object-kind"]').setValue('barrel')
    await wrapper.get('[data-testid="object-name"]').setValue(' Powder keg ')
    await wrapper.get('[data-testid="object-effect"]').setValue('prone')
    await wrapper.get('[data-testid="object-radius"]').setValue(5)
    await wrapper.get('[data-testid="object-secret"]').setValue(true)
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_object', objectKind: 'barrel', objectName: 'Powder keg', effect: 'prone', radiusFt: 5, secret: true, q: 2, r: 0 })
    await wrapper.get('[data-testid="object-kind"]').setValue('trap')
    for (const [id, value] of [['object-detect', 12], ['object-disarm', 15], ['object-trigger', 5], ['object-lock', 0]] as const) {
      await wrapper.get(`[data-testid="${id}"]`).setValue(value)
    }
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_object', objectKind: 'trap', detectDc: 12, disarmDc: 15, triggerFt: 5 })
    await wrapper.get('[data-testid="object-kind"]').setValue('chest')
    for (const id of ['object-detect', 'object-disarm', 'object-trigger']) await wrapper.get(`[data-testid="${id}"]`).setValue(0)
    await wrapper.get('[data-testid="object-lock"]').setValue(14)
    await wrapper.get('[data-testid="object-key"]').setValue(' iron-key ')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_object', objectKind: 'chest', lockDc: 14, key: 'iron-key' })
    await wrapper.get('[data-testid="object-key"]').setValue('')
    await wrapper.get('[data-testid="object-lock"]').setValue(0)
    await wrapper.get('[data-testid="object-name"]').setValue('')
    await wrapper.get('[data-testid="object-effect"]').setValue('')
    await wrapper.get('[data-testid="object-radius"]').setValue(0)
    await wrapper.get('[data-testid="object-secret"]').setValue(false)
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    expect(s.sent.at(-1)).toEqual(expect.objectContaining({ kind: 'place_object', objectKind: 'chest', q: 1, r: 1 }))
    expect(s.sent.at(-1)).not.toHaveProperty('objectName')
    const trap = { id: '0190c7a8-0000-7000-8000-0000000000a4', kind: 'trap' as const, name: 'Pit', q: 2, r: 0, open: false, broken: false, armed: true }
    s.receive(snapshot([aria], 'dm', { map: liveMap, objects: [{ ...lever, open: false, secret: false }, { ...barrel, broken: false }, { ...door, locked: true }, trap] }))
    await flushPromises()
    expect(wrapper.get('[data-hex="0,1"]').attributes('aria-label')).toContain('Lever (up)')
    expect(wrapper.get('[data-hex="1,1"]').attributes('aria-label')).toContain('Barrel (closed)')
    expect(wrapper.get('[data-testid="object-Door"]').text()).toContain('closed, locked')
    expect(wrapper.find('[data-testid="use-Door"]').exists()).toBe(false)
    for (const m of ['key', 'tools', 'force', 'knock']) {
      await wrapper.get(`[data-testid="unlock-${m}-Door"]`).trigger('click')
      expect(s.sent.at(-1)).toMatchObject({ kind: 'unlock', tokenId: aria.id, objectId: door.id, method: m })
    }
    expect(wrapper.get('[data-testid="object-Pit"]').text()).toContain('Pit · trap')
    await wrapper.get('[data-testid="disarm-Pit"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'disarm', tokenId: aria.id, objectId: trap.id })
    s.receive(snapshot([aria], 'dm', { map: liveMap, objects: [{ ...trap, armed: false }] }))
    await flushPromises()
    expect(wrapper.get('[data-testid="object-Pit"]').text()).toContain('spent')
    expect(wrapper.find('[data-testid="disarm-Pit"]').exists()).toBe(false)
  })
})

describe('sneaking', () => {
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id }

  it('lets the party sneak and tints the hexes watched creatures can notice it in', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([aria], 'party'))
    await flushPromises()
    await wrapper.get('[data-testid="start-sneaking"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'sneak', on: true })
    s.receive(snapshot([aria], 'party', { sneak: { waiting: true, reach: [{ q: 1, r: 0 }] } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="sneak-status"]').text()).toBe('Sneaking: roll Stealth.')
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('watched')
    s.receive(snapshot([aria], 'party', { sneak: { waiting: false, reach: [] } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="sneak-status"]').text()).toBe('Sneaking. Tinted hexes are watched.')
    await wrapper.get('[data-testid="stop-sneaking"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'sneak', on: false })
    s.receive(snapshot([aria], 'party', { map: liveMap, sneak: { waiting: false, reach: [{ q: 0, r: 0 }] } }))
    await flushPromises()
    expect(wrapper.get('[data-hex="0,0"]').attributes('aria-label')).toContain('watched')
    expect(wrapper.find('[data-testid="start-turns"]').exists()).toBe(false)
    s.receive(snapshot([aria], 'party', { exploration: { order: [aria.id], turn: aria.id, leftFt: 20 } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="exploration-turn"]').text()).toBe('Aria explores · 20 ft left')
    await wrapper.get('[data-testid="pass-turn"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'pass_turn' })
  })

  it('lets the DM put exploration into turns', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([aria], 'dm'))
    await flushPromises()
    await wrapper.get('[data-testid="start-turns"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'explore', on: true })
    s.receive(snapshot([aria], 'dm', { exploration: { order: [aria.id], turn: '0190c7a8-0000-7000-8000-000000000099', leftFt: 30 } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="exploration-turn"]').text()).toBe('Someone explores · 30 ft left')
    await wrapper.get('[data-testid="stop-turns"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'explore', on: false })
  })
})

describe('table remote', () => {
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 2 }
  const brom: LiveToken = { ...aria, id: '0190c7a8-0000-7000-8000-00000000000f', label: 'Brom', q: 2, r: 0 }
  const table = (extra: Partial<LiveTable> = {}): LiveTable => ({ camera: 'follow_turn', q: 0, r: 0, zoomPct: 100, scene: 'local', blackout: false, ...extra })

  it('points the camera at the turn, the party or where the DM put it', () => {
    const base = { tokens: [goblin, aria, brom], fog: false, visible: [], remembered: [] }
    expect(focus({ ...base, table: table({ camera: 'free', q: 3, r: -2 }) })).toEqual({ q: 3, r: -2 })
    expect(focus({ ...base, table: table({ camera: 'show_party' }) })).toEqual({ q: 1, r: 1 })
    const combat = { status: 'active' as const, round: 1, combatants: [{ id: goblin.id, tokenId: goblin.id, label: 'Goblin Boss', kind: 'enemy' as const, rollId: goblin.id, acting: true, done: false, action: true, bonusAction: true, reaction: true, movementFt: 30, speedFt: 30 }] }
    expect(focus({ ...base, combat, table: table() })).toEqual({ q: 1, r: 0 })
    expect(focus({ ...base, table: table() })).toEqual({ q: 1, r: 1 })
    expect(focus({ ...base, tokens: [goblin] })).toEqual({ q: 0, r: 0 })
  })

  it('shows the Table Display scene the DM chose, dark when blacked out', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, {})
    const s = FakeSocket.last()
    s.receive(snapshot([goblin, aria], 'table', { table: table({ camera: 'free', q: 1, r: 0, zoomPct: 200 }) }))
    await flushPromises()
    const world = wrapper.get('[data-testid="camera-world"]')
    expect(world.attributes('style')).toContain('scale(2)')
    s.receive({ kind: 'ping', seq: 1, ping: { q: 1, r: 0 } })
    await flushPromises()
    expect(wrapper.get('[data-testid="ping"]').attributes('aria-label')).toBe('The DM pinged here')
    const views = [
      [table({ scene: 'title', title: 'Chapter One', body: 'The mists close in.' }), 'scene-title', 'Chapter One'],
      [table({ scene: 'handout', title: 'A letter', body: 'Come quickly.' }), 'scene-handout', 'Come quickly.'],
      [table({ scene: 'world', worldMap: liveMap }), 'scene-world', ''],
      [table({ blackout: true }), 'blackout', ''],
    ] as const
    let seq = 2
    for (const [t, id, text] of views) {
      s.receive({ kind: 'view', seq: seq++, view: { tokens: [], fog: false, visible: [], remembered: [], table: t } })
      await flushPromises()
      expect(wrapper.get(`[data-testid="${id}"]`).text()).toContain(text)
    }
    s.receive({ kind: 'view', seq: seq++, view: { tokens: [aria], fog: false, visible: [], remembered: [], map: liveMap, table: table({ camera: 'show_party' }) } })
    await flushPromises()
    expect(wrapper.find('[data-testid="map-board"]').exists()).toBe(true)
    await expectAccessible(wrapper.element as Element)
  })

  it('lets the DM steer the table from the session page', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/maps`]: () => [localMap, realmMap],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([goblin], 'dm', { table: table({ zoomPct: 120 }) }))
    await flushPromises()
    const remote = wrapper.get('[data-testid="table-remote"]')
    await remote.get('[data-testid="camera-show_party"]').setValue(true)
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_camera', camera: 'show_party', zoomPct: 120 })
    await remote.get('[data-testid="camera-zoom"]').setValue(200)
    await remote.get('[data-testid="camera-zoom"]').trigger('change')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_camera', camera: 'follow_turn', zoomPct: 200 })
    await remote.get('[data-testid="scene-title"]').setValue('Chapter One')
    await remote.get('[data-testid="scene-body"]').setValue('Mists.')
    await remote.get('form').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_scene', scene: 'title', title: 'Chapter One', body: 'Mists.' })
    await remote.get('[data-testid="scene"]').setValue('world')
    expect(remote.get('[data-testid="scene-map"]').findAll('option').map((o) => o.text())).toEqual(['Realm'])
    await remote.get('[data-testid="scene-map"]').setValue(WID)
    await remote.get('form').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_scene', scene: 'world', mapId: WID })
    expect(s.sent.at(-1)).not.toHaveProperty('title')
    await remote.get('[data-testid="scene"]').setValue('local')
    await remote.get('form').trigger('submit')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_scene', scene: 'local' })
    await remote.get('[data-testid="blackout-toggle"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_blackout', on: true })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [goblin], fog: false, visible: [], remembered: [], table: table({ blackout: true, zoomPct: 150 }) } })
    await flushPromises()
    expect(remote.get('[data-testid="blackout-toggle"]').text()).toBe('Lights back on')
    expect((remote.get('[data-testid="camera-zoom"]').element as HTMLInputElement).value).toBe('150')
    await wrapper.get('[data-testid="tool-camera"]').setValue(true)
    await wrapper.get('[data-hex="1,-1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'table_camera', camera: 'free', q: 1, r: -1, zoomPct: 150 })
    await wrapper.get('[data-testid="tool-ping"]').setValue(true)
    await wrapper.get('[data-hex="1,-1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'ping', q: 1, r: -1 })
    await expectAccessible(wrapper.element as Element)
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
          writes.push(`POST ${String(u.searchParams.get('name'))} ${String(u.searchParams.get('kind'))} ${String((await req.arrayBuffer()).byteLength)}`)
          return localMap
        }
        return [localMap]
      },
    })
    expect(wrapper.get('[data-testid="map-list"]').text()).toContain('Crypt')
    expect(wrapper.get('[data-testid="map-list"]').text()).toContain('local map · 200 × 160 px')
    const form = wrapper.get('[data-testid="map-upload"]')
    expect(form.get('button').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="map-name"]').setValue('Crypt')
    await wrapper.get('[data-testid="map-kind"]').setValue('world')
    const input = wrapper.get('[data-testid="map-file"]')
    Object.defineProperty(input.element, 'files', { value: [new File([new Uint8Array(5)], 'crypt.png', { type: 'image/png' })], configurable: true })
    await input.trigger('change')
    await expectAccessible(wrapper.element as Element)
    await form.trigger('submit')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('map') }, { timeout: 5000 })
    expect(writes).toEqual(['POST Crypt world 5'])
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
    expect(writes[1]).toBe('PUT {"name":"Crypt of Night","hexSizePx":50,"originX":20,"originY":25,"ambient":"dim","gridStrength":20}')
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

describe('world map', () => {
  it('says how long a journey takes', () => {
    expect([duration(0, 0), duration(45, 1), duration(240, 1), duration(270, 1), duration(600, 2)]).toEqual([
      '0 min', '45 min', '4 h', '4 h 30 min', '2 days (10 h on the road)',
    ])
    expect(journey(realm().legs)).toBe('42 mi · 3 days (19 h on the road)')
  })

  it('lets the DM choose, draw and travel the world map', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/maps`]: () => [localMap, realmMap],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([goblin], 'dm'))
    await flushPromises()
    expect(wrapper.get('[data-testid="map-choice"]').findAll('option').map((o) => o.text())).toEqual(['No map (open grid)', 'Crypt'])
    await wrapper.get('[data-testid="scope-world"]').setValue(true)
    expect(wrapper.get('[data-testid="no-world"]').text()).toContain('Choose a world map')
    expect(wrapper.get('[data-testid="dm-controls"]').isVisible()).toBe(false)
    await wrapper.get('[data-testid="use-world"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_world' })
    expect(s.sent.at(-1)).not.toHaveProperty('mapId')
    await wrapper.get('[data-testid="world-choice"]').setValue(WID)
    await wrapper.get('[data-testid="use-world"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'set_world', mapId: WID })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [goblin], fog: false, visible: [], remembered: [], world: realm() } })
    await flushPromises()
    expect(wrapper.find('[data-testid="party-marker"]').exists()).toBe(true)
    expect(wrapper.find('[data-node="Mill"]').exists()).toBe(true)
    expect(wrapper.find(`[data-route="${ROAD}"]`).exists()).toBe(true)
    expect(wrapper.get('[data-testid="party-at"]').text()).toBe('The party is at Oakford.')
    const roads = wrapper.get('[data-testid="roads"]')
    expect(roads.text()).toContain('Mill · 12 mi · 4 h')
    await roads.get('[data-testid="pace"]').setValue('fast')
    expect(roads.text()).toContain('Mill · 12 mi · 3 h')
    await roads.get('button').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'travel', routeId: ROAD, pace: 'fast' })
    expect(wrapper.get('[data-testid="legs"]').text()).toContain('Oakford → Mill · 30 mi at a slow pace · 2 days (15 h on the road)')
    expect(wrapper.get('[data-testid="journey"]').text()).toBe('In all: 42 mi · 3 days (19 h on the road)')
    const sent = () => s.sent.length
    let before = sent()
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    expect(sent()).toBe(before)
    await wrapper.get('[data-testid="world-node-name"]').setValue(' Ford ')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(sent()).toBe(before)
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'add_node', label: 'Ford', q: 1, r: 1 })
    await wrapper.get('[data-testid="world-tool-party"]').setValue(true)
    before = sent()
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    expect(sent()).toBe(before)
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_party', nodeId: MILL })
    await wrapper.get('[data-testid="world-tool-remove"]').setValue(true)
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'remove_node', nodeId: OAK })
    await wrapper.get('[data-testid="world-tool-route"]').setValue(true)
    await wrapper.get('[data-testid="world-distance"]').setValue(30)
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(wrapper.text()).toContain('From Oakford: now tap where the route goes.')
    expect(wrapper.get('[data-node="Oakford"] circle').classes()).toContain('node--from')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'add_route', nodeId: OAK, toNodeId: MILL, distanceMi: 30 })
    expect(wrapper.text()).toContain('Tap the location the route starts from.')
    await wrapper.get(`[data-testid="remove-route-${ROAD}"]`).trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'remove_route', routeId: ROAD })
    await expectAccessible(wrapper.element as Element)
    s.receive({ kind: 'view', seq: 3, view: { tokens: [goblin], fog: false, visible: [], remembered: [], world: realm({ partyNodeId: undefined, legs: [], routes: [{ id: ROAD, fromNodeId: OAK, toNodeId: '0190c7a8-0000-7000-8000-000000000099', distanceMi: 12, plans: [] }] }) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="party-at"]').text()).toBe('The party is not on this map yet.')
    expect(wrapper.find('[data-testid="legs"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Oakford – somewhere unseen · 12 mi')
    await wrapper.get('[data-testid="scope-local"]').setValue(true)
    expect(wrapper.get('[data-testid="dm-controls"]').isVisible()).toBe(true)
  })

  it('shows players the world map without the DM tools', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party'))
    await flushPromises()
    await wrapper.get('[data-testid="scope-world"]').setValue(true)
    expect(wrapper.get('[data-testid="no-world"]').text()).toBe('The DM has not opened a world map yet.')
    s.receive({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [], world: realm() } })
    await flushPromises()
    expect(wrapper.find('[data-testid="world-choice"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="roads"]').text()).toContain('Mill · 12 mi · 4 h')
    expect(wrapper.get('[data-testid="roads"]').find('button').exists()).toBe(false)
    const before = s.sent.length
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    expect(s.sent.length).toBe(before)
    expect(wrapper.get('[data-hex="2,0"]').classes()).toContain('cell--unseen')
    await expectAccessible(wrapper.element as Element)
    s.receive({ kind: 'view', seq: 3, view: { tokens: [], fog: false, visible: [], remembered: [], world: realm({ partyNodeId: MILL, routes: [{ ...realm().routes[0], plans: [] } as LiveWorld['routes'][number]] }) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="roads"]').text()).toContain('Oakford · 12 mi ·')
  })

  it('draws the party travels on the Table Display when it shows their world map', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, {})
    const s = FakeSocket.last()
    const scene: LiveTable = { camera: 'follow_turn', q: 0, r: 0, zoomPct: 100, scene: 'world', blackout: false, worldMap: { ...liveMap, id: WID, name: 'Realm' } }
    s.receive(snapshot([], 'table', { table: scene, world: realm() }))
    await flushPromises()
    expect(wrapper.find('[data-testid="party-marker"]').exists()).toBe(true)
    expect(wrapper.get('[data-hex="2,0"]').classes()).toContain('cell--unseen')
    s.receive({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [], table: { ...scene, worldMap: liveMap }, world: realm() } })
    await flushPromises()
    expect(wrapper.find('[data-testid="party-marker"]').exists()).toBe(false)
    expect(wrapper.get('[data-hex="2,0"]').classes()).toContain('cell--lit')
  })
})

describe('encounter zones', () => {
  const ZID = '0190c7a8-0000-7000-8000-000000000025'
  const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, controllerId: player.id }
  const brom: LiveToken = { ...aria, id: '0190c7a8-0000-7000-8000-00000000000f', label: 'Brom', q: 0, r: 1, controllerId: undefined }
  const zone = (extra: Partial<LiveZone> = {}): LiveZone => ({
    id: ZID, name: 'Ambush', q: 1, r: 0, radiusHexes: 1, dmOnly: false, held: false, status: 'armed', creatures: 2, checks: [], ...extra,
  })
  const perceptionRoll = (id: string) => ({
    id, purpose: 'Perception for Brom', notation: '1d20', requestedBy: 'Joris', roller: { id: member.id, name: 'Joris' }, mine: true, canRoll: true,
    status: 'pending', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }], dice: [{ no: 0, group: 0, faces: 20, kept: false }],
    modifiers: [], createdAt: '2026-09-30T20:00:00Z',
  })

  it('finds the hexes each zone reaches', () => {
    expect(zoneHexes([{ q: 0, r: 0, radiusHexes: 1 }], hexes(2))).toHaveLength(7)
    expect(zoneHexes([], hexes(2))).toEqual([])
  })

  it('lets the DM draw, hold, spring and remove zones, and roll Perception for their creatures', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => perceptionRoll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([aria, brom], 'dm', { zones: [zone()] }))
    await flushPromises()
    expect(wrapper.get('[data-hex="1,0"]').classes()).toContain('hex--zone')
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('in an encounter zone')
    const panel = wrapper.get('[data-testid="zone-Ambush"]')
    expect(panel.text()).toContain('Ambush · 1 hex · 2 hidden · Armed')
    await panel.get('[aria-label="Spring Ambush now"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'spring_zone', zoneId: ZID })
    await panel.get('[aria-label="Hold off Ambush"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'hold_zone', zoneId: ZID, on: true })
    await wrapper.get('[data-testid="tool-zone"]').setValue(true)
    const before = s.sent.length
    await wrapper.get('[data-hex="-1,0"]').trigger('click')
    expect(s.sent.length).toBe(before)
    await wrapper.get('[data-testid="zone-name"]').setValue(' Den ')
    await wrapper.get('[data-testid="zone-radius"]').setValue(4)
    await wrapper.get('[data-testid="zone-dm-only"]').setValue(true)
    await wrapper.get('[data-hex="-1,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'add_zone', label: 'Den', q: -1, r: 0, radiusHexes: 4, dmOnly: true })
    const statuses = [
      [zone({ held: true }), 'Held off', 'Let Ambush'],
      [zone({ dmOnly: true, radiusHexes: 2 }), 'Springs when you say', 'Hold off Ambush'],
    ] as const
    let seq = 2
    for (const [z, text, hold] of statuses) {
      s.receive({ kind: 'view', seq: seq++, view: { tokens: [aria, brom], fog: false, visible: [], remembered: [], zones: [z] } })
      await flushPromises()
      expect(wrapper.get('[data-testid="zone-Ambush"]').text()).toContain(text)
      await wrapper.get(`[aria-label="${hold}"]`).trigger('click')
    }
    expect(s.sent.at(-1)).toMatchObject({ kind: 'hold_zone', on: true })
    const rolling = zone({ status: 'spotting', dc: 16, checks: [{ tokenId: aria.id, noticed: true }, { tokenId: brom.id }, { tokenId: goblin.id, noticed: false }] })
    s.receive({ kind: 'view', seq: seq++, view: { tokens: [aria, brom], fog: false, visible: [], remembered: [], zones: [rolling], perception: [{ rollId: '0190c7a8-0000-7000-8000-000000000061', tokenId: brom.id }, { rollId: '0190c7a8-0000-7000-8000-000000000062', tokenId: aria.id }] } })
    await flushPromises()
    const spotting = wrapper.get('[data-testid="zone-Ambush"]')
    expect(spotting.text()).toContain('Waiting for Perception · Stealth DC 16')
    expect(spotting.text()).toContain('Aria: noticed')
    expect(spotting.text()).toContain('Brom: rolling Perception')
    expect(spotting.text()).toContain('Someone: unaware')
    expect(spotting.find('[aria-label="Spring Ambush now"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(1)
    await spotting.get('[aria-label="Remove Ambush"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'remove_zone', zoneId: ZID })
    s.receive({ kind: 'view', seq: seq++, view: { tokens: [aria, brom], fog: false, visible: [], remembered: [], zones: [zone({ status: 'sprung' })] } })
    await flushPromises()
    expect(wrapper.get('[data-testid="zone-Ambush"]').text()).toContain('Sprung')
    await expectAccessible(wrapper.element as Element)
  })

  it('shows players their Perception card and who was surprised, never the zone', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => ({ ...perceptionRoll(u.pathname.split('/')[6] ?? ''), purpose: 'Perception for Aria' }),
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([aria, brom], 'party', { perception: [{ rollId: '0190c7a8-0000-7000-8000-000000000061', tokenId: brom.id }, { rollId: '0190c7a8-0000-7000-8000-000000000062', tokenId: aria.id }] }))
    await flushPromises()
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="roll-card"]').text()).toContain('Perception for Aria')
    expect(wrapper.find('[data-testid="zones"]').exists()).toBe(false)
    const combatant = { id: '0190c7a8-0000-7000-8000-000000000070', tokenId: aria.id, label: 'Aria', kind: 'party' as const, rollId: '0190c7a8-0000-7000-8000-000000000071', acting: false, done: false, action: true, bonusAction: true, reaction: true, movementFt: 30, speedFt: 30 }
    s.receive({ kind: 'view', seq: 2, view: { tokens: [aria, brom], fog: false, visible: [], remembered: [], combat: { status: 'rolling', round: 0, combatants: [{ ...combatant, surprised: true }, { ...combatant, id: '0190c7a8-0000-7000-8000-000000000072', tokenId: brom.id, label: 'Brom' }] } } })
    await flushPromises()
    expect(wrapper.get('[data-testid="rail-Aria"] [data-testid="surprised"]').text()).toBe('Surprised')
    expect(wrapper.find('[data-testid="rail-Brom"] [data-testid="surprised"]').exists()).toBe(false)
  })
})

describe('encounter checks', () => {
  const ROAD = '0190c7a8-0000-7000-8000-000000000032'
  const road = {
    id: ROAD, name: 'Road', chancePct: 30, visibility: 'open', updatedAt: '2026-10-01T20:00:00Z',
    entries: [{ weight: 1, kind: 'encounter', label: 'Ambush', monsters: [{ monsterSlug: 'goblin', count: 3 }] }, { weight: 1, kind: 'nothing', label: '', monsters: [] }],
  }
  const ROLL = '0190c7a8-0000-7000-8000-000000000045'
  const dmChecks: LiveCheck[] = [
    { id: '0190c7a8-0000-7000-8000-000000000041', trigger: 'long_rest', visibility: 'open', status: 'resolved', outcome: 'encounter', chancePct: 30, chanceRoll: 12, tableName: 'Road', mode: 'normal', seed: '42', entryLabel: 'Ambush', monsters: [{ slug: 'goblin', count: 3 }, { slug: 'ogre', count: 1 }] },
    { id: '0190c7a8-0000-7000-8000-000000000042', trigger: 'dm', visibility: 'secret', status: 'resolved', outcome: 'nothing', tableName: 'Den', mode: 'pick', seed: '7', entryLabel: 'Wind' },
    { id: '0190c7a8-0000-7000-8000-000000000043', trigger: 'travel_leg', visibility: 'open', status: 'pending', chancePct: 30, rollId: ROLL, tableName: 'Road', mode: 'normal', seed: '8' },
  ]
  const [longRest, picked, travelling] = dmChecks as [LiveCheck, LiveCheck, LiveCheck]
  const roll = (id: string) => ({
    id, purpose: 'Encounter check', notation: '1d100', requestedBy: 'Joris', roller: { id: member.id, name: 'Joris' }, mine: true, canRoll: true,
    status: 'pending', groups: [{ index: 0, count: 1, faces: 100, sign: 1 }], dice: [{ no: 0, group: 0, faces: 100, kept: false }], modifiers: [], createdAt: '2026-10-01T20:00:00Z',
  })

  it('writes each check as one line', () => {
    expect(checkLine(longRest)).toBe('Long rest on Road · rolled 12 against 30% · encounter: Ambush · goblin x3, ogre · seed 42')
    expect(checkLine(picked)).toBe('The DM checks on Den · all quiet (Wind) · seed 7')
    expect(checkLine(travelling)).toBe('Travel leg on Road · rolling… · seed 8')
    expect(checkLine({ id: ROLL, trigger: 'short_rest', visibility: 'secret', status: 'resolved', outcome: 'encounter' })).toBe('Short rest · something approaches!')
    expect(checkLine({ id: ROLL, trigger: 'short_rest', visibility: 'open', status: 'resolved', outcome: 'nothing', chanceRoll: 80 })).toBe('Short rest · rolled 80 against 0% · all quiet')
  })

  it('lets the DM rest, check, force, pick and schedule, and roll the open check', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/encounter-tables`]: () => [road],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => roll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([goblin], 'dm'))
    await flushPromises()
    const panel = wrapper.get('[data-testid="encounter-checks"]')
    expect(panel.text()).toContain('No checks yet this session.')
    await panel.get('[data-testid="rest-short"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'rest', rest: 'short' })
    await panel.get('[data-testid="rest-long"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'rest', rest: 'long' })
    expect(panel.get('[data-testid="run-check"]').attributes('disabled')).toBeDefined()
    await panel.get('[data-testid="check-table"]').setValue(ROAD)
    await panel.get('[data-testid="run-check"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'encounter_check', tableId: ROAD, mode: 'normal' })
    expect(s.sent.at(-1)).not.toHaveProperty('entry')
    await panel.get('[data-testid="check-mode"]').setValue('pick')
    await panel.get('[data-testid="check-entry"]').setValue(1)
    expect(panel.get('[data-testid="check-entry"]').findAll('option').map((o) => o.text())).toEqual(['Ambush', 'nothing'])
    await panel.get('[data-testid="run-check"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'encounter_check', mode: 'pick', entry: 1 })
    await panel.get('[data-testid="check-due"]').setValue('next_travel')
    await panel.get('[data-testid="schedule-check"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'schedule_check', tableId: ROAD, due: 'next_travel' })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [goblin], fog: false, visible: [], remembered: [], checks: dmChecks } })
    await flushPromises()
    const lines = wrapper.get('[data-testid="encounter-checks"]').findAll('li').map((l) => l.text())
    expect(lines[0]).toBe('Travel leg on Road · rolling… · seed 8')
    expect(lines[2]).toContain('goblin x3, ogre')
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(1)
    await expectAccessible(wrapper.element as Element)
  })

  it('shows players and the Table Display only what the check lets them see', async () => {
    const playerChecks = [
      { id: '0190c7a8-0000-7000-8000-000000000041', trigger: 'long_rest', visibility: 'secret', status: 'resolved', outcome: 'encounter' },
      { id: '0190c7a8-0000-7000-8000-000000000043', trigger: 'travel_leg', visibility: 'open', status: 'pending', chancePct: 30, rollId: ROLL },
    ]
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party'))
    await flushPromises()
    expect(wrapper.find('[data-testid="encounter-checks"]').exists()).toBe(false)
    s.receive({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [], checks: playerChecks } })
    await flushPromises()
    const panel = wrapper.get('[data-testid="encounter-checks"]')
    expect(panel.find('[data-testid="rest-long"]').exists()).toBe(false)
    expect(panel.findAll('li').map((l) => l.text())).toEqual(['Travel leg · rolling…', 'Long rest · something approaches!'])
    expect(wrapper.findAll('[data-testid="roll-card"]')).toHaveLength(0)

    const tv = await mountApp(`/campaigns/${ID}/sessions/${SID}/table`, { [`/api/v1/campaigns/${ID}/rolls/`]: (u) => roll(u.pathname.split('/')[6] ?? '') })
    const t = FakeSocket.last()
    t.receive(snapshot([], 'table', { checks: playerChecks }))
    await flushPromises()
    expect(tv.wrapper.get('[data-testid="table-check"] [data-testid="roll-card"]').text()).toContain('Encounter check')
    t.receive({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [], checks: [{ ...playerChecks[1], status: 'resolved', outcome: 'nothing', chanceRoll: 88 }] } })
    await flushPromises()
    expect(tv.wrapper.get('[data-testid="table-check"]').text()).toBe('Travel leg · rolled 88 against 30% · all quiet')
    t.receive({ kind: 'view', seq: 3, view: { tokens: [], fog: false, visible: [], remembered: [], checks: playerChecks, table: { camera: 'follow_turn', q: 0, r: 0, zoomPct: 100, scene: 'local', blackout: true } } })
    await flushPromises()
    expect(tv.wrapper.find('[data-testid="table-check"]').exists()).toBe(false)
  })
})

describe('inventory', () => {
  const STASH = '0190c7a8-0000-7000-8000-000000000061'
  const ARIA = '0190c7a8-0000-7000-8000-000000000062'
  const BROM = '0190c7a8-0000-7000-8000-000000000063'
  const DROP = '0190c7a8-0000-7000-8000-000000000064'
  const containers: LiveContainer[] = [
    { id: DROP, kind: 'loot_drop', label: 'Loot: Hoard', items: [{ slug: 'anvil', name: 'Anvil', count: 4, weightLb: 400 }], instances: [], coins: [{ coin: 'gp', count: 100 }], weightLb: 402 },
    { id: ARIA, kind: 'character', label: 'Aria', characterId: ARIA, ownerId: player.id, items: [{ slug: 'rope', name: 'Rope', count: 2, weightLb: 10 }], instances: [], coins: [], weightLb: 301.2, capacityLb: 120, encumbered: true },
    { id: BROM, kind: 'character', label: 'Brom', characterId: BROM, ownerId: member.id, items: [{ slug: 'rope', name: 'Rope', count: 1, weightLb: 5 }], instances: [], coins: [], weightLb: 5, capacityLb: 225 },
    { id: STASH, kind: 'party_stash', label: 'Party Stash', items: [], instances: [], coins: [], weightLb: 0 },
  ]
  const [drop, aria, brom, stash] = containers as [LiveContainer, LiveContainer, LiveContainer, LiveContainer]
  const lined: LiveContainer = { ...aria, instances: [{ id: DROP, slug: 'rope', name: 'Climbing Line', count: 1, charges: 3, identified: true, weightLb: 5 }] }
  const transfer = (data: string) => ({ getData: () => data, setData: () => undefined })

  it('knows who may take and put, and what a pack weighs', () => {
    expect([canTake(drop, false, player.id), canTake(aria, false, player.id), canTake(brom, false, player.id), canTake(brom, true, player.id)]).toEqual([true, true, false, true])
    expect([canPut(drop, true, player.id), canPut(stash, false, player.id), canPut(brom, false, player.id), canPut(brom, true, member.id)]).toEqual([false, true, false, true])
    expect([load(aria), load(stash), load({ ...stash, weightLb: 2.25 })]).toEqual(['301.2 / 120 lb', '0 lb', '2.3 lb'])
    const bag = (ownerId?: string): LiveContainer => ({ id: STASH, kind: 'bag', label: 'Backpack', parentId: ARIA, ownerId, items: [], instances: [], coins: [], weightLb: 0 })
    expect([canTake(bag(player.id), false, player.id), canTake(bag(member.id), false, player.id), canTake(bag(), false, player.id), canPut(bag(member.id), false, player.id)]).toEqual([
      true,
      false,
      true,
      false,
    ])
  })

  it('names Item Instances by what the viewer may know', () => {
    const one = { id: ARIA, slug: 'rope', name: 'Climbing Line', count: 1, identified: true, weightLb: 5 }
    expect([
      instanceLabel({ ...one, charges: 3, slot: 'ring_1', attuned: true }),
      instanceLabel({ ...one, charges: 1 }),
      instanceLabel({ ...one, name: 'Anvil', identified: false }),
      instanceLabel({ ...one, name: 'Rope', count: 3 }),
    ]).toEqual(['Climbing Line · 3 charges · ring 1 · attuned', 'Climbing Line · 1 charge', 'Anvil · unidentified', 'Rope ×3'])
  })

  it('lets a player take loot into their own pack by choosing or dragging', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party', { inventory: containers }))
    await flushPromises()
    const panel = wrapper.get('[data-testid="inventory"]')
    expect(panel.find('[data-testid="roll-loot"]').exists()).toBe(false)
    expect(panel.findAll('article').map((a) => a.attributes('aria-label'))).toEqual(['Party Stash', 'Aria', 'Brom', 'Loot: Hoard'])
    expect(wrapper.get('[data-testid="container-Aria"] [data-testid="encumbered"]').text()).toBe('Encumbered')
    expect(wrapper.get('[data-testid="container-Aria"] [data-testid="load"]').text()).toBe('301.2 / 120 lb')
    expect(wrapper.get('[data-testid="container-Party Stash"]').text()).toContain('Empty.')
    expect(wrapper.get('[data-testid="container-Brom"] [data-testid="private"]').text()).toBe('Private.')
    expect(wrapper.find('[data-testid="container-Aria"] [data-testid="private"]').exists()).toBe(false)
    s.receive(snapshot([], 'party', { inventory: [drop, lined, brom, stash] }))
    await flushPromises()
    const line = wrapper.get('[data-testid="container-Aria"] [data-testid="instance"]')
    expect(line.get('span').text()).toBe('Climbing Line · 3 charges · 5 lb')
    await line.get('[aria-label="Where Climbing Line goes"]').setValue(STASH)
    await line.get('[aria-label="Move Climbing Line from Aria"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_item', fromId: ARIA, toId: STASH, instanceId: DROP })
    const carried: Record<string, string> = {}
    await line.trigger('dragstart', { dataTransfer: { setData: (k: string, v: string) => (carried[k] = v), getData: () => '' } })
    await wrapper.get('[data-testid="container-Party Stash"]').trigger('drop', { dataTransfer: transfer(carried['application/json'] ?? '') })
    expect(s.sent.at(-1)).toEqual(expect.objectContaining({ kind: 'move_item', fromId: ARIA, toId: STASH, instanceId: DROP }))
    s.receive(snapshot([], 'party', { inventory: containers }))
    await flushPromises()
    expect(wrapper.find('[aria-label="Move Rope from Brom"]').exists()).toBe(false)
    const hoard = wrapper.get('[data-testid="container-Loot: Hoard"]')
    expect(hoard.get('[aria-label="Where Anvil goes"]').findAll('option').map((o) => o.text())).toEqual(['Party Stash', 'Aria'])
    await hoard.get('[aria-label="Where Anvil goes"]').setValue(ARIA)
    await hoard.get('[aria-label="How many Anvil"]').setValue(3)
    await hoard.get('[aria-label="Move Anvil from Loot: Hoard"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_item', fromId: DROP, toId: ARIA, itemSlug: 'anvil', count: 3 })
    await hoard.get('[aria-label="How many gp"]').setValue(500)
    await hoard.get('[aria-label="Move gp from Loot: Hoard"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_coins', fromId: DROP, toId: STASH, coin: 'gp', count: 100 })
    const stack = hoard.get('li')
    const dragged: Record<string, string> = {}
    await stack.trigger('dragstart', { dataTransfer: { setData: (k: string, v: string) => (dragged[k] = v), getData: () => '' } })
    const packOf = wrapper.get('[data-testid="container-Aria"]')
    await packOf.trigger('dragover', { dataTransfer: transfer('') })
    expect(packOf.classes()).toContain('over')
    await packOf.trigger('dragleave')
    await packOf.trigger('drop', { dataTransfer: transfer(dragged['application/json'] ?? '') })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_item', fromId: DROP, toId: ARIA, itemSlug: 'anvil', count: 4 })
    const sent = s.sent.length
    await wrapper.get('[data-testid="container-Brom"]').trigger('dragover', { dataTransfer: transfer('') })
    expect(wrapper.get('[data-testid="container-Brom"]').classes()).not.toContain('over')
    await wrapper.get('[data-testid="container-Brom"]').trigger('drop', { dataTransfer: transfer(dragged['application/json'] ?? '') })
    await hoard.trigger('drop', { dataTransfer: transfer(dragged['application/json'] ?? '') })
    await packOf.trigger('drop', { dataTransfer: transfer('') })
    await packOf.trigger('drop', { dataTransfer: transfer(JSON.stringify({ from: ARIA, itemSlug: 'rope', count: 2 })) })
    expect(s.sent.length).toBe(sent)
    await wrapper.get('[data-testid="container-Aria"] li').trigger('dragstart', { dataTransfer: { setData: (k: string, v: string) => (dragged[k] = v), getData: () => '' } })
    await wrapper.get('[data-testid="container-Party Stash"]').trigger('drop', { dataTransfer: transfer(dragged['application/json'] ?? '') })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_item', fromId: ARIA, toId: STASH, itemSlug: 'rope', count: 2 })
    await expectAccessible(wrapper.element as Element)
  })

  it('lets a player give from their own pack to another Character', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party', { inventory: containers }))
    await flushPromises()
    const pack = wrapper.get('[data-testid="container-Aria"]')
    expect(pack.get('[aria-label="Where Rope goes"]').findAll('option').map((o) => o.text())).toEqual(['Party Stash', 'Brom'])
    const dragged: Record<string, string> = {}
    await pack.get('li').trigger('dragstart', { dataTransfer: { setData: (k: string, v: string) => (dragged[k] = v), getData: () => '' } })
    await wrapper.get('[data-testid="container-Brom"]').trigger('drop', { dataTransfer: transfer(dragged['application/json'] ?? '') })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_item', fromId: ARIA, toId: BROM, itemSlug: 'rope', count: 2 })
  })

  it('lets players claim loot with need or greed and the DM share it out', async () => {
    const claimed: LiveContainer = {
      ...drop,
      instances: [{ id: STASH, slug: 'rope', name: 'Moonrope', count: 1, identified: true, weightLb: 5 }],
      claims: [{ characterId: ARIA, name: 'Aria', item: 'anvil', choice: 'need', roll: 14 }],
    }
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party', { inventory: [claimed, aria, brom, stash] }))
    await flushPromises()
    const hoard = wrapper.get('[data-testid="container-Loot: Hoard"]')
    expect(hoard.get('[data-testid="claim-Aria-anvil"]').text()).toBe('Aria: need 14')
    expect(hoard.find('[data-testid="claim-for"]').exists()).toBe(false)
    expect(hoard.find('[data-testid="settle-loot"]').exists()).toBe(false)
    await hoard.get('[data-testid="greed-anvil"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'claim_loot', fromId: DROP, characterId: ARIA, option: 'greed', itemSlug: 'anvil' })
    await hoard.get(`[data-testid="need-${STASH}"]`).trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'claim_loot', fromId: DROP, characterId: ARIA, option: 'need', instanceId: STASH })
    await hoard.get('[data-testid="pass-anvil"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'claim_loot', option: 'pass', itemSlug: 'anvil' })
    await expectAccessible(wrapper.element as Element)
  })

  it('lets the DM claim for any Character and share a pile out', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/loot-tables`]: () => [],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'dm', { inventory: containers }))
    await flushPromises()
    const hoard = wrapper.get('[data-testid="container-Loot: Hoard"]')
    await hoard.get('[data-testid="claim-for"]').setValue(BROM)
    await hoard.get('[data-testid="need-anvil"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'claim_loot', characterId: BROM, option: 'need', itemSlug: 'anvil' })
    await hoard.get('[data-testid="settle-loot"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'settle_loot', fromId: DROP })
  })

  it('offers no claims to a player without a Character', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party', { inventory: [drop, brom, stash] }))
    await flushPromises()
    expect(wrapper.find('[data-testid="need-anvil"]').exists()).toBe(false)
  })

  it('lets the DM roll loot, end a fight with loot, and move anything', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/loot-tables`]: () => [{ id: '0190c7a8-0000-7000-8000-000000000071', name: 'Purse', rolls: 1, entries: [], updatedAt: '2026-10-01T20:00:00Z' }],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    const combat = { status: 'active', round: 1, combatants: [] }
    s.receive(snapshot([goblin], 'dm', { inventory: containers, combat }))
    await flushPromises()
    const panel = wrapper.get('[data-testid="inventory"]')
    expect(panel.get('[data-testid="roll-loot"]').attributes('disabled')).toBeDefined()
    await panel.get('[data-testid="roll-loot-table"]').setValue('0190c7a8-0000-7000-8000-000000000071')
    await panel.get('[data-testid="roll-loot"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'roll_loot', lootTableId: '0190c7a8-0000-7000-8000-000000000071' })
    await wrapper.get('[aria-label="Move Rope from Brom"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_item', fromId: BROM, toId: STASH, itemSlug: 'rope', count: 1 })
    await wrapper.get('[data-testid="end-combat"]').trigger('click')
    expect(s.sent.at(-1)).not.toHaveProperty('lootTableId')
    await wrapper.get('[data-testid="fight-loot"]').setValue('0190c7a8-0000-7000-8000-000000000071')
    await wrapper.get('[data-testid="end-combat"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'end_combat', lootTableId: '0190c7a8-0000-7000-8000-000000000071' })
  })
})

describe('shopping', () => {
  const STORE = '0190c7a8-0000-7000-8000-000000000081'
  const ARIA = '0190c7a8-0000-7000-8000-000000000062'
  const BROM = '0190c7a8-0000-7000-8000-000000000063'
  const ARIA_PACK = '0190c7a8-0000-7000-8000-000000000072'
  const BROM_PACK = '0190c7a8-0000-7000-8000-000000000073'
  const ROLL = '0190c7a8-0000-7000-8000-000000000082'
  const packs: LiveContainer[] = [
    { id: ARIA_PACK, kind: 'character', label: 'Aria', characterId: ARIA, ownerId: player.id, items: [{ slug: 'rope', name: 'Rope', count: 2, weightLb: 10 }], instances: [], coins: [], weightLb: 10 },
    { id: BROM_PACK, kind: 'character', label: 'Brom', characterId: BROM, ownerId: member.id, items: [], instances: [], coins: [], weightLb: 0 },
    { id: '0190c7a8-0000-7000-8000-000000000061', kind: 'party_stash', label: 'Party Stash', items: [], instances: [], coins: [], weightLb: 0 },
  ]
  const open: LiveShop = {
    id: STORE, name: 'Store', kind: 'general', settlement: 'Oakford', owner: 'Tamsin',
    stock: [{ slug: 'rope', name: 'Rope', count: 3, priceCp: 150, weightLb: 5 }, { slug: 'torch', name: 'Torch', count: 10, priceCp: 1, weightLb: 1 }],
    haggles: [],
    offers: [{ characterId: ARIA, slug: 'rope', priceCp: 55 }],
  }
  const roll = (id: string) => ({
    id, purpose: 'Haggle at Store', notation: '1d20', requestedBy: 'Joris', roller: { id: player.id, name: 'Aria' }, mine: true, canRoll: true,
    status: 'pending', groups: [{ index: 0, count: 1, faces: 20, sign: 1 }], dice: [{ no: 0, group: 0, faces: 20, kept: false }], modifiers: [], createdAt: '2026-10-01T20:00:00Z',
  })
  const view = (extra: object) => ({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [], inventory: packs, gameDay: 4, ...extra } })

  it('lets the DM open a shop, trade for any Character and close it', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/shops`]: () => [{ id: STORE, settlementId: OAK, name: 'Store', kind: 'general', markupPct: 50, haggleDc: 15, hagglePct: 10, restock: 'never', stockedDay: 0, stock: [], updatedAt: '2026-10-01T20:00:00Z' }],
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'dm', { inventory: packs, gameDay: 4 }))
    await flushPromises()
    const panel = wrapper.get('[data-testid="shop-panel"]')
    expect(panel.get('[data-testid="game-day"]').text()).toBe('Day 4')
    expect(panel.get('[data-testid="open-shop"]').attributes('disabled')).toBeDefined()
    await panel.get('[data-testid="shop-choice"]').setValue(STORE)
    await panel.get('[data-testid="open-shop"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'open_shop', shopId: STORE })
    s.receive(view({ shop: open }))
    await flushPromises()
    expect(wrapper.get('[data-testid="shop-open"]').text()).toBe('Store · general in Oakford · kept by Tamsin')
    expect(wrapper.get('[data-testid="shop-buyer"]').findAll('option').map((o) => o.text())).toEqual(['Aria', 'Brom'])
    await wrapper.get('[data-testid="shop-buyer"]').setValue(BROM_PACK)
    await wrapper.get('[aria-label="How many Torch"]').setValue(4)
    await wrapper.get('[data-testid="buy-torch"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'buy', fromId: BROM_PACK, itemSlug: 'torch', count: 4 })
    await wrapper.get('[data-testid="buy-rope"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'buy', fromId: BROM_PACK, itemSlug: 'rope', count: 1 })
    expect(wrapper.get('[data-testid="trade-window"]').text()).toContain('Nothing to sell.')
    await wrapper.get('[data-testid="haggle"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'haggle', fromId: BROM_PACK })
    // A Shop of a Faction says how that Faction's Standing moves its prices; a Shop of none says nothing.
    expect(wrapper.find('[data-testid="shop-standing"]').exists()).toBe(false)
    const lines = { friendly: 'The Lantern Watch holds the party Friendly: prices are 10% lower.', hostile: 'The Lantern Watch holds the party Hostile: prices are 50% higher.', neutral: 'The Lantern Watch holds the party Neutral: prices are as listed.' }
    let n = 3
    for (const [tier, pricePct] of [['friendly', -10], ['hostile', 50], ['neutral', 0]] as const) {
      s.receive({ ...view({ shop: { ...open, standing: { faction: 'The Lantern Watch', tier, pricePct } } }), seq: n++ })
      await flushPromises()
      expect(wrapper.get('[data-testid="shop-standing"]').text()).toBe(lines[tier])
    }
    s.receive({ ...view({ shop: { ...open, stock: [] } }), seq: n })
    await flushPromises()
    expect(wrapper.get('[aria-label="Stock of Store"]').text()).toBe('Sold out.')
    await wrapper.get('[data-testid="close-shop"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'close_shop' })
    await expectAccessible(wrapper.element as Element)
  })

  it('lets a player haggle, see the price move, buy and sell for their own Character', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rolls/`]: (u) => roll(u.pathname.split('/')[6] ?? ''),
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([], 'party', { inventory: packs }))
    await flushPromises()
    expect(wrapper.find('[data-testid="shop-panel"]').exists()).toBe(false)
    s.receive(view({ shop: open }))
    await flushPromises()
    const panel = wrapper.get('[data-testid="shop-panel"]')
    expect(panel.find('[data-testid="open-shop"]').exists()).toBe(false)
    expect(panel.find('[data-testid="shop-buyer"]').exists()).toBe(false)
    expect(panel.get('[data-testid="stock-rope"] [data-testid="price"]').text()).toBe('1 gp 5 sp')
    await panel.get('[data-testid="haggle"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'haggle', fromId: ARIA_PACK })
    s.receive({ ...view({ shop: { ...open, haggles: [{ characterId: ARIA, rollId: ROLL }, { characterId: BROM, rollId: '0190c7a8-0000-7000-8000-000000000083' }] } }), seq: 3 })
    await flushPromises()
    expect(wrapper.get('[data-testid="haggle-pending"]').text()).toBe('Haggling…')
    expect(wrapper.findAll('[data-testid="shop-panel"] [data-testid="roll-card"]')).toHaveLength(1)
    s.receive({ ...view({ shop: { ...open, haggles: [{ characterId: ARIA, rollId: ROLL, adjustPct: -10 }] } }), seq: 4 })
    await flushPromises()
    expect(wrapper.get('[data-testid="haggle-result"]').text()).toBe('Haggled: 10% off')
    expect(wrapper.get('[data-testid="stock-rope"] [data-testid="price"]').text()).toBe('1 gp 3 sp 5 cp')
    expect(wrapper.findAll('[data-testid="shop-panel"] [data-testid="roll-card"]')).toHaveLength(0)
    await wrapper.get('[data-testid="buy-rope"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'buy', fromId: ARIA_PACK, itemSlug: 'rope', count: 1 })
    expect(wrapper.get('[data-testid="make-trade"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="offer-rope"]').text()).toBe('5 sp 5 cp')
    expect(wrapper.find('[data-testid="sell-junk"]').exists()).toBe(false)
    await wrapper.get('[data-testid="give-rope"]').setValue(5)
    await wrapper.get('[data-testid="want-torch"]').setValue(3)
    expect(wrapper.get('[data-testid="trade-balance"]').text()).toBe('You pay 3 cp and get 1 gp 1 sp: 1 gp 7 cp to you')
    await wrapper.get('[data-testid="want-rope"]').setValue(1)
    expect(wrapper.get('[data-testid="trade-balance"]').text()).toBe('You pay 1 gp 3 sp 8 cp and get 1 gp 1 sp: 2 sp 8 cp from your purse')
    await wrapper.get('[data-testid="make-trade"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({
      kind: 'trade', fromId: ARIA_PACK, sells: [{ itemSlug: 'rope', count: 2 }], buys: [{ itemSlug: 'rope', count: 1 }, { itemSlug: 'torch', count: 3 }],
    })
    expect(wrapper.get('[data-testid="make-trade"]').attributes('disabled')).toBeDefined()
    s.receive({ ...view({ shop: { ...open, haggles: [{ characterId: ARIA, adjustPct: 20 }] } }), seq: 5 })
    await flushPromises()
    expect(wrapper.get('[data-testid="haggle-result"]').text()).toBe('Haggled: 20% dearer')
    s.receive({ ...view({ shop: { ...open, haggles: [{ characterId: ARIA, adjustPct: 0 }] } }), seq: 6 })
    await flushPromises()
    expect(wrapper.get('[data-testid="haggle-result"]').text()).toBe('Haggled: no change')
    await expectAccessible(wrapper.element as Element)
  })
})

describe('trading', () => {
  it('sells the junk in one go and prices what the shop has not seen at the counter', async () => {
    const ARIA = '0190c7a8-0000-7000-8000-000000000062'
    const PACK = '0190c7a8-0000-7000-8000-000000000072'
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    const pack: LiveContainer = {
      id: PACK, kind: 'character', label: 'Aria', characterId: ARIA, ownerId: player.id, instances: [], coins: [], weightLb: 4,
      items: [{ slug: 'silver-ingot', name: 'Silver Ingot', count: 2, weightLb: 2 }, { slug: 'odd-stone', name: 'Odd Stone', count: 1, weightLb: 1 }],
    }
    const shop: LiveShop = { id: '0190c7a8-0000-7000-8000-000000000081', name: 'Store', kind: 'general', settlement: 'Oakford', stock: [], haggles: [], offers: [{ characterId: ARIA, slug: 'silver-ingot', priceCp: 250, junk: true }] }
    s.receive(snapshot([], 'party', { inventory: [pack], shop }))
    await flushPromises()
    expect(wrapper.get('[data-testid="offer-odd-stone"]').text()).toBe('priced at the counter')
    await wrapper.get('[data-testid="sell-junk"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'trade', fromId: PACK, sells: [{ itemSlug: 'silver-ingot', count: 2 }], buys: [] })
  })
})

describe('action log', () => {
  const entry = (seq: number, extra: object = {}) => ({
    seq, kind: 'hp_adjusted', actor: 'Joris', origin: 'mcp', client: 'prep-agent', label: 'Goblin 1', undoable: true, createdAt: '2026-10-01T20:00:00Z', ...extra,
  })

  it('lists the session\'s actions for the DM and undoes one over the socket', async () => {
    let calls = 0
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/sessions/${SID}/log`]: () => {
        calls += 1
        return [entry(12), entry(11, { kind: 'encounter_spawned', label: 'Goblin 1, Goblin 2', client: undefined, origin: 'ui', undoable: false }), entry(10, { kind: 'rest_taken', label: '' })]
      },
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    const s = FakeSocket.last()
    s.receive(snapshot([goblin], 'dm'))
    await flushPromises()
    const log = wrapper.get('[data-testid="action-log"]')
    expect(log.get('[data-testid="log-12"]').text()).toContain('#12 hp adjusted: Goblin 1 · Joris via prep-agent')
    expect(log.get('[data-testid="log-11"]').text()).toContain('encounter spawned: Goblin 1, Goblin 2 · Joris')
    expect(log.get('[data-testid="log-10"]').text()).toContain('#10 rest taken · Joris via prep-agent')
    expect(log.find('[data-testid="undo-11"]').exists()).toBe(false)
    await log.get('[data-testid="undo-12"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'undo', seq: 12 })
    const before = calls
    s.receive({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [] } })
    await flushPromises()
    expect(calls).toBeGreaterThan(before)
    await expectAccessible(wrapper.element as Element)
  })

  it('says when the log cannot be read, and players never see it', async () => {
    const dm = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/sessions/${SID}/log`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign(),
    })
    FakeSocket.last().receive(snapshot([], 'dm'))
    await flushPromises()
    expect(dm.wrapper.get('[data-testid="action-log"]').text()).toContain('The Action Log could not be read.')
    const player = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    FakeSocket.last().receive(snapshot([], 'party'))
    await flushPromises()
    expect(player.wrapper.find('[data-testid="action-log"]').exists()).toBe(false)
  })
})

describe('rest', () => {
  const ARIA = '0190c7a8-0000-7000-8000-000000000071'
  const BROM = '0190c7a8-0000-7000-8000-000000000072'
  const ROLL = '0190c7a8-0000-7000-8000-000000000073'
  const tokens: LiveToken[] = [
    { id: ARIA, label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id },
    { id: BROM, label: 'Brom', kind: 'party', q: 1, r: 0, hidden: false, darkvisionFt: 0, controllerId: member.id },
  ]
  const resters = [
    { characterId: ARIA, tokenId: ARIA, name: 'Aria', hitDie: 'd10' as const, hitDiceLeft: 2 },
    { characterId: BROM, tokenId: BROM, name: 'Brom', hitDie: 'd12' as const, hitDiceLeft: 1, rollId: ROLL },
  ]

  it('lets a player propose a rest and spend their own Hit Dice', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign('player') })
    const s = FakeSocket.last()
    s.receive(snapshot(tokens, 'party'))
    await flushPromises()
    await wrapper.get('[data-testid="propose-short"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'propose_rest', rest: 'short' })
    await wrapper.get('[data-testid="propose-long"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'propose_rest', rest: 'long' })
    s.receive(snapshot(tokens, 'party', { rest: { kind: 'short', status: 'proposed', proposedBy: player.id, agreed: [player.id], waiting: [], waitingOnDm: true, resters } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="rest-status"]').text()).toBe('Short Rest proposed. Waiting on the DM.')
    expect(wrapper.find('[data-testid="agree-rest"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="finish-rest"]').exists()).toBe(false)
    s.receive(snapshot(tokens, 'party', { rest: { kind: 'short', status: 'resting', proposedBy: player.id, agreed: [player.id, member.id], waiting: [], waitingOnDm: false, resters } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="rest-status"]').text()).toBe('Short Rest under way.')
    expect(wrapper.get('[data-testid="rester-Brom"]').text()).toContain('Rolling…')
    expect(wrapper.find('[aria-label="Spend a Hit Die for Brom"]').exists()).toBe(false)
    await wrapper.get('[aria-label="Spend a Hit Die for Aria"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'spend_hit_die', tokenId: ARIA })
    await expectAccessible(wrapper.get('[data-testid="rest"]').element)
  })

  it('offers Hit Dice in a Long Rest only when the Campaign plays with slow natural healing', async () => {
    const variant = (value: string) => [{ slug: 'slow-natural-healing', name: 'Slow natural healing', description: 'x', automated: true, value, options: [{ value: 'off', label: 'Off' }, { value: 'on', label: 'On' }] }]
    const asked = { times: 0, value: 'off' }
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
      [`/api/v1/campaigns/${ID}/rule-variants`]: () => {
        asked.times++
        return variant(asked.value)
      },
      [`/api/v1/campaigns/${ID}`]: () => campaign('player'),
    })
    const s = FakeSocket.last()
    const long = { kind: 'long', status: 'resting', proposedBy: player.id, agreed: [player.id, member.id], waiting: [], waitingOnDm: false, resters }
    s.receive(snapshot(tokens, 'party', { rest: { ...long, status: 'proposed' } }))
    await flushPromises()
    const before = asked.times
    // The DM switched it on since the page loaded: the rest starting asks again.
    asked.value = 'on'
    s.receive({ kind: 'view', seq: 2, view: { tokens, fog: false, visible: [], remembered: [], rest: long } })
    await flushPromises()
    expect(asked.times).toBeGreaterThan(before)
    await wrapper.get('[aria-label="Spend a Hit Die for Aria"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'spend_hit_die', tokenId: ARIA })
    asked.value = 'off'
    s.receive({ kind: 'view', seq: 3, view: { tokens, fog: false, visible: [], remembered: [] } })
    await flushPromises()
    s.receive({ kind: 'view', seq: 4, view: { tokens, fog: false, visible: [], remembered: [], rest: long } })
    await flushPromises()
    expect(wrapper.find('[aria-label="Spend a Hit Die for Aria"]').exists()).toBe(false)
  })

  it('lets the DM agree, finish, call off or interrupt, and never rest mid-fight', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign() })
    const s = FakeSocket.last()
    const proposed = { kind: 'long', status: 'proposed', proposedBy: player.id, agreed: [player.id], waiting: [BROM], waitingOnDm: true, resters }
    s.receive(snapshot(tokens, 'dm', { rest: proposed }))
    await flushPromises()
    expect(wrapper.get('[data-testid="rest-status"]').text()).toBe('Long Rest proposed. Waiting on the DM and 1 player.')
    await wrapper.get('[data-testid="agree-rest"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'agree_rest' })
    expect(wrapper.get('[data-testid="interrupt-rest"]').text()).toBe('Call it off')
    s.receive(snapshot(tokens, 'dm', { rest: { ...proposed, waiting: [BROM, ARIA], waitingOnDm: false } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="rest-status"]').text()).toBe('Long Rest proposed. Waiting on 2 players.')
    expect(wrapper.find('[data-testid="agree-rest"]').exists()).toBe(false)
    s.receive(snapshot(tokens, 'dm', { rest: { ...proposed, waiting: [], waitingOnDm: false } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="rest-status"]').text()).toBe('Long Rest proposed.')
    s.receive(snapshot(tokens, 'dm', { rest: { ...proposed, status: 'resting', waiting: [], waitingOnDm: false } }))
    await flushPromises()
    expect(wrapper.find('[aria-label="Spend a Hit Die for Aria"]').exists()).toBe(false)
    await wrapper.get('[data-testid="finish-rest"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'finish_rest' })
    expect(wrapper.get('[data-testid="interrupt-rest"]').text()).toBe('Interrupt')
    await wrapper.get('[data-testid="interrupt-rest"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'interrupt_rest' })
    s.receive(snapshot(tokens, 'dm', { rest: { ...proposed, kind: 'short', status: 'resting', waiting: [], waitingOnDm: false } }))
    await flushPromises()
    await wrapper.get('[aria-label="Spend a Hit Die for Aria"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'spend_hit_die', tokenId: ARIA })
    s.receive(snapshot(tokens, 'dm', { combat: { status: 'active', round: 1, combatants: [] } }))
    await flushPromises()
    expect(wrapper.get('[data-testid="rest"]').text()).toContain('Nobody rests in the middle of a fight.')
  })
})

describe('hotbar', () => {
  it('counts attacks left, opens the off-hand attack and the free object interaction', async () => {
    const { mount } = await import('@vue/test-utils')
    const Hotbar = (await import('./Hotbar.vue')).default
    const token: LiveToken = {
      ...goblin, attacks: [
        { name: 'Longsword', toHit: 6, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d8', damageBonus: 3 },
        { name: 'Dagger', toHit: 6, reachFt: 5, rangeFt: 20, longRangeFt: 60, damage: '1d4', damageBonus: 3, light: true, mastery: 'nick' },
        { name: 'Greataxe', toHit: 6, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d12', damageBonus: 3, mastery: 'cleave' },
      ],
    }
    const w = mount(Hotbar, { global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient() }]] }, props: { token, armed: null, blocked: '', attacksLeft: 1, offHand: true, interaction: true, cleave: true } })
    expect(w.get('[data-testid="mastery-1"]').text()).toBe('nick')
    expect(w.get('[data-testid="mastery-2"]').attributes('title')).toContain('second creature')
    expect(w.find('[data-testid="mastery-0"]').exists()).toBe(false)
    await w.get('[data-testid="cleave"]').trigger('click')
    expect(w.emitted('cleave')?.at(-1)).toEqual([2])
    expect(w.get('[data-testid="attacks-left"]').text()).toBe('1 attack left this action')
    expect(w.find('[data-testid="off-hand-0"]').isVisible()).toBe(false)
    await w.get('[data-testid="off-hand-1"]').trigger('click')
    expect(w.emitted('offHand')?.at(-1)).toEqual([1])
    await w.get('form.interact').trigger('submit')
    expect(w.emitted('interact')).toBeUndefined()
    await w.get('[data-testid="interact-what"]').setValue(' draws a dagger ')
    await w.get('form.interact').trigger('submit')
    expect(w.emitted('interact')?.at(-1)).toEqual(['draws a dagger'])
    await w.setProps({ attacksLeft: 2, offHand: false, interaction: false })
    expect(w.get('[data-testid="attacks-left"]').text()).toBe('2 attacks left this action')
    expect(w.find('[data-testid="off-hand-1"]').exists()).toBe(false)
    expect(w.find('form.interact').exists()).toBe(false)
  })

  it('swaps weapon sets for a Character only', async () => {
    const { mount } = await import('@vue/test-utils')
    const Hotbar = (await import('./Hotbar.vue')).default
    const w = mount(Hotbar, { global: { plugins: [[VueQueryPlugin, { queryClient: new QueryClient() }]] }, props: { token: goblin, armed: null, blocked: '' } })
    expect(w.find('[data-testid="swap-weapons"]').exists()).toBe(false)
    await w.setProps({ token: { ...goblin, kind: 'party' } })
    await w.get('[data-testid="swap-weapons"]').trigger('click')
    expect(w.emitted('swap')).toHaveLength(1)
  })
})

describe('reaction settings', () => {
  it('asks, always takes or never takes each kind of reaction', async () => {
    const { mount } = await import('@vue/test-utils')
    const ReactionSettings = (await import('./ReactionSettings.vue')).default
    const token: LiveToken = { ...goblin, label: 'Aria', reactions: [{ kind: 'opportunity_attack', mode: 'always', condition: 'target_bloodied' }] }
    const w = mount(ReactionSettings, { props: { token }, attachTo: document.body })
    expect((w.get('[data-testid="reaction-opportunity_attack"]').element as HTMLSelectElement).value).toBe('always')
    expect((w.get('[data-testid="reaction-opportunity_attack-bloodied"]').element as HTMLInputElement).checked).toBe(true)
    expect((w.get('[data-testid="reaction-shield"]').element as HTMLSelectElement).value).toBe('ask')
    await w.get('[data-testid="reaction-shield"]').setValue('always')
    expect(w.emitted('set')?.at(-1)).toEqual(['shield', 'always', ''])
    await w.get('[data-testid="reaction-opportunity_attack-bloodied"]').setValue(false)
    expect(w.emitted('set')?.at(-1)).toEqual(['opportunity_attack', 'always', ''])
    await w.get('[data-testid="reaction-readied"]').setValue('never')
    expect(w.emitted('set')?.at(-1)).toEqual(['readied', 'never', ''])
    await w.get('[data-testid="reaction-opportunity_attack"]').setValue('always')
    expect(w.emitted('set')?.at(-1)).toEqual(['opportunity_attack', 'always', 'target_bloodied'])
    expect(w.find('[data-testid="reaction-shield-bloodied"]').exists()).toBe(false)
    await expectAccessible(w.element as Element)
    w.unmount()
  })
})

describe('the fallen', () => {
  it('shows death saves and offers stabilising and revival', async () => {
    const { mount } = await import('@vue/test-utils')
    const DyingPanel = (await import('./DyingPanel.vue')).default
    const aria: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-000000000081', label: 'Aria', kind: 'party', dying: { successes: 1, failures: 2 } }
    const brom: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-000000000082', label: 'Brom', kind: 'party', dying: { successes: 0, failures: 0, dead: true } }
    const nim: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-000000000083', label: 'Nim', kind: 'party', dying: { successes: 0, failures: 0, stable: true } }
    const helper: LiveToken = { ...goblin, id: '0190c7a8-0000-7000-8000-000000000084', label: 'Mira', kind: 'party' }
    const w = mount(DyingPanel, { props: { tokens: [aria, brom, nim, helper], dm: true, helper }, attachTo: document.body })
    expect(w.get('[data-testid="dying-Aria"]').text()).toContain('Aria: 1 success, 2 failures')
    expect(w.get('[data-testid="dying-Brom"]').text()).toContain('Dead')
    expect(w.get('[data-testid="dying-Nim"]').text()).toContain('Stable')
    await w.get('[aria-label="Stabilise Aria with Medicine"]').trigger('click')
    expect(w.emitted('send')?.at(-1)).toEqual([{ kind: 'stabilise', tokenId: helper.id, targetId: aria.id, option: 'medicine' }])
    await w.get('[aria-label="Stabilise Aria with a spell"]').trigger('click')
    expect(w.emitted('send')?.at(-1)).toEqual([{ kind: 'stabilise', tokenId: helper.id, targetId: aria.id, option: 'spell' }])
    await w.get('[aria-label="Revive Brom with raise dead"]').trigger('click')
    expect(w.emitted('send')?.at(-1)).toEqual([{ kind: 'revive', targetId: brom.id, option: 'raise_dead' }])
    expect(w.find('[aria-label="Stabilise Nim with Medicine"]').exists()).toBe(false)
    await expectAccessible(w.element as Element)
    await w.setProps({ tokens: [{ ...aria, dying: { successes: 2, failures: 1 } }], dm: false, helper: null })
    expect(w.text()).toContain('2 successes, 1 failure')
    expect(w.find('button').exists()).toBe(false)
    await w.setProps({ tokens: [helper] })
    expect(w.find('[data-testid="dying"]').exists()).toBe(false)
    w.unmount()
  })

  it('marks the fallen on the roster strip', async () => {
    const { mount } = await import('@vue/test-utils')
    const RosterStrip = (await import('./RosterStrip.vue')).default
    const tokens: LiveToken[] = [
      { ...goblin, id: '0190c7a8-0000-7000-8000-000000000091', dying: { successes: 1, failures: 2 } },
      { ...goblin, id: '0190c7a8-0000-7000-8000-000000000092', dying: { successes: 0, failures: 0, stable: true } },
      { ...goblin, id: '0190c7a8-0000-7000-8000-000000000093', dying: { successes: 0, failures: 3, dead: true } },
    ]
    const c = (n: number, label: string) => ({
      id: `0190c7a8-0000-7000-8000-00000000010${String(n)}`, tokenId: tokens[n]?.id ?? '', label, kind: 'party' as const, rollId: goblin.id,
      acting: false, done: false, action: true, bonusAction: true, reaction: true, movementFt: 30, speedFt: 30,
    })
    const roster = [0, 1, 2].map((n) => ({ tokenId: tokens[n]?.id ?? '', label: 'ABC'[n] ?? '', kind: 'party' as const, acting: false, effects: [] }))
    const w = mount(RosterStrip, { props: { roster, combat: { status: 'active', round: 1, combatants: [c(0, 'A'), c(1, 'B'), c(2, 'C')] }, tokens } })
    expect(w.findAll('[data-testid="fallen"]').map((f) => f.text())).toEqual(['Dying 1✓ 2✗', 'Stable', 'Dead'])
  })
})
