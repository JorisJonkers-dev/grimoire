import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LiveToken } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { FakeSocket } from '@/test/fakeSocket'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { board, hexes, initials } from './board'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const SID = '0190c7a8-0000-7000-8000-00000000000b'
const member = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = (myRole = 'dm') => ({
  id: ID, name: 'Strahd', ruleset: 'srd-2024', myRole, memberCount: 1, createdAt: '2026-09-30T20:00:00Z', me: member, members: [member],
})
const goblin: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin Boss', kind: 'enemy', q: 1, r: 0, hidden: false }
const lurker: LiveToken = { id: '0190c7a8-0000-7000-8000-00000000000c', label: 'Lurker', kind: 'enemy', q: -1, r: 0, hidden: true }
const snapshot = (tokens: unknown[], audience = 'dm') => ({
  kind: 'snapshot', seq: 1, tokens, session: { id: SID, number: 3, gridRadius: 2, audience },
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
    await wrapper.get('[data-testid="token-hidden"]').setValue(true)
    await wrapper.get('[data-hex="0,1"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'place_token', label: 'Ireena', tokenKind: 'party', q: 0, r: 1, hidden: true })
    await wrapper.get('[data-hex="0,-1"]').trigger('click')
    expect(s.sent).toHaveLength(1)
    await wrapper.get('[data-hex="1,0"]').trigger('click')
    expect(wrapper.get('[data-testid="selected-token"]').text()).toContain('Goblin Boss')
    await wrapper.get('[data-hex="2,0"]').trigger('click')
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_token', tokenId: goblin.id, q: 2, r: 0 })
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
    s.receive({ kind: 'ended', seq: 1 })
    await flushPromises()
    expect(wrapper.text()).toContain('The session has ended')
  })
})
