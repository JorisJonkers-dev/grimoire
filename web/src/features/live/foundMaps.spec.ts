import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveWorld } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const WID = '0190c7a8-0000-7000-8000-000000000020'
const CRYPT = '0190c7a8-0000-7000-8000-00000000000d'
const CELLAR = '0190c7a8-0000-7000-8000-00000000000e'
const OAK = '0190c7a8-0000-7000-8000-000000000022'
const LAIR = '0190c7a8-0000-7000-8000-000000000023'
const KEEP = '0190c7a8-0000-7000-8000-000000000024'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Aria', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const picture = { width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, gridKind: 'hexes' as const, gridStrength: 20 }
const local = (id: string, name: string) => ({ ...picture, id, name, kind: 'local' as const, imageUrl: `/api/v1/campaigns/${ID}/maps/${id}/image`, ambient: 'bright', scaleMiles: 6, found: false })
const world = (extra: Partial<LiveWorld> = {}): LiveWorld => ({
  map: { ...picture, id: WID, name: 'Realm', imageUrl: `/api/v1/campaigns/${ID}/maps/${WID}/image?v=2`, imageVersion: 2 },
  found: false,
  revealed: [{ q: 0, r: 0 }, { q: 1, r: 0 }],
  nodes: [{ id: OAK, name: 'Oakford', q: 0, r: 0 }],
  routes: [],
  legs: [],
  ...extra,
})

async function open(as: 'dm' | 'player', w: LiveWorld) {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/maps`]: () => [local(CRYPT, 'Crypt'), local(CELLAR, 'Cellar'), { ...local(WID, 'Realm'), kind: 'world' }],
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: { tokens: [], fog: false, visible: [], remembered: [], world: w }, session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  await mounted.wrapper.get('[data-testid="scope-world"]').setValue(true)
  return { ...mounted, s }
}
const fogOf = (wrapper: Awaited<ReturnType<typeof open>>['wrapper'], hex: string) =>
  wrapper.get(`[data-hex="${hex}"]`).classes().find((c) => ['cell--lit', 'cell--remembered', 'cell--unseen'].includes(c))

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('fog on the world map', () => {
  it('is dark for a party without the world map, but for where it has been', async () => {
    const { wrapper } = await open('player', world())
    expect([fogOf(wrapper, '0,0'), fogOf(wrapper, '1,0'), fogOf(wrapper, '2,0'), fogOf(wrapper, '1,1')]).toEqual(['cell--lit', 'cell--lit', 'cell--unseen', 'cell--unseen'])
    expect(wrapper.get('[data-testid="world-fog"]').text()).toBe('The party has no map of these lands: only where it has been shows.')
    expect(wrapper.find('[data-testid="found-maps"]').exists()).toBe(false)
  })

  it('is dimmed where a party with the world map has not been, and marks the local maps it found', async () => {
    const { wrapper } = await open('player', world({ found: true, nodes: [{ id: OAK, name: 'Oakford', q: 0, r: 0 }, { id: KEEP, name: 'Keep', q: 1, r: 1, mapId: CRYPT, found: true }] }))
    expect([fogOf(wrapper, '0,0'), fogOf(wrapper, '1,0'), fogOf(wrapper, '2,0'), fogOf(wrapper, '1,1')]).toEqual(['cell--lit', 'cell--lit', 'cell--remembered', 'cell--remembered'])
    expect(wrapper.get('[data-testid="world-fog"]').text()).toBe('The party has this map: it is dimmed where the party has not been.')
    expect(wrapper.get('[data-hex="2,0"]').attributes('aria-label')).toContain('remembered')
    // A found local map shows its outline at its place; a place without one has none.
    expect(wrapper.findAll('[data-found-map]').map((e) => e.attributes('data-found-map'))).toEqual(['Keep'])
    expect(wrapper.find('[data-secret]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="found-maps"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('shows the DM every place, the secret ones marked, and lets the DM say what the party has found', async () => {
    const nodes = [
      { id: OAK, name: 'Oakford', q: 0, r: 0 },
      { id: LAIR, name: 'Lair', q: 1, r: 0, secret: true },
      { id: KEEP, name: 'Keep', q: 1, r: 1, mapId: CRYPT },
    ]
    const { wrapper, s } = await open('dm', world({ nodes }))
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    // The DM's map is never dimmed or dark: what the party has not seen is only outlined.
    expect([fogOf(wrapper, '0,0'), fogOf(wrapper, '2,0')]).toEqual(['cell--lit', 'cell--unseen'])
    expect(wrapper.get('[data-hex="2,0"]').classes()).toContain('cell--dm')
    expect(wrapper.find('[data-testid="world-fog"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-secret]').map((e) => e.attributes('data-secret'))).toEqual(['Lair'])
    expect(wrapper.get('[data-node="Lair"]').text()).toBe('Lair (secret)')
    expect(wrapper.findAll('[data-local-map]').map((e) => e.attributes('data-local-map'))).toEqual(['Keep'])
    expect(wrapper.find('[data-found-map]').exists()).toBe(false)

    const found = wrapper.get('[data-testid="found-maps"]')
    expect(found.findAll('li').map((li) => li.text())).toEqual(['Realm, the world map: not foundThe party found it', 'Crypt, at Keep: not foundThe party found it'])
    await found.get(`[data-testid="find-${WID}"]`).trigger('click')
    expect(sent()).toMatchObject({ kind: 'find_map', mapId: WID, on: true })
    await found.get(`[data-testid="find-${CRYPT}"]`).trigger('click')
    expect(sent()).toMatchObject({ kind: 'find_map', mapId: CRYPT, on: true })
    s.receive({ kind: 'view', seq: 2, view: { tokens: [], fog: false, visible: [], remembered: [], world: world({ found: true, nodes: [nodes[0], nodes[1], { ...nodes[2], found: true }] as LiveWorld['nodes'] }) } })
    await flushPromises()
    expect(found.findAll('li').map((li) => li.text())).toEqual(['Realm, the world map: foundThe party lost it', 'Crypt, at Keep: foundThe party lost it'])
    expect(wrapper.findAll('[data-found-map]').map((e) => e.attributes('data-found-map'))).toEqual(['Keep'])
    expect(fogOf(wrapper, '2,0')).toBe('cell--unseen')
    await found.get(`[data-testid="find-${WID}"]`).trigger('click')
    expect(sent()).toMatchObject({ kind: 'find_map', mapId: WID, on: false })

    // A new place can be secret, and can be where a local map lies: one no other place has.
    await wrapper.get('[data-testid="world-node-name"]').setValue('Vault')
    expect(wrapper.get('[data-testid="world-node-map"]').findAll('option').map((o) => o.text())).toEqual(['No local map', 'Cellar'])
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toEqual(expect.objectContaining({ kind: 'add_node', label: 'Vault', q: 2, r: 0 }))
    expect(sent()).not.toHaveProperty('secret')
    expect(sent()).not.toHaveProperty('mapId')
    await wrapper.get('[data-testid="world-node-secret"]').setValue(true)
    await wrapper.get('[data-testid="world-node-map"]').setValue(CELLAR)
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'add_node', label: 'Vault', q: 0, r: 1, secret: true, mapId: CELLAR })
    await expectAccessible(wrapper.element as Element)
  })
})
