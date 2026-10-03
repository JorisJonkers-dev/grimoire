/** A WebSocket stand-in the tests drive by hand. */
export class FakeSocket {
  static all: FakeSocket[] = []
  sent: unknown[] = []
  closed = false
  onopen: ((ev: Event) => void) | null = null
  onmessage: ((ev: MessageEvent) => void) | null = null
  onclose: ((ev: CloseEvent) => void) | null = null

  constructor(public url: string) {
    FakeSocket.all.push(this)
  }

  send(data: string) {
    this.sent.push(JSON.parse(data))
  }

  close() {
    this.closed = true
  }

  open() {
    this.onopen?.(new Event('open'))
  }

  receive(frame: unknown) {
    this.onmessage?.(new MessageEvent('message', { data: typeof frame === 'string' ? frame : JSON.stringify(withRoster(frame)) }))
  }

  drop() {
    this.onclose?.(new CloseEvent('close'))
  }

  static last(): FakeSocket {
    const s = FakeSocket.all.at(-1)
    if (!s) throw new Error('no socket opened')
    return s
  }
}

type Seen = { id: string; label: string; kind: string; hp?: number; hpMax?: number; tempHp?: number; health?: string; hidden?: boolean; effects?: unknown[] }
type Fighter = { tokenId: string; label: string; kind: string; acting?: boolean }
type Frame = { view?: { tokens?: Seen[]; combat?: { combatants: Fighter[] }; roster?: unknown } }

/**
 * The roster strip the server would send with a view a test builds by hand: the fight in initiative
 * order, or every creature with stats, the party first. A combatant the fixture gives no token for
 * still shows, by its label.
 */
function withRoster(frame: unknown): unknown {
  const f = frame as Frame
  const view = f.view
  if (typeof frame !== 'object' || !view || view.roster !== undefined) return frame
  const tokens = view.tokens ?? []
  const entry = (t: Seen, acting: boolean) => ({
    tokenId: t.id, label: t.label, kind: t.kind, hp: t.hp, hpMax: t.hpMax, tempHp: t.tempHp, health: t.health,
    hidden: t.hidden || undefined, acting, effects: t.effects ?? [],
  })
  const roster = view.combat
    ? view.combat.combatants.map((c) => entry({ ...tokens.find((t) => t.id === c.tokenId), id: c.tokenId, label: c.label, kind: c.kind }, c.acting ?? false))
    : tokens
        .filter((t) => t.hp !== undefined || t.health !== undefined)
        .sort((a, b) => Number(b.kind === 'party') - Number(a.kind === 'party') || a.label.localeCompare(b.label))
        .map((t) => entry(t, false))
  return { ...f, view: { ...view, roster } }
}
