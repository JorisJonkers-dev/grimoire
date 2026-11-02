import { afterEach, describe, expect, it, vi } from 'vitest'
import type { LiveUpdate } from '@/infrastructure/api/types.gen'
import { zLiveUpdate } from '@/infrastructure/api/zod.gen'
import { mountDemo } from '@/test/mountDemo'
import { createDemo } from './demo'
import { liveSnapshot } from './demoSocket'
import { ids } from './ids'
import type { Persona } from './role'
import { prepIds } from './seeds/prep/ids'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const live = (audience: string, session: string = ids.session) =>
  `wss://demo.invalid/api/v1/campaigns/${ids.campaign}/sessions/${session}/live${audience ? `?audience=${audience}` : ''}`

const tick = () => new Promise((resolve) => setTimeout(resolve, 0))

/** Opens a demo socket as a persona and records what it does. */
async function open(persona: Persona, url: string) {
  const demo = createDemo(persona)
  const socket = new demo.Socket(url)
  const frames: LiveUpdate[] = []
  const seen: string[] = []
  socket.onopen = () => seen.push('open')
  socket.onerror = () => seen.push('error')
  socket.onclose = () => seen.push('close')
  socket.onmessage = (ev: MessageEvent) => frames.push(JSON.parse(String(ev.data)) as LiveUpdate)
  await tick()
  return { demo, socket, frames, seen }
}

describe('the demo socket answers a live Session as the server does', () => {
  it('opens after the page has set its handlers, and sends the snapshot first', async () => {
    const { socket, frames, seen } = await open('dm', live('dm'))
    expect(seen).toEqual(['open'])
    expect(socket.readyState).toBe(WebSocket.OPEN)
    expect(frames.map((f) => f.kind)).toEqual(['snapshot'])
    expect(zLiveUpdate.safeParse(frames[0]).success).toBe(true)
    expect(frames[0]?.session).toEqual({ id: ids.session, number: 4, gridRadius: 12, audience: 'dm' })
  })

  it('shows the DM every creature with its numbers, and what only the DM uses', () => {
    const { view } = liveSnapshot(createDemo('dm').world, 'dm')
    const ghoul = view?.tokens.find((t) => t.id === prepIds.tokens.ghoul)
    expect(ghoul).toMatchObject({ label: 'Ghoul', hidden: true, hp: 22, ac: 12 })
    expect(view?.roster?.find((r) => r.tokenId === prepIds.tokens.ghoul)?.hidden).toBe(true)
    expect(view).toMatchObject({ map: { id: ids.map }, checkpoints: [], noUndo: false })
    expect(view?.roster?.slice(0, 4).every((r) => r.kind === 'party')).toBe(true)
  })

  it.each(['party', 'table'] as const)('leaves the hidden creature and the DM’s numbers out of the %s view', (audience) => {
    const update = liveSnapshot(createDemo('player').world, audience)
    expect(zLiveUpdate.safeParse(update).success).toBe(true)
    const text = JSON.stringify(update)
    expect(text).not.toContain(prepIds.tokens.ghoul)
    expect(text).not.toContain('Ghoul')
    expect(text).not.toContain('Claw')
    expect(text).not.toContain('checkpoints')
    const goblin = update.view?.tokens.find((t) => t.id === prepIds.tokens.goblin)
    expect(goblin).toMatchObject({ health: 'hurt' })
    expect(goblin?.hp).toBeUndefined()
    expect(goblin?.attacks).toBeUndefined()
    const tamsin = update.view?.tokens.find((t) => t.characterId === ids.characters[0])
    expect(tamsin).toMatchObject({ hp: 21, hpMax: 24, controllerId: ids.members[1] })
  })

  it('reads how hurt a creature looks', () => {
    const world = createDemo('player').world
    const goblin = world.prep.live.tokens.find((t) => t.id === prepIds.tokens.goblin)
    const looks = [10, 7, 5, 0].map((hp) => {
      if (goblin) goblin.hp = hp
      return liveSnapshot(world, 'party').view?.tokens.find((t) => t.id === prepIds.tokens.goblin)?.health
    })
    expect(looks).toEqual(['unhurt', 'hurt', 'bloodied', 'down'])
  })

  it('sends a view without a map when the Session has none', () => {
    const world = createDemo('dm').world
    world.prep.maps = []
    const { view } = liveSnapshot(world, 'dm')
    expect(view?.map).toBeUndefined()
    expect(view?.ambient).toBeUndefined()
  })

  it('sends the snapshot again on resync and turns down any other command', async () => {
    const { socket, frames } = await open('player', live('party'))
    socket.send(JSON.stringify({ kind: 'resync', nonce: '1' }))
    socket.send(JSON.stringify({ kind: 'move_token', nonce: '2' }))
    socket.send(JSON.stringify({ kind: 'ping' }))
    expect(frames.map((f) => [f.kind, f.nonce])).toEqual([['snapshot', undefined], ['snapshot', undefined], ['rejected', '2'], ['rejected', undefined]])
    expect(frames.every((f) => zLiveUpdate.safeParse(f).success)).toBe(true)
  })

  it.each<[string, Persona, string]>([
    ['a Player asking for the DM’s view', 'player', live('dm')],
    ['a caller signed out', 'signed-out', live('party')],
    ['a caller outside the Campaign', 'admin', live('party')],
    ['a Session the Campaign does not have', 'dm', live('dm', ids.outsider)],
    ['no audience', 'dm', live('')],
    ['an audience there is not', 'dm', live('everyone')],
    ['a path that is not a live Session', 'dm', `wss://demo.invalid/api/v1/campaigns/${ids.campaign}/live?audience=dm`],
  ])('refuses %s without opening', async (_what, persona, url) => {
    const { socket, frames, seen } = await open(persona, url)
    expect(seen).toEqual(['error', 'close'])
    expect(frames).toEqual([])
    socket.send(JSON.stringify({ kind: 'resync' }))
    expect(frames).toEqual([])
    expect(socket.readyState).toBe(WebSocket.CLOSED)
  })

  it('closes once, and never opens a socket closed while connecting', async () => {
    const demo = createDemo('dm')
    const early = new demo.Socket(live('dm'))
    const seen: string[] = []
    early.onopen = () => seen.push('open')
    early.onclose = () => seen.push('close')
    early.close()
    early.close()
    await tick()
    expect(seen).toEqual(['close'])
    const { socket, seen: later } = await open('dm', live('dm'))
    socket.close()
    await tick()
    expect(later).toEqual(['open', 'close'])
  })
})

describe('the live Session page in the demo', () => {
  it('shows the DM the creature hidden from the party', async () => {
    const { wrapper } = await mountDemo(`/campaigns/${ids.campaign}/sessions/${ids.session}`, 'dm')
    expect(wrapper.text()).toContain('Ghoul')
    expect(wrapper.text()).toContain('Goblin Warrior')
    wrapper.unmount()
  })

  it('never shows a Player the hidden creature', async () => {
    const { wrapper } = await mountDemo(`/campaigns/${ids.campaign}/sessions/${ids.session}`, 'player')
    expect(wrapper.text()).toContain('Goblin Warrior')
    expect(wrapper.text()).toContain('Tamsin Reed')
    expect(wrapper.text()).not.toContain('Ghoul')
    wrapper.unmount()
  })
})
