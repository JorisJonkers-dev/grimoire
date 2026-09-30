import { getCurrentInstance, onBeforeUnmount, reactive } from 'vue'
import type { LiveAreaPreview, LiveAttackPreview, LiveCommand, LivePath, LiveSessionView, LiveView } from '@/infrastructure/api/types.gen'
import { SessionState } from './sessionState'

export type Audience = 'dm' | 'party' | 'table'
export type Connection = 'connecting' | 'open' | 'reconnecting' | 'ended'

// A command as a page sends it; the session fills in the nonce and any missing coordinates.
export type Outgoing = Omit<LiveCommand, 'nonce' | 'q' | 'r' | 'hidden'> & Partial<Pick<LiveCommand, 'q' | 'r' | 'hidden'>>

type Socket = Pick<WebSocket, 'send' | 'close'> & {
  onopen: ((ev: Event) => void) | null
  onmessage: ((ev: MessageEvent) => void) | null
  onclose: ((ev: CloseEvent) => void) | null
}
export type SocketFactory = (url: string) => Socket

export function socketUrl(loc: Pick<Location, 'protocol' | 'host'>, campaignId: string, sessionId: string, audience: Audience): string {
  const scheme = loc.protocol === 'https:' ? 'wss' : 'ws'
  return `${scheme}://${loc.host}/api/v1/campaigns/${campaignId}/sessions/${sessionId}/live?audience=${audience}`
}

export type LiveState = {
  connection: Connection
  session: LiveSessionView | null
  view: LiveView | null
  path: LivePath | null
  preview: LiveAttackPreview | null
  areaPreview: LiveAreaPreview | null
  ping: { q: number; r: number; n: number } | null
  rejection: string
}

/** How long a token takes to walk one hex on screen. */
export const STEP_MS = 250

/** One live Session socket: applies Updates by sequence, resyncs on a gap, reconnects with backoff. */
export function useLiveSession(
  campaignId: string,
  sessionId: string,
  audience: Audience,
  open: SocketFactory = (url) => new WebSocket(url),
  delay: (attempt: number) => number = (n) => Math.min(10_000, 500 * 2 ** n),
) {
  const state = new SessionState()
  const view = reactive<LiveState>({ connection: 'connecting', session: null, view: null, path: null, preview: null, areaPreview: null, ping: null, rejection: '' })
  let socket: Socket | null = null
  let attempt = 0
  let stopped = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let nonce = 0
  let pace: ReturnType<typeof setTimeout> | undefined

  // A walk plays its steps one hex at a time; anything newer cuts the walk short.
  const play = (frames: LiveView[]) => {
    clearTimeout(pace)
    const [first, ...rest] = frames
    view.view = first ?? null
    if (rest.length > 0) pace = setTimeout(() => { play(rest) }, STEP_MS)
  }
  const sync = () => {
    view.session = state.session
    play([...state.steps, ...(state.view ? [state.view] : [])])
    view.path = state.path
    view.preview = state.preview
    view.areaPreview = state.areaPreview
    view.ping = state.ping
    view.rejection = state.rejection
    if (state.ended) view.connection = 'ended'
  }
  function send(cmd: Outgoing) {
    nonce++
    state.rejection = ''
    view.rejection = ''
    socket?.send(JSON.stringify({ q: 0, r: 0, hidden: false, ...cmd, nonce: String(nonce) }))
  }
  function connect() {
    view.connection = attempt === 0 ? 'connecting' : 'reconnecting'
    const s = open(socketUrl(window.location, campaignId, sessionId, audience))
    socket = s
    s.onopen = () => {
      attempt = 0
      view.connection = 'open'
    }
    s.onmessage = (ev) => {
      let frame: unknown
      try {
        frame = JSON.parse(String(ev.data))
      } catch {
        return
      }
      if (state.apply(frame) === 'resync') send({ kind: 'resync' })
      sync()
    }
    s.onclose = () => {
      if (stopped || state.ended) {
        sync()
        return
      }
      view.connection = 'reconnecting'
      timer = setTimeout(connect, delay(attempt++))
    }
  }
  function close() {
    stopped = true
    clearTimeout(timer)
    clearTimeout(pace)
    socket?.close()
  }
  connect()
  // Only a component's setup can tie the socket to its lifetime; anyone else must call close.
  if (getCurrentInstance()) onBeforeUnmount(close)
  return { view, send, close }
}
