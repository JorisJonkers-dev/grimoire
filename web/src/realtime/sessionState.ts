import type { LiveSessionView, LiveUpdate, LiveView } from '@/infrastructure/api/types.gen'
import { zLiveUpdate } from '@/infrastructure/api/zod.gen'

export type Outcome = 'applied' | 'resync' | 'ignored'

/** Applies Updates in sequence order; a view that is not the next one means the client must resync. */
export class SessionState {
  seq = -1
  session: LiveSessionView | null = null
  view: LiveView | null = null
  ended = false
  rejection = ''

  apply(frame: unknown): Outcome {
    const parsed = zLiveUpdate.safeParse(frame)
    if (!parsed.success) return 'ignored'
    const u: LiveUpdate = parsed.data
    switch (u.kind) {
      case 'snapshot':
        this.seq = u.seq
        this.session = u.session ?? this.session
        this.view = u.view ?? this.view
        return 'applied'
      case 'rejected':
        this.rejection = u.reason ?? 'That was not allowed.'
        return 'applied'
      case 'ended':
        this.ended = true
        return 'applied'
      default:
        if (this.seq < 0 || u.seq !== this.seq + 1) return 'resync'
        this.seq = u.seq
        this.view = u.view ?? this.view
        return 'applied'
    }
  }
}
