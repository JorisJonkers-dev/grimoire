import type { LiveView } from '@/infrastructure/api/types.gen'
import type { Coord } from '@/shared/hex'

function middle(points: Coord[]): Coord | null {
  if (points.length === 0) return null
  const q = points.reduce((sum, p) => sum + p.q, 0) / points.length
  const r = points.reduce((sum, p) => sum + p.r, 0) / points.length
  return { q: Math.round(q), r: Math.round(r) }
}

/** The hex the Table Display's camera looks at: whoever acts now, the party, or where the DM put it. */
export function focus(view: LiveView): Coord {
  const t = view.table
  if (t?.camera === 'free') return { q: t.q, r: t.r }
  const party = view.tokens.filter((x) => x.kind === 'party')
  if (t?.camera === 'follow_turn' || !t) {
    const acting = new Set(view.combat?.combatants.filter((c) => c.acting).map((c) => c.tokenId))
    const turn = middle(view.tokens.filter((x) => acting.has(x.id)))
    if (turn) return turn
  }
  return middle(party) ?? { q: 0, r: 0 }
}
