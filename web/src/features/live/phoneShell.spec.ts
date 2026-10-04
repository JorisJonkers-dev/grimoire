import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'
import { DM_PAGES, PLAYER_PAGES, ZOOM_MAX, ZOOM_MIN, pageAfterSwipe, pinched } from './phoneShell'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : player, members: [dm, player],
})
const sword = { name: 'Longsword', toHit: 5, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d8', damageBonus: 3, damageType: 'slashing' }
const aria: LiveToken = {
  id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id,
  ac: 16, hp: 12, hpMax: 12, attacks: [sword],
}
const orc: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Orc', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0, ac: 13, hp: 15, hpMax: 15, attacks: [sword] }
const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
  rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
  reaction: true, movementFt: 30, speedFt: 30, ...extra,
})
const fight = (extra: Record<string, unknown> = {}) => ({
  tokens: [aria, orc], fog: false, visible: [], remembered: [], combat: { status: 'active', round: 1, combatants: [fighter(aria, { acting: true, ...extra }), fighter(orc)] },
})
async function open(as: 'dm' | 'player') {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { [`/api/v1/campaigns/${ID}`]: () => campaign(as) })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: fight(), session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  return { ...mounted, s }
}
async function point(el: { element: Element }, type: string, pointerId: number, x: number, y: number, pointerType = 'touch') {
  el.element.dispatchEvent(new PointerEvent(type, { pointerId, pointerType, clientX: x, clientY: y, bubbles: true }))
  await flushPromises()
}
const touch = point

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('phone shell', () => {
  it('knows which page a swipe leads to and how far a pinch zooms', () => {
    expect(PLAYER_PAGES.map((p) => p.key)).toEqual(['map', 'actions', 'spells', 'character', 'party'])
    expect(DM_PAGES.map((p) => p.key)).toEqual(['map', 'actions', 'table', 'tools', 'party'])
    expect(pageAfterSwipe(PLAYER_PAGES, 'map', -80, 5)).toBe('actions')
    expect(pageAfterSwipe(PLAYER_PAGES, 'actions', 80, 5)).toBe('map')
    expect(pageAfterSwipe(PLAYER_PAGES, 'map', 80, 0)).toBe('map')
    expect(pageAfterSwipe(PLAYER_PAGES, 'party', -200, 0)).toBe('party')
    expect(pageAfterSwipe(PLAYER_PAGES, 'spells', -40, 0)).toBe('spells')
    expect(pageAfterSwipe(PLAYER_PAGES, 'spells', -90, 70)).toBe('spells')
    expect(pageAfterSwipe(DM_PAGES, 'actions', -90, 0)).toBe('table')
    expect(pageAfterSwipe(DM_PAGES, 'spells', -90, 0)).toBe('spells')
    expect(pinched(1, 100, 150)).toBe(1.5)
    expect(pinched(2, 100, 400)).toBe(ZOOM_MAX)
    expect(pinched(1, 100, 10)).toBe(ZOOM_MIN)
    expect(pinched(1.5, 0, 50)).toBe(1.5)
  })

  it('gives a player pages to tap or swipe between, with their turn\'s resources always in view', async () => {
    const { wrapper, s } = await open('player')
    const dock = wrapper.get('[data-testid="dock"]')
    const tab = (key: string) => wrapper.get(`[data-testid="page-${key}"]`)
    expect(wrapper.findAll('[data-testid="phone-pages"] button').map((b) => b.text())).toEqual(['Map', 'Actions', 'Spells', 'Character', 'Party'])
    expect(tab('map').attributes('aria-current')).toBe('page')
    // On a narrow screen the roster scrolls sideways, so the keyboard can reach it.
    expect(wrapper.get('[data-testid="roster-strip"] ol').attributes()).toMatchObject({ tabindex: '0', 'aria-label': 'Creatures, in order' })
    expect(dock.classes()).toContain('dock--page-map')
    // What a page holds is marked on it: the hotbar is on Actions, the spell list on Spells.
    expect(wrapper.get('[data-testid="hotbar-Aria"]').attributes('data-page')).toBe('actions')
    expect(wrapper.get('[data-testid="spell-list"]').attributes('data-page')).toBe('spells')
    expect(wrapper.get('[data-testid="tokens"]').attributes('data-page')).toBe('party')
    for (const part of dock.element.children) for (const child of part.children) expect(child.getAttribute('data-page'), child.outerHTML.slice(0, 80)).toBeTruthy()

    await tab('spells').trigger('click')
    expect(tab('spells').attributes('aria-current')).toBe('page')
    expect(tab('map').attributes('aria-current')).toBeUndefined()
    expect(dock.classes()).toContain('dock--page-spells')
    await wrapper.get('[data-testid="spell-list"] [data-testid="list-spell-fireball"]').trigger('click')
    expect(wrapper.find('[data-testid="area-aiming"]').exists()).toBe(true)
    // Aiming needs the map: playing a spell from the list goes back to it.
    expect(dock.classes()).toContain('dock--page-map')
    await wrapper.get('[data-testid="list-slot"]').setValue(3)
    await wrapper.get('[data-testid="list-spell-fireball"]').trigger('click')
    await wrapper.get('[data-hex="1,-1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'preview_area', effect: 'fireball', slot: 3 })
    await tab('spells').trigger('click')
    await wrapper.get('[data-testid="list-summon-find-familiar"]').trigger('click')
    expect(wrapper.find('[data-testid="summoning"]').exists()).toBe(true)
    expect(dock.classes()).toContain('dock--page-map')

    // A swipe across the sheet or the bar turns the page; a drag down the sheet does not.
    await touch(dock, 'pointerdown', 1, 200, 500)
    await touch(dock, 'pointerup', 1, 110, 505)
    expect(dock.classes()).toContain('dock--page-actions')
    await touch(dock, 'pointerdown', 1, 200, 500)
    await touch(dock, 'pointerup', 1, 190, 300)
    expect(dock.classes()).toContain('dock--page-actions')
    const bar = wrapper.get('[data-testid="phone-pages"]')
    await touch(bar, 'pointerdown', 1, 100, 780)
    await touch(bar, 'pointerup', 1, 220, 780)
    expect(dock.classes()).toContain('dock--page-map')
    await touch(bar, 'pointerup', 2, 0, 780)
    // A touch that stays put on a page's button turns to it, click or no click.
    await touch(tab('party'), 'pointerdown', 5, 300, 780)
    await touch(tab('party'), 'pointerup', 5, 303, 782)
    expect(dock.classes()).toContain('dock--page-party')
    await touch(bar, 'pointerdown', 6, 100, 780)
    await touch(bar, 'pointerup', 6, 101, 780)
    expect(dock.classes()).toContain('dock--page-party')
    await tab('map').trigger('click')
    await point(dock, 'pointerdown', 3, 300, 500, 'mouse')
    await point(dock, 'pointerup', 3, 100, 500, 'mouse')
    expect(dock.classes()).toContain('dock--page-map')

    // The turn's resources float above the bar on every page.
    const chips = wrapper.get('[data-testid="resources"]')
    expect(chips.text()).toBe('ActionBonusReaction30 / 30 ft')
    s.receive({ kind: 'view', seq: 2, view: fight({ action: false, movementFt: 10 }) })
    await flushPromises()
    expect(chips.get('[data-testid="chip-action"]').classes()).toContain('chip--spent')
    expect(chips.get('[data-testid="chip-action"]').attributes('aria-label')).toBe('Action: spent')
    expect(chips.get('[data-testid="chip-reaction"]').attributes('aria-label')).toBe('Reaction: available')
    expect(chips.text()).toContain('10 / 30 ft')
    await expectAccessible(wrapper.element as Element)

  })

  it('gives the DM a remote: creatures, the Table Display and the tools, each a page, with ping and camera a tap away', async () => {
    const { wrapper, s } = await open('dm')
    const dock = wrapper.get('[data-testid="dock"]')
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    expect(wrapper.findAll('[data-testid="phone-pages"] button').map((b) => b.text())).toEqual(['Map', 'Creatures', 'Table', 'Tools', 'Party'])
    expect(wrapper.find('[data-testid="spell-list"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="dm-controls"]').attributes('data-page')).toBe('tools')
    expect(wrapper.get('[data-testid="table-remote"]').element.closest('[data-page]')?.getAttribute('data-page')).toBe('table')
    expect(wrapper.get('[data-testid="control-switcher"]').attributes('data-page')).toBe('actions')
    for (const part of dock.element.children) for (const child of part.children) expect(child.getAttribute('data-page'), child.outerHTML.slice(0, 80)).toBeTruthy()

    await wrapper.get('[data-testid="page-table"]').trigger('click')
    expect(dock.classes()).toContain('dock--page-table')
    // Ping and the camera need the map: choosing either goes there, and the next tap on a hex does it.
    await wrapper.get('[data-testid="remote-ping"]').trigger('click')
    expect(dock.classes()).toContain('dock--page-map')
    expect(wrapper.get('[data-testid="pointing"]').text()).toContain('Tap the map to ping it.')
    await wrapper.get('[data-hex="1,-1"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'ping', q: 1, r: -1 })
    await wrapper.get('[data-testid="page-table"]').trigger('click')
    await wrapper.get('[data-testid="remote-camera"]').trigger('click')
    expect(wrapper.get('[data-testid="pointing"]').text()).toContain('Tap the map to point the Table Display there.')
    await wrapper.get('[data-hex="-1,1"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'table_camera', camera: 'free', q: -1, r: 1 })
    await wrapper.get('[data-testid="stop-pointing"]').trigger('click')
    expect(wrapper.find('[data-testid="pointing"]').exists()).toBe(false)
    await wrapper.get('[data-testid="blackout-toggle"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'table_blackout', on: true })
    await touch(dock, 'pointerdown', 1, 300, 500)
    await touch(dock, 'pointerup', 1, 100, 500)
    expect(dock.classes()).toContain('dock--page-actions')
    await expectAccessible(wrapper.element as Element)
  })

  it('zooms the map by pinching or with the zoom buttons', async () => {
    const { wrapper } = await open('player')
    const stage = wrapper.get('[data-testid="stage"]')
    const zoom = () => wrapper.get('[data-testid="zoomer"]').attributes('style')
    expect(zoom()).toContain('width: 100%')
    await touch(stage, 'pointerdown', 1, 100, 300)
    await touch(stage, 'pointermove', 1, 90, 300)
    await touch(stage, 'pointermove', 1, 100, 300)
    expect(zoom()).toContain('width: 100%')
    await touch(stage, 'pointerdown', 2, 200, 300)
    await touch(stage, 'pointermove', 2, 300, 300)
    expect(zoom()).toContain('width: 200%')
    await touch(stage, 'pointermove', 1, 0, 300)
    expect(zoom()).toContain('width: 300%')
    await touch(stage, 'pointerup', 2, 300, 300)
    await touch(stage, 'pointermove', 1, 290, 300)
    expect(zoom()).toContain('width: 300%')
    await touch(stage, 'pointercancel', 1, 290, 300)
    await point(stage, 'pointerdown', 9, 0, 0, 'mouse')
    await point(stage, 'pointermove', 9, 50, 0, 'mouse')
    expect(zoom()).toContain('width: 300%')

    await wrapper.get('[data-testid="zoom-out"]').trigger('click')
    expect(zoom()).toContain('width: 250%')
    expect(wrapper.get('[data-testid="zoom-in"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="zoom-reset"]').trigger('click')
    expect(zoom()).toContain('width: 100%')
    await wrapper.get('[data-testid="zoom-out"]').trigger('click')
    expect(wrapper.get('[data-testid="zoom-out"]').attributes('disabled')).toBeDefined()
    for (let i = 0; i < 6; i++) await wrapper.get('[data-testid="zoom-in"]').trigger('click')
    expect(zoom()).toContain('width: 300%')
    expect(wrapper.get('[data-testid="zoom-in"]').attributes('disabled')).toBeDefined()
    await expectAccessible(wrapper.element as Element)
  })

  it('lays a wide screen out over the map: a bar across the top, the turn in a tray below, everything else in a drawer that folds away', async () => {
    const { wrapper } = await open('player')
    const bar = wrapper.get('[data-testid="live-bar"]')
    expect(bar.get('.brand').text()).toBe('Grimoire')
    expect(bar.get('.where').text()).toBe('Morvain · Session 3')
    expect(bar.get('h1').text()).toBe('Session 3')
    expect(bar.get('[data-testid="bar-turn"]').text()).toBe('Round 1 · Aria')
    const dock = wrapper.get('[data-testid="dock"]')
    expect([...dock.element.children].map((c) => c.getAttribute('data-testid'))).toEqual(['tray', 'drawer'])
    expect(wrapper.get('[data-testid="tray"]').find('[data-testid="hotbar-Aria"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="tray"]').find('[data-testid="your-turn"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="drawer"]').find('[data-testid="tokens"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="drawer"]').find('[data-testid="spell-list"]').exists()).toBe(true)
    // The hotbar keeps the attacks and the common actions in view; the rarer choices open under More.
    const more = wrapper.get('[data-testid="hotbar-Aria"] [data-testid="hotbar-more"]')
    expect(more.element.tagName).toBe('DETAILS')
    expect(more.get('summary').text()).toBe('More')
    expect(more.find('[data-testid="area-spell"]').exists()).toBe(true)
    expect(more.find('[data-testid="ready"]').exists()).toBe(true)
    expect(more.find('[data-testid="action-dash"]').exists()).toBe(false)
    expect(more.find('[data-testid="attack-0"]').exists()).toBe(false)

    const fold = wrapper.get('[data-testid="panels-toggle"]')
    expect(fold.attributes()).toMatchObject({ 'aria-expanded': 'true', 'aria-controls': 'live-drawer' })
    expect(wrapper.get('main').classes()).not.toContain('live--folded')
    await fold.trigger('click')
    expect(fold.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('main').classes()).toContain('live--folded')
    await fold.trigger('click')
    expect(wrapper.get('main').classes()).not.toContain('live--folded')
    await expectAccessible(wrapper.element as Element)
  })

  it('marks the DM\'s bar, and keeps the creature in hand and the tools in the drawer', async () => {
    const { wrapper } = await open('dm')
    expect(wrapper.get('[data-testid="live-bar"] .brand').text()).toBe('Grimoire · DM')
    expect(wrapper.get('[data-testid="tray"]').find('[data-testid="control-switcher"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="drawer"]').find('[data-testid="dm-controls"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="drawer"]').find('[data-testid="table-remote"]').exists()).toBe(true)
  })
})
