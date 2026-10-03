import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ActionBars, LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { fakeClock, mountApp } from '@/test/mountApp'
import { tilesOf } from './actionBar'
import { HOLD_MS } from './motion'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const CHAR = '0190c7a8-0000-7000-8000-000000000031'
const OWNED = '0190c7a8-0000-7000-8000-000000000032'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : player, members: [dm, player],
})
const sword = { name: 'Longsword', toHit: 5, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '1d8', damageBonus: 3, damageType: 'slashing', mastery: 'sap' }
const bow = { name: 'Longbow', toHit: 4, reachFt: 0, rangeFt: 150, longRangeFt: 600, damage: '1d8', damageBonus: 2, damageType: 'piercing' }
const dagger = { name: 'Dagger', toHit: 5, reachFt: 5, rangeFt: 20, longRangeFt: 60, damage: '1d4', damageBonus: 3, damageType: 'piercing' }
const aria: LiveToken = {
  id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id,
  characterId: CHAR, ac: 16, hp: 12, hpMax: 12, attacks: [sword, bow],
}
const orc: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Orc', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0, health: 'unhurt' }
const fighter = (t: LiveToken, extra: Record<string, unknown> = {}) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
  rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting: false, done: false, action: true, bonusAction: true,
  reaction: true, movementFt: 30, speedFt: 30, ...extra,
})
const fight = (me: LiveToken, extra: Record<string, unknown> = {}) => ({
  tokens: [me, orc], fog: false, visible: [], remembered: [], combat: { status: 'active', round: 1, combatants: [fighter(me, { acting: true, ...extra }), fighter(orc)] },
})
const snapshot = (audience: string, me = aria) => ({ kind: 'snapshot', seq: 1, view: fight(me), session: { id: SID, number: 3, gridRadius: 2, audience } })

// Every tile of Aria's that a layout has not placed is put away, but for the ones named as new to it.
const rest = (bars: string[][], fresh: string[] = []) => tilesOf(aria).map((t) => t.key).filter((k) => !bars.flat().includes(k) && !fresh.includes(k))

