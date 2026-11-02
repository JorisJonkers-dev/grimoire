import type { LiveMap, LiveRosterEntry, LiveToken, LiveUpdate, LiveView } from '@/infrastructure/api/types.gen'
import { ids } from './ids'
import type { Viewer } from './role'
import type { DemoToken } from './seeds/prep/play'
import type { World } from './world'

export type Audience = 'dm' | 'party' | 'table'

const audiences: readonly string[] = ['dm', 'party', 'table']

// What anyone can tell by looking at a creature whose hit points they may not see.
function health(hp: number, most: number): NonNullable<LiveToken['health']> {
  if (hp <= 0) return 'down'
  if (hp * 2 <= most) return 'bloodied'
  if (hp < most) return 'hurt'
  return 'unhurt'
}

// The DM sees every creature's numbers, and everyone sees the party's; of anyone else the party and
// the Table Display see only how hurt it looks.
function tokenView(t: DemoToken, audience: Audience): LiveToken {
  const base: LiveToken = {
    id: t.id, label: t.label, kind: t.kind, q: t.q, r: t.r, hidden: t.hidden, darkvisionFt: t.darkvisionFt, effects: [],
    ...(t.characterId ? { characterId: t.characterId } : {}), ...(t.controllerId ? { controllerId: t.controllerId } : {}),
  }
  if (audience === 'dm' || t.kind === 'party') return { ...base, ac: t.ac, hp: t.hp, hpMax: t.hpMax, attacks: t.attacks }
  return { ...base, health: health(t.hp, t.hpMax) }
}

// The roster strip from what the audience already sees: the party first, then everyone else by name.
function rosterOf(tokens: LiveToken[]): LiveRosterEntry[] {
  return tokens
    .filter((t) => t.hp !== undefined || t.health !== undefined)
    .sort((a, b) => Number(b.kind === 'party') - Number(a.kind === 'party') || a.label.localeCompare(b.label))
    .map((t) => ({
      tokenId: t.id, label: t.label, kind: t.kind, hp: t.hp, hpMax: t.hpMax, health: t.health, acting: false, effects: [],
      ...(t.hidden ? { hidden: true } : {}),
    }))
}

/**
 * What the server sends a screen as it joins: the Session and the view its audience may see. This is
 * the audience boundary, as in the API: the party and the Table Display never get a hidden token,
 * and what only the DM uses is left off their view.
 */
export function liveSnapshot(world: World, audience: Audience): LiveUpdate {
  const { session, tokens } = world.prep.live
  const local = world.prep.maps.find((m) => m.id === ids.map)
  const map: LiveMap | undefined = local && {
    id: local.id, name: local.name, imageUrl: local.imageUrl, width: local.width, height: local.height, hexSizePx: local.hexSizePx,
    originX: local.originX, originY: local.originY, imageVersion: 1, gridKind: local.gridKind, gridStrength: local.gridStrength,
  }
  const shown = tokens
    .filter((t) => audience === 'dm' || !t.hidden)
    .map((t) => tokenView(t, audience))
    .sort((a, b) => a.id.localeCompare(b.id))
  const view: LiveView = {
    tokens: shown, fog: false, visible: [], remembered: [], roster: rosterOf(shown),
    gameDay: world.prep.downtime.gameDay, gameMinute: world.prep.downtime.gameMinute,
    ...(map ? { map } : {}), ...(local ? { ambient: local.ambient } : {}),
    ...(audience === 'dm' ? { zones: [], surfaceKinds: [], conditions: [], checkpoints: [], noUndo: false } : {}),
  }
  return {
    kind: 'snapshot', seq: session.seq, view,
    session: { id: session.id, number: session.number, gridRadius: session.gridRadius, audience },
  }
}

/**
 * Whether the socket may open at all, as the API decides before it upgrades: a signed-in Member, on
 * the Campaign's live Session, with an audience the caller may watch. Players may not take the DM's.
 */
export function admits(world: World, who: Viewer, url: string): Audience | null {
  const parsed = new URL(url)
  const match = /^\/api\/v1\/campaigns\/([^/]+)\/sessions\/([^/]+)\/live$/.exec(parsed.pathname)
  const audience = parsed.searchParams.get('audience') ?? ''
  if (!match || !who.role || match[1] !== world.campaign.id || match[2] !== world.prep.live.session.id) return null
  if (!audiences.includes(audience) || (audience === 'dm' && who.role !== 'dm')) return null
  return audience as Audience
}

/**
 * A WebSocket that never leaves the browser: a live Session's socket opens onto the seeded world and
 * answers with the view its audience may see. Commands other than resync are not played in the demo;
 * each is answered as rejected, which is how the page learns a command did nothing.
 */
export function createDemoSocket(world: () => World, viewer: () => Viewer): typeof WebSocket {
  class DemoSocket {
    static readonly CONNECTING = 0
    static readonly OPEN = 1
    static readonly CLOSING = 2
    static readonly CLOSED = 3
    readyState = DemoSocket.CONNECTING
    onopen: ((ev: Event) => void) | null = null
    onmessage: ((ev: MessageEvent) => void) | null = null
    onerror: ((ev: Event) => void) | null = null
    onclose: ((ev: CloseEvent) => void) | null = null
    private audience: Audience | null = null

    constructor(readonly url: string) {
      // A real socket opens after the constructor returns, once the page has set its handlers.
      setTimeout(() => {
        this.connect()
      }, 0)
    }

    private connect() {
      if (this.readyState !== DemoSocket.CONNECTING) return
      this.audience = admits(world(), viewer(), this.url)
      if (!this.audience) {
        this.readyState = DemoSocket.CLOSED
        this.onerror?.(new Event('error'))
        this.onclose?.(new CloseEvent('close', { code: 1008 }))
        return
      }
      this.readyState = DemoSocket.OPEN
      this.onopen?.(new Event('open'))
      this.deliver(liveSnapshot(world(), this.audience))
    }

    private deliver(update: LiveUpdate) {
      this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(update) }))
    }

    send(data: string): void {
      if (this.readyState !== DemoSocket.OPEN || !this.audience) return
      const command = JSON.parse(data) as { kind?: string; nonce?: string }
      if (command.kind === 'resync') {
        this.deliver(liveSnapshot(world(), this.audience))
        return
      }
      this.deliver({ kind: 'rejected', seq: world().prep.live.session.seq, ...(command.nonce ? { nonce: command.nonce } : {}), reason: 'That is not available in the demo.' })
    }

    close(): void {
      if (this.readyState === DemoSocket.CLOSED) return
      this.readyState = DemoSocket.CLOSED
      setTimeout(() => {
        this.onclose?.(new CloseEvent('close', { code: 1000 }))
      }, 0)
    }
  }
  return DemoSocket as unknown as typeof WebSocket
}
