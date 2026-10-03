import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'
import { after, board, stacks } from './board'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const dm = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const player = { id: '0190c7a8-0000-7000-8000-000000000005', displayName: 'Tamsin', role: 'player', joinedAt: '2026-09-30T20:00:00Z', isMe: false }
const campaign = (as: 'dm' | 'player') => ({
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: as, memberCount: 2, createdAt: '2026-09-30T20:00:00Z',
  me: as === 'dm' ? { ...dm, isMe: true } : { ...player, isMe: true }, members: [dm, player],
})
const bite = { name: 'Bite', toHit: 4, reachFt: 5, rangeFt: 0, longRangeFt: 0, damage: '2d4', damageBonus: 2, damageType: 'piercing' }
const aria: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000e', label: 'Aria', kind: 'party', q: 0, r: 0, hidden: false, darkvisionFt: 0, controllerId: player.id, ac: 16, hp: 12, hpMax: 12, attacks: [{ ...bite, name: 'Longsword' }] }
const steed: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Steed', kind: 'party', q: 1, r: 0, hidden: false, darkvisionFt: 0, ac: 13, hp: 11, hpMax: 11, attacks: [bite] }
const goblin: LiveToken = { ...steed, id: '0190c7a8-0000-7000-8000-00000000000c', label: 'Goblin', kind: 'enemy', q: -1, r: 0, hp: 7, hpMax: 7 }
// mounted puts Aria on the Steed, controlling it or not.
const mounted = (controlled: boolean): LiveToken[] => [
  { ...steed, riderId: aria.id },
  { ...aria, q: 1, mountId: steed.id, ...(controlled ? { mountControlled: true } : {}) },
  goblin,
]
const fighter = (t: LiveToken, acting: boolean) => ({
  id: `${t.id.slice(0, -3)}1${t.id.slice(-2)}`, tokenId: t.id, label: t.label, kind: t.kind, controllerId: t.controllerId,
  rollId: `${t.id.slice(0, -3)}2${t.id.slice(-2)}`, initiative: 10, rank: 1, acting, done: false, action: true, bonusAction: true,
  reaction: true, movementFt: 30, speedFt: 30,
})
const view = (tokens: LiveToken[], acting: string[] = []) => ({
  tokens, fog: false, visible: [], remembered: [],
  ...(acting.length ? { combat: { status: 'active', round: 1, combatants: tokens.map((t) => fighter(t, acting.includes(t.id))) } } : {}),
})
async function open(as: 'dm' | 'player', tokens: LiveToken[], acting: string[] = []) {
  const mounted = await mountApp(`/campaigns/${ID}/sessions/${SID}`, {
    [`/api/v1/campaigns/${ID}/sessions/${SID}/log`]: () => [],
    [`/api/v1/campaigns/${ID}/characters`]: () => [],
    [`/api/v1/campaigns/${ID}`]: () => campaign(as),
  })
  const s = FakeSocket.last()
  s.receive({ kind: 'snapshot', seq: 1, view: view(tokens, acting), session: { id: SID, number: 3, gridRadius: 2, audience: as === 'dm' ? 'dm' : 'party' } })
  await flushPromises()
  return { ...mounted, s }
}

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
})

describe('a rider and its mount on one hex', () => {
  it('shows the rider on top, whatever order the tokens come in, and says what it rides', () => {
    for (const tokens of [mounted(true), mounted(true).reverse()]) {
      expect(stacks(tokens).get('1,0')?.map((t) => t.label)).toEqual(['Aria', 'Steed'])
      const cell = board(2, tokens, null).find((c) => c.q === 1 && c.r === 0)
      expect(cell).toMatchObject({ mark: 'A', tone: 'ally', label: 'Aria (12/12 HP), riding Steed (11/11 HP)' })
    }
    // Tokens that merely share a hex are named side by side, and one alone as before.
    expect(board(2, [goblin, { ...steed, q: -1 }], null).find((c) => c.q === -1 && c.r === 0)?.label).toBe('Goblin (7/7 HP), with Steed (11/11 HP)')
    expect(board(2, [goblin], null).find((c) => c.q === -1 && c.r === 0)?.label).toBe('Goblin (7/7 HP)')
    // The under token is the selected one: the hex shows as selected.
    expect(board(2, mounted(true), steed.id).find((c) => c.q === 1 && c.r === 0)?.tone).toBe('selected')
  })

  it('steps through the tokens of a hex: the rider, its mount, then none', () => {
    const at = stacks(mounted(false))
    expect(after(at, { q: 1, r: 0 }, undefined)?.label).toBe('Aria')
    expect(after(at, { q: 1, r: 0 }, aria.id)?.label).toBe('Steed')
    expect(after(at, { q: 1, r: 0 }, steed.id)).toBeUndefined()
    expect(after(at, { q: 1, r: 0 }, goblin.id)?.label).toBe('Aria')
    expect(after(at, { q: 2, r: 0 }, undefined)).toBeUndefined()
  })
})

