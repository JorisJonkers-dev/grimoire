import type { LiveSessionView, LiveToken, LiveUpdate } from '@/infrastructure/api/types.gen'
import { zLiveUpdate } from '@/infrastructure/api/zod.gen'

export type Outcome = 'applied' | 'resync' | 'ignored'

/** Applies Updates in sequence order; a gap means the client must ask for a resync. */
export class SessionState {
  seq = -1
  session: LiveSessionView | null = null
  tokens = new Map<string, LiveToken>()
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
        this.tokens = new Map((u.tokens ?? []).map((t) => [t.id, t]))
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
        if (u.kind === 'token' && u.token) this.tokens.set(u.token.id, u.token)
        if (u.kind === 'token_removed' && u.tokenId) this.tokens.delete(u.tokenId)
        return 'applied'
    }
  }
}
