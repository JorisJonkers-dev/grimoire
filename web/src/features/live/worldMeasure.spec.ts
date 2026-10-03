import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveWorld } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'
import { measured } from './travel'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const WID = '0190c7a8-0000-7000-8000-000000000020'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const world: LiveWorld = {
  map: { id: WID, name: 'Realm', imageUrl: `/api/v1/campaigns/${ID}/maps/${WID}/image?v=0`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, imageVersion: 0, gridKind: 'hexes', gridStrength: 20 },
  found: false,
  revealed: [{ q: 0, r: 0 }, { q: 1, r: 0 }],
  nodes: [],
  routes: [],
  legs: [],
}
const plans = (slow: number, normal: number, fast: number, days = 1) => [
  { pace: 'slow' as const, minutes: slow, days }, { pace: 'normal' as const, minutes: normal, days }, { pace: 'fast' as const, minutes: fast, days },
]

async function open(as: 'dm' | 'player') {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/maps`]: () => [],
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: { tokens: [], fog: false, visible: [], remembered: [], world }, session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  await mounted.wrapper.get('[data-testid="scope-world"]').setValue(true)
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('measuring a route', () => {
  it('says a measured route in hexes, miles and time on the road', () => {
    expect(measured({ hexes: 5, miles: 12.5, plans: plans(375, 250, 188) }, 'normal')).toBe('5 hexes · 12.5 miles · 4 h 10 min at a normal pace')
    expect(measured({ hexes: 1, miles: 1, plans: plans(30, 20, 15) }, 'fast')).toBe('1 hex · 1 mile · 15 min at a fast pace')
    expect(measured({ hexes: 40, miles: 240, plans: plans(7200, 4800, 3600, 10) }, 'slow')).toBe('40 hexes · 240 miles · 10 days (120 h on the road) at a slow pace')
    expect(measured({ hexes: 3, miles: 0.3, plans: [] }, 'normal')).toBe('3 hexes · 0.3 miles')
    expect(measured({ hexes: 7, miles: 16.66, plans: plans(500, 334, 250) }, 'normal')).toBe('7 hexes · 16.7 miles · 5 h 34 min at a normal pace')
  })

  it('lets a player mark a route on the world map and read its length at a pace they choose', async () => {
    const { wrapper, s } = await open('player')
    const sent = () => s.sent.filter((c) => (c as { kind: string }).kind === 'measure_route')
    expect(wrapper.find('[data-testid="measure"]').exists()).toBe(false)
    // Off, a tap on the map measures nothing.
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    await wrapper.get('[data-testid="measure-toggle"]').trigger('click')
    expect(wrapper.get('[data-testid="measure-toggle"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-testid="measure-hint"]').text()).toBe('Tap the map to mark the route. Each tap adds a point.')
    expect(wrapper.find('[data-testid="measure-result"]').exists()).toBe(false)

    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(sent()).toEqual([])
    expect(wrapper.get('[data-testid="measure-hint"]').text()).toBe('Tap where the route goes next.')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toHaveLength(1)
    expect(sent()[0]).toMatchObject({ kind: 'measure_route', hexes: [{ q: 0, r: 0 }, { q: 2, r: 0 }] })
    s.receive({ kind: 'route_measured', seq: 1, measure: { hexes: 2, miles: 12, plans: plans(360, 240, 180) } })
    await flushPromises()
    expect(wrapper.get('[data-testid="measure-result"]').text()).toBe('2 hexes · 12 miles · 4 h at a normal pace')
    await wrapper.get('[data-testid="measure-pace"]').setValue('fast')
    expect(wrapper.get('[data-testid="measure-result"]').text()).toBe('2 hexes · 12 miles · 3 h at a fast pace')

    // The route is drawn through its points, and the hexes say they are on it.
    await wrapper.get('[data-hex="1,1"]').trigger('click')
    expect(sent().at(-1)).toMatchObject({ hexes: [{ q: 0, r: 0 }, { q: 2, r: 0 }, { q: 1, r: 1 }] })
    expect(wrapper.get('[data-testid="measure-line"]').attributes('points')).toBe('34.6,40 173.2,40 138.6,100')
    expect(wrapper.findAll('[data-waypoint]')).toHaveLength(3)
    expect(wrapper.get('[data-hex="1,1"]').attributes('aria-label')).toContain('on the path')
    await expectAccessible(wrapper.element as Element)

    // Taking a point back measures what is left; with one point there is nothing to say.
    await wrapper.get('[data-testid="measure-undo"]').trigger('click')
    expect(sent()).toHaveLength(3)
    expect(sent().at(-1)).toMatchObject({ hexes: [{ q: 0, r: 0 }, { q: 2, r: 0 }] })
    await wrapper.get('[data-testid="measure-undo"]').trigger('click')
    expect(sent()).toHaveLength(3)
    expect(wrapper.find('[data-testid="measure-result"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="measure-line"]').exists()).toBe(false)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    await wrapper.get('[data-testid="measure-clear"]').trigger('click')
    expect(wrapper.findAll('[data-waypoint]')).toHaveLength(0)
    expect(wrapper.get('[data-testid="measure-undo"]').attributes('disabled')).toBeDefined()
    // Putting the measure away forgets the route.
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    await wrapper.get('[data-testid="measure-toggle"]').trigger('click')
    expect(wrapper.find('[data-testid="measure"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-waypoint]')).toHaveLength(0)
    await wrapper.get('[data-testid="measure-toggle"]').trigger('click')
    expect(wrapper.get('[data-testid="measure-hint"]').text()).toBe('Tap the map to mark the route. Each tap adds a point.')
  })

  it('stops at fifty points, and says so', async () => {
    const { wrapper, s } = await open('player')
    await wrapper.get('[data-testid="measure-toggle"]').trigger('click')
    for (let i = 0; i < 26; i++) {
      await wrapper.get('[data-hex="0,0"]').trigger('click')
      await wrapper.get('[data-hex="1,0"]').trigger('click')
    }
    const routes = s.sent.filter((c) => (c as { kind: string }).kind === 'measure_route') as { hexes: unknown[] }[]
    expect(routes).toHaveLength(49)
    expect(routes.at(-1)?.hexes).toHaveLength(50)
    expect(wrapper.findAll('[data-waypoint]')).toHaveLength(50)
    expect(wrapper.get('[data-testid="measure-hint"]').text()).toBe('A route has at most 50 points.')
  })

  it('measures instead of using the DM\'s map tool while the measure is out', async () => {
    const { wrapper, s } = await open('dm')
    await wrapper.get('[data-testid="world-node-name"]').setValue('Oakford')
    await wrapper.get('[data-testid="measure-toggle"]').trigger('click')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(s.sent.map((c) => (c as { kind: string }).kind)).toEqual(['measure_route'])
    await wrapper.get('[data-testid="measure-toggle"]').trigger('click')
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'add_node', label: 'Oakford', q: 0, r: 0 })
  })
})