describe('riding in a live Session', () => {
  it('gets a Character onto a creature, controlling it unless told otherwise', async () => {
    const { wrapper, s } = await open('player', [aria, steed, goblin], [aria.id])
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    const bar = wrapper.get('[data-testid="hotbar-Aria"]')
    expect(bar.get('[data-testid="ride"]').text()).toBe('Mount')
    expect(wrapper.find('[data-testid="riding"]').exists()).toBe(false)
    await bar.get('[data-testid="ride"]').trigger('click')
    expect(wrapper.get('[data-testid="riding"]').text()).toContain('Tap the creature to ride.')
    expect((wrapper.get('[data-testid="ride-control"]').element as HTMLInputElement).checked).toBe(true)
    await expectAccessible(wrapper.element as Element)
    // Her own hex is nothing to ride: the choice stays open.
    const before = s.sent.length
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent).toHaveLength(before)
    expect(wrapper.find('[data-testid="riding"]').exists()).toBe(true)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'mount', tokenId: aria.id, targetId: steed.id, controlled: true })
    expect(wrapper.find('[data-testid="riding"]').exists()).toBe(false)

    // Left to act for itself, the mount is not controlled.
    await bar.get('[data-testid="ride"]').trigger('click')
    await wrapper.get('[data-testid="ride-control"]').setValue(false)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'mount', tokenId: aria.id, targetId: steed.id, controlled: false })
  })

  it('gets a rider off onto the hex tapped, and walks a controlled mount in its rider\'s place', async () => {
    const { wrapper, s } = await open('player', mounted(true), [aria.id, steed.id])
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    expect(wrapper.get('[data-hex="1,0"]').attributes('aria-label')).toContain('Aria (12/12 HP), riding Steed (11/11 HP)')
    // The Steed she controls is hers to play on her turn.
    expect(wrapper.find('[data-testid="hotbar-Steed"]').exists()).toBe(true)
    // A tap on the board walks the Steed, not its rider.
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'plan_walk', tokenId: steed.id, q: 2, r: 0 })

    const bar = wrapper.get('[data-testid="hotbar-Aria"]')
    expect(bar.get('[data-testid="ride"]').text()).toBe('Dismount')
    await bar.get('[data-testid="ride"]').trigger('click')
    expect(wrapper.get('[data-testid="riding"]').text()).toBe('Tap a free hex next to your mount.')
    expect(wrapper.find('[data-testid="ride-control"]').exists()).toBe(false)
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'dismount', tokenId: aria.id, q: 2, r: 0 })
    expect(wrapper.find('[data-testid="riding"]').exists()).toBe(false)
  })

  it('draws the mount under its rider on a map, and marks the hex when either is in hand', async () => {
    const MID = '0190c7a8-0000-7000-8000-000000000002'
    const map = { id: MID, name: 'Crypt', imageUrl: `/api/v1/campaigns/${ID}/maps/${MID}/image?v=0`, width: 200, height: 160, hexSizePx: 40, originX: 34.64, originY: 40, imageVersion: 0, gridKind: 'hexes' as const, gridStrength: 20 }
    const { wrapper, s } = await open('dm', [aria, steed, goblin])
    s.receive({ kind: 'view', seq: 2, view: { ...view(mounted(false)), map, fog: false } })
    await flushPromises()
    const hex = wrapper.get('[data-hex="1,0"]')
    expect(hex.attributes('aria-label')).toBe('Hex 1, 0: Aria (12/12 HP), riding Steed (11/11 HP)')
    expect(hex.findAll('circle.token').map((c) => c.classes().includes('token--under'))).toEqual([true, false])
    expect(hex.get('.mark').text()).toBe('A')
    expect(wrapper.get('[data-hex="0,0"]').attributes('aria-label')).toBe('Hex 0, 0')
    expect(wrapper.findAll('[data-testid="token-under"]')).toHaveLength(1)
    for (const picked of [true, true, false]) {
      await hex.trigger('click')
      expect(hex.classes().includes('cell--selected')).toBe(picked)
    }
  })

  it('lets the DM put the creature in hand on a mount, and take it off, outside a fight', async () => {
    const crate: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000d', label: 'Crate', kind: 'object', q: 0, r: 1, hidden: false, darkvisionFt: 0 }
    const { wrapper, s } = await open('dm', [aria, steed, goblin, crate])
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    expect(wrapper.find('[data-testid="ride-token"]').exists()).toBe(false)
    // A crate rides nothing.
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(wrapper.find('[data-testid="selected-token"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="ride-token"]').exists()).toBe(false)
    await wrapper.get('[data-hex="0,0"]').trigger('click')
    expect(wrapper.get('[data-testid="ride-token"]').text()).toBe('Mount')
    await wrapper.get('[data-testid="ride-token"]').trigger('click')
    expect(wrapper.get('[data-testid="riding"]').text()).toContain('Tap the creature to ride.')
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'mount', tokenId: aria.id, targetId: steed.id, controlled: true })

    s.receive({ kind: 'view', seq: 2, view: view(mounted(true)) })
    await flushPromises()
    expect(wrapper.get('[data-testid="ride-token"]').text()).toBe('Dismount')
    await wrapper.get('[data-testid="ride-token"]').trigger('click')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'dismount', tokenId: aria.id, q: 2, r: 0 })
  })

  it('leaves an independent mount to the DM', async () => {
    const { wrapper, s } = await open('player', mounted(false), [aria.id, steed.id])
    expect(wrapper.find('[data-testid="hotbar-Steed"]').exists()).toBe(false)
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'plan_walk', tokenId: aria.id })
  })

  it('lets the DM take the rider, then its mount, in hand, and aim at either', async () => {
    const { wrapper, s } = await open('dm', mounted(false), [goblin.id])
    const sent = () => s.sent.at(-1) as Record<string, unknown>
    const hex = wrapper.get('[data-hex="1,0"]')
    const selected = () => hex.classes().includes('hex--selected')
    await hex.trigger('click')
    expect(selected()).toBe(true)
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'plan_walk', tokenId: aria.id })
    await hex.trigger('click')
    expect(selected()).toBe(true)
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(sent()).toMatchObject({ kind: 'plan_walk', tokenId: steed.id })
    await hex.trigger('click')
    expect(selected()).toBe(false)

    // Aiming: the first tap is at the rider; once that is what the preview shows, the next is at the mount.
    await wrapper.get('[data-testid="hotbar-Goblin"] [data-testid="attack-0"]').trigger('click')
    await hex.trigger('click')
    expect(sent()).toMatchObject({ kind: 'preview_attack', tokenId: goblin.id, targetId: aria.id })
    s.receive({ kind: 'attack_preview', seq: 1, preview: { tokenId: goblin.id, attackNo: 0, targetId: aria.id, name: 'Bite', hitChance: 50, mode: 'normal', damageMin: 4, damageMax: 10, critMax: 18, reasons: [] } })
    await flushPromises()
    await hex.trigger('click')
    expect(sent()).toMatchObject({ kind: 'preview_attack', tokenId: goblin.id, targetId: steed.id })
    // And once it shows the mount, the tap after that is at the rider again.
    s.receive({ kind: 'attack_preview', seq: 1, preview: { tokenId: goblin.id, attackNo: 0, targetId: steed.id, name: 'Bite', hitChance: 50, mode: 'normal', damageMin: 4, damageMax: 10, critMax: 18, reasons: [] } })
    await flushPromises()
    await hex.trigger('click')
    expect(sent()).toMatchObject({ kind: 'preview_attack', tokenId: goblin.id, targetId: aria.id })
  })
})