/** The API of one Character's bars: it keeps what was last saved, and every body it was sent. */
function barsApi(saved: ActionBars = { bars: [], quick: [], stowed: [], arranged: false }) {
  const puts: ActionBars[] = []
  const store = { saved }
  return {
    puts,
    routes: {
      [`/api/v1/characters/${OWNED}/action-bars`]: async (_u: URL, req: Request) => {
        if (req.method === 'PUT') {
          store.saved = { ...((await req.clone().json()) as ActionBars), arranged: true }
          puts.push(store.saved)
        }
        return store.saved
      },
      '/api/v1/characters': () => ({
        items: [{
          id: OWNED, name: 'Aria', ruleset: 'srd-2024', species: 'human', class: 'fighter', background: 'soldier', backstory: '', hasPortrait: false,
          campaigns: [{ campaignId: ID, campaignName: 'Morvain', characterId: CHAR, level: 3, hpCurrent: 12, hpMax: 12 }],
        }],
      }),
    },
  }
}
async function open(as: 'dm' | 'player', api: ReturnType<typeof barsApi>) {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, { ...api.routes, [`/api/v1/campaigns/${ID}`]: () => campaign(as) })
  const s = FakeSocket.last()
  s.receive(snapshot(as === 'dm' ? 'dm' : 'party'))
  await flushPromises()
  return { ...mounted, s }
}
const press = async (code: string, shiftKey = false, target: EventTarget = window) => {
  target.dispatchEvent(new KeyboardEvent('keydown', { code, shiftKey, bubbles: true }))
  await flushPromises()
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('action bars', () => {
  it('shows every action in the usual order until the player arranges them, and never asks for a creature that is not theirs', async () => {
    const api = barsApi()
    const { wrapper } = await open('player', api)
    expect(wrapper.find('[data-testid="action-bars"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="area-spell"]').exists()).toBe(true)
    await wrapper.get('[data-testid="arrange-bars"]').trigger('click')
    await flushPromises()
    expect(api.puts).toHaveLength(1)
    expect(api.puts[0]?.bars.map((b) => b.length)).toEqual([10, 10])
    expect(api.puts[0]?.bars[0]?.slice(0, 3)).toEqual(['attack:Longsword', 'attack:Longbow', 'action:dash'])
    expect(api.puts[0]?.quick).toHaveLength(4)
    expect(wrapper.findAll('[data-testid="bar-1"] .tile')).toHaveLength(10)
    expect(wrapper.find('[data-testid="area-spell"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="edit-bars"]').attributes('aria-pressed')).toBe('true')

    const asDM = await open('dm', barsApi())
    expect(asDM.wrapper.find('[data-testid="hotbar-Aria"]').exists()).toBe(true)
    expect(asDM.wrapper.find('[data-testid="arrange-bars"]').exists()).toBe(false)
    expect(asDM.calls.some((u) => u.pathname.startsWith('/api/v1/characters'))).toBe(false)
  })

  it('plays tiles by tap and by the keys 1 to 0, and leaves a tile the Character lacks greyed in its place', async () => {
    const placed = [['attack:Longsword', 'attack:Glaive', 'action:dash'], ['action:study', 'spell:fireball', 'summon:find-familiar', 'move:swap', 'move:jump', 'move:throw', 'move:misty-step', 'unarmed:grapple']]
    const api = barsApi({ bars: placed, quick: ['action:dash', 'attack:Glaive'], stowed: rest(placed, ['attack:Longbow']), arranged: true })
    const { wrapper, s } = await open('player', api)
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    // Longbow is new to this layout: it joins the end of the first bar. Glaive keeps its place, greyed.
    expect(wrapper.findAll('[data-testid="bar-1"] .tile').map((t) => t.text())).toEqual(['Longsword+5 · 1d8+3 · reach 5 ftsap1', 'Glaive2', 'Dash3', 'Longbow+4 · 1d8+2 · range 150/600 ft4'])
    expect(wrapper.get('[data-testid="missing-attack:Glaive"]').attributes('disabled')).toBeDefined()
    await expectAccessible(wrapper.element as Element)

    await press('Digit3')
    expect(sent()).toMatchObject({ kind: 'take_action', action: 'dash' })
    await press('Digit1', true)
    expect(sent()).toMatchObject({ kind: 'take_action', action: 'study' })
    const before = s.sent.length
    await press('Digit2')
    await press('Digit9')
    await press('KeyA')
    const input = wrapper.get('[data-testid="area-slot"]').element
    await press('Digit3', false, input)
    window.dispatchEvent(new KeyboardEvent('keydown', { code: 'Digit3', ctrlKey: true }))
    expect(s.sent).toHaveLength(before)
    await press('Digit0')
    expect(s.sent).toHaveLength(before)

    await wrapper.get('[data-testid="attack-0"]').trigger('click')
    expect(wrapper.get('[data-testid="attack-0"]').attributes('aria-pressed')).toBe('true')
    await wrapper.get('[data-testid="spell-fireball"]').trigger('click')
    expect(wrapper.find('[data-testid="area-aiming"]').exists()).toBe(true)
    await wrapper.get('[data-hex="1,-1"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'preview_area', effect: 'fireball' })
    await wrapper.get('[data-testid="swap-weapons"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'swap_weapons' })
    for (const [tile, shown] of [['summon-find-familiar', 'summoning'], ['jump', 'jumping'], ['throw', 'throwing'], ['misty-step', 'teleporting']] as const) {
      await wrapper.get(`[data-testid="${tile}"]`).trigger('click')
      expect(wrapper.find(`[data-testid="${shown}"]`).exists()).toBe(true)
    }
    await wrapper.get('[data-testid="unarmed-grapple"]').trigger('click')

    // The quick bar sits on the map: the same tiles, the greyed one too.
    const quick = wrapper.get('[data-testid="stage"] [data-testid="quick-bar"]')
    expect(quick.findAll('button').map((b) => b.text())).toEqual(['Dash', 'Glaive'])
    await quick.get('[data-testid="quick-action-dash"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'take_action', action: 'dash' })

    // With the action spent every tile waits, and the keys with them.
    s.receive({ kind: 'view', seq: 2, view: fight({ ...aria, attacks: [sword, bow, dagger] }, { action: false }) })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="bar-1"] .tile').at(-1)?.text()).toContain('Dagger')
    expect(wrapper.get('[data-testid="action-dash"]').attributes('disabled')).toBeDefined()
    const spent = s.sent.length
    await press('Digit3')
    expect(s.sent).toHaveLength(spent)
  })

  it('lets the player move, stow, bring back and pin tiles by tap, by drag and by holding a tile', async () => {
    const placed = [['attack:Longsword', 'attack:Longbow', 'action:dash'], ['action:dodge']]
    const api = barsApi({ bars: placed, quick: ['attack:Longsword'], stowed: rest(placed), arranged: true })
    const { wrapper, s } = await open('player', api)
    const last = () => api.puts.at(-1)
    const tap = async (id: string) => {
      await wrapper.get(`[data-testid="${id}"]`).trigger('click')
      await flushPromises()
    }
    // Holding a tile starts editing; a short press does not.
    fakeClock()
    await wrapper.get('[data-testid="action-dash"]').trigger('pointerdown')
    await vi.advanceTimersByTimeAsync(HOLD_MS - 50)
    await wrapper.get('[data-testid="action-dash"]').trigger('pointerup')
    await vi.advanceTimersByTimeAsync(100)
    expect(wrapper.get('[data-testid="edit-bars"]').attributes('aria-pressed')).toBe('false')
    await wrapper.get('[data-testid="action-dash"]').trigger('pointerdown')
    await vi.advanceTimersByTimeAsync(HOLD_MS)
    vi.useRealTimers()
    expect(wrapper.get('[data-testid="edit-bars"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-testid="action-bars"]').classes()).toContain('bars--editing')
    // The tap that ends the hold does not pick the tile up, and while editing nothing is played.
    await tap('action-dash')
    expect(wrapper.get('[data-testid="action-dash"]').attributes('aria-pressed')).toBe('false')
    const quiet = s.sent.length
    await press('Digit3')
    expect(s.sent).toHaveLength(quiet)

    // Tap a tile, then where it goes.
    await tap('attack-1')
    expect(wrapper.get('[data-testid="attack-1"]').attributes('aria-pressed')).toBe('true')
    await tap('attack-0')
    expect(last()?.bars[0]).toEqual(['attack:Longbow', 'attack:Longsword', 'action:dash'])
    await tap('attack-1')
    await tap('attack-1')
    expect(wrapper.get('[data-testid="attack-1"]').attributes('aria-pressed')).toBe('false')
    await tap('action-dash')
    await tap('add-to-bar-2')
    expect(last()?.bars).toEqual([['attack:Longbow', 'attack:Longsword'], ['action:dodge', 'action:dash']])

    // Put a tile away and bring another back: + asks which, and a tap in the drawer answers.
    await tap('stow-action:dodge')
    expect(last()).toMatchObject({ bars: [['attack:Longbow', 'attack:Longsword'], ['action:dash']] })
    expect(last()?.stowed.at(-1)).toBe('action:dodge')
    await tap('add-to-bar-1')
    await tap('stowed-action:hide')
    expect(last()?.bars[0]).toEqual(['attack:Longbow', 'attack:Longsword', 'action:hide'])
    await tap('stowed-action:dodge')
    expect(wrapper.get('[data-testid="stowed-action:dodge"]').attributes('aria-pressed')).toBe('true')
    await tap('attack-0')
    expect(last()?.bars[0]).toEqual(['attack:Longbow', 'action:dodge', 'attack:Longsword', 'action:hide'])

    // Pin and unpin on the quick bar.
    await tap('pin-action:hide')
    expect(last()?.quick).toEqual(['attack:Longsword', 'action:hide'])
    await tap('pin-attack:Longsword')
    expect(last()?.quick).toEqual(['action:hide'])

    // Drag between bars and into the drawer.
    await wrapper.get('[data-testid="attack-1"]').trigger('dragstart')
    await wrapper.get('[data-testid="bar-2"]').trigger('drop')
    await flushPromises()
    expect(last()?.bars[1]).toEqual(['action:dash', 'attack:Longbow'])
    await wrapper.get('[data-testid="action-dash"]').trigger('dragstart')
    await wrapper.get('[data-testid="action-dodge"]').trigger('drop')
    await flushPromises()
    expect(last()?.bars[0]?.[0]).toBe('action:dash')
    await wrapper.get('[data-testid="action-hide"]').trigger('dragstart')
    await wrapper.get('[data-testid="bar-drawer"]').trigger('drop')
    await flushPromises()
    expect(last()?.stowed).toContain('action:hide')
    await wrapper.get('[data-testid="bar-drawer"]').trigger('drop')
    await expectAccessible(wrapper.element as Element)

    await tap('edit-bars')
    expect(wrapper.find('[data-testid="bar-drawer"]').exists()).toBe(false)
    await tap('action-dash')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'take_action', action: 'dash' })
  })
})
