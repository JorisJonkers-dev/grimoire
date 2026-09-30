/* eslint-disable vue/one-component-per-file -- each test mounts its own tiny host */
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import contract from '../../../fixtures/live-messages.json'
import { zLiveCommand, zLiveUpdate } from '@/infrastructure/api/zod.gen'
import { FakeSocket } from '@/test/fakeSocket'
import { socketUrl, useLiveSession } from './liveSession'
import { SessionState } from './sessionState'

const token = { id: '0190c7a8-0000-7000-8000-00000000000a', label: 'Goblin', kind: 'enemy', q: 1, r: 0, hidden: false, darkvisionFt: 0 }
const view = (tokens = [token]) => ({ tokens, fog: false, visible: [], remembered: [] })
const snapshot = (seq: number, tokens = [token]) => ({
  kind: 'snapshot', seq, view: view(tokens), session: { id: '0190c7a8-0000-7000-8000-00000000000b', number: 2, gridRadius: 5, audience: 'party' },
})

describe('the wire contract', () => {
  it('parses every message the server fixture holds', () => {
    for (const u of contract.updates) expect(zLiveUpdate.safeParse(u).success).toBe(true)
    for (const c of contract.commands) expect(zLiveCommand.safeParse(c).success).toBe(true)
  })
})

describe('SessionState', () => {
  it('applies updates in order and asks to resync on a gap', () => {
    const s = new SessionState()
    expect(s.apply({ kind: 'view', seq: 1, view: view() })).toBe('resync')
    expect(s.apply(snapshot(3))).toBe('applied')
    expect(s.view?.tokens[0]?.label).toBe('Goblin')
    expect(s.apply({ kind: 'view', seq: 4, view: view([{ ...token, q: 2 }]) })).toBe('applied')
    expect(s.view?.tokens[0]?.q).toBe(2)
    expect(s.apply({ kind: 'view', seq: 5 })).toBe('applied')
    expect(s.view?.tokens).toHaveLength(1)
    expect(s.apply({ kind: 'view', seq: 7, view: view([]) })).toBe('resync')
    expect(s.view?.tokens).toHaveLength(1)
    expect(s.apply({ kind: 'view', seq: 6, view: view([]) })).toBe('applied')
    expect(s.view?.tokens).toHaveLength(0)
    expect(s.apply({ kind: 'rejected', seq: 6, reason: 'Nope' })).toBe('applied')
    expect(s.rejection).toBe('Nope')
    expect(s.apply({ kind: 'rejected', seq: 6 })).toBe('applied')
    expect(s.rejection).toBe('That was not allowed.')
    expect(s.apply({ kind: 'weird' })).toBe('ignored')
    expect(s.apply({ kind: 'snapshot', seq: 8 })).toBe('applied')
    expect(s.session?.number).toBe(2)
    expect(s.view?.tokens).toHaveLength(0)
    const path = { tokenId: token.id, hexes: [{ q: 1, r: 0 }, { q: 2, r: 0 }], costFt: 5 }
    expect(s.apply({ kind: 'path', seq: 8, path })).toBe('applied')
    expect(s.path).toEqual(path)
    expect(s.apply({ kind: 'path', seq: 8 })).toBe('applied')
    expect(s.path).toBeNull()
    expect(s.apply({ kind: 'path', seq: 8, path })).toBe('applied')
    expect(s.apply({ kind: 'view', seq: 9, steps: [view()], view: view([]) })).toBe('applied')
    expect(s.steps).toHaveLength(1)
    expect(s.path).toBeNull()
    const preview = { tokenId: token.id, targetId: token.id, attackNo: 0, name: 'Bite', hitChance: 50, mode: 'normal', damageMin: 1, damageMax: 4, critMax: 8, reasons: [] }
    expect(s.apply({ kind: 'attack_preview', seq: 9, preview })).toBe('applied')
    expect(s.preview).toEqual(preview)
    expect(s.apply({ kind: 'attack_preview', seq: 9 })).toBe('applied')
    expect(s.preview).toBeNull()
    s.apply({ kind: 'attack_preview', seq: 9, preview })
    s.apply({ kind: 'rejected', seq: 9, reason: 'Out of range.' })
    expect(s.preview).toBeNull()
    s.apply({ kind: 'attack_preview', seq: 9, preview })
    s.apply({ kind: 'view', seq: 10, view: view() })
    expect(s.preview).toBeNull()
    const area = { tokenId: token.id, effect: 'fireball', name: 'Fireball', dc: 13, hexes: [{ q: 1, r: 0 }], targets: [], allies: 0 }
    expect(s.apply({ kind: 'area_preview', seq: 10, area })).toBe('applied')
    expect(s.areaPreview).toEqual(area)
    s.apply({ kind: 'rejected', seq: 10, reason: 'No.' })
    expect(s.areaPreview).toBeNull()
    s.apply({ kind: 'area_preview', seq: 10 })
    expect(s.areaPreview).toBeNull()
    s.apply({ kind: 'area_preview', seq: 10, area })
    s.apply({ kind: 'view', seq: 11, view: view() })
    expect(s.areaPreview).toBeNull()
    expect(s.apply({ kind: 'ping', seq: 11, ping: { q: 1, r: 2 } })).toBe('applied')
    expect(s.ping).toEqual({ q: 1, r: 2, n: 1 })
    s.apply({ kind: 'ping', seq: 11, ping: { q: 1, r: 2 } })
    expect(s.ping?.n).toBe(2)
    s.apply({ kind: 'ping', seq: 11 })
    expect(s.ping).toBeNull()
    expect(s.apply({ kind: 'ended', seq: 8 })).toBe('applied')
    expect(s.steps).toHaveLength(0)
    expect(s.ended).toBe(true)
  })
})

