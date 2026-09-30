import { onBeforeUnmount, reactive } from 'vue'
import type { LiveCommand, LiveSessionView, LiveToken } from '@/infrastructure/api/types.gen'
import { SessionState } from './sessionState'

export type Audience = 'dm' | 'party' | 'table'
export type Connection = 'connecting' | 'open' | 'reconnecting' | 'ended'

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

export type LiveView = {
  connection: Connection
  session: LiveSessionView | null
  tokens: LiveToken[]
  rejection: string
}

/** One live Session socket: applies Updates by sequence, resyncs on a gap, reconnects with backoff. */
export function useLiveSession(
  campaignId: string,
  sessionId: string,
  audience: Audience,
  open: SocketFactory = (url) => new WebSocket(url),
  delay: (attempt: number) => number = (n) => Math.min(10_000, 500 * 2 ** n),
) {
  const state = new SessionState()
  const view = reactive<LiveView>({ connection: 'connecting', session: null, tokens: [], rejection: '' })
  let socket: Socket | null = null
  let attempt = 0
  let stopped = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let nonce = 0

  const sync = () => {
    view.session = state.session
    view.tokens = [...state.tokens.values()]
    view.rejection = state.rejection
    if (state.ended) view.connection = 'ended'
  }
  function send(cmd: Omit<LiveCommand, 'nonce' | 'q' | 'r' | 'hidden'> & Partial<Pick<LiveCommand, 'q' | 'r' | 'hidden'>>) {
    nonce++
    state.rejection = ''
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
    socket?.close()
  }
  connect()
  onBeforeUnmount(close)
  return { view, send, close }
}
