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
    this.onmessage?.(new MessageEvent('message', { data: typeof frame === 'string' ? frame : JSON.stringify(frame) }))
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