describe('useLiveSession', () => {
  beforeEach(() => {
    FakeSocket.all = []
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  function harness() {
    let api!: ReturnType<typeof useLiveSession>
    const w = mount(
      defineComponent({
        setup() {
          api = useLiveSession('c1', 's1', 'dm', (url) => new FakeSocket(url), () => 100)
          return () => h('div')
        },
      }),
    )
    return { w, api: () => api }
  }

  it('connects, resyncs on gaps, sends commands and reconnects', () => {
    const { w, api } = harness()
    const s = FakeSocket.last()
    expect(s.url).toBe('ws://localhost:3000/api/v1/campaigns/c1/sessions/s1/live?audience=dm'.replace('localhost:3000', window.location.host))
    expect(api().view.connection).toBe('connecting')
    s.open()
    expect(api().view.connection).toBe('open')
    s.receive(snapshot(1))
    expect(api().view.view?.tokens).toHaveLength(1)
    s.receive({ kind: 'view', seq: 2, steps: [{ tokens: [], fog: false, visible: [], remembered: [] }], view: { tokens: [token, token], fog: false, visible: [], remembered: [] } })
    expect(api().view.view?.tokens).toHaveLength(0)
    vi.advanceTimersByTime(250)
    expect(api().view.view?.tokens).toHaveLength(2)
    s.receive({ kind: 'view', seq: 3, steps: [{ tokens: [], fog: false, visible: [], remembered: [] }], view: { tokens: [token], fog: false, visible: [], remembered: [] } })
    s.receive({ kind: 'view', seq: 4, view: { tokens: [token, token, token], fog: false, visible: [], remembered: [] } })
    vi.advanceTimersByTime(250)
    expect(api().view.view?.tokens).toHaveLength(3)
    s.receive('not json')
    s.receive({ kind: 'view', seq: 7 })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'resync', q: 0, r: 0, hidden: false })
    api().send({ kind: 'move_token', tokenId: token.id, q: 3, r: 1 })
    expect(s.sent.at(-1)).toMatchObject({ kind: 'move_token', q: 3, r: 1, nonce: '2' })
    s.drop()
    expect(api().view.connection).toBe('reconnecting')
    vi.advanceTimersByTime(100)
    const again = FakeSocket.last()
    expect(again).not.toBe(s)
    expect(api().view.connection).toBe('reconnecting')
    again.open()
    again.receive({ kind: 'ended', seq: 1 })
    expect(api().view.connection).toBe('ended')
    again.drop()
    expect(FakeSocket.all).toHaveLength(2)
    w.unmount()
    expect(again.closed).toBe(true)
  })

  it('stops reconnecting once closed and builds secure URLs', () => {
    const { w } = harness()
    const s = FakeSocket.last()
    w.unmount()
    s.drop()
    vi.advanceTimersByTime(1000)
    expect(FakeSocket.all).toHaveLength(1)
    expect(socketUrl({ protocol: 'https:', host: 'grimoire.example' }, 'c', 's', 'table')).toBe(
      'wss://grimoire.example/api/v1/campaigns/c/sessions/s/live?audience=table',
    )
  })

  it('backs off with a default delay and default socket', () => {
    const created: string[] = []
    vi.stubGlobal(
      'WebSocket',
      class extends FakeSocket {
        constructor(url: string) {
          super(url)
          created.push(url)
        }
      },
    )
    const w = mount(
      defineComponent({
        setup() {
          useLiveSession('c', 's', 'party')
          return () => h('div')
        },
      }),
    )
    FakeSocket.last().drop()
    vi.advanceTimersByTime(499)
    expect(created).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(created).toHaveLength(2)
    w.unmount()
    vi.unstubAllGlobals()
  })
})
