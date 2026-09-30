import type { LiveToken } from '@/infrastructure/api/types.gen'
import { type Coord, distance } from '@/shared/hex'
import type { GridCell } from '@/shared/map/grid'

/** Every hex within the radius of the origin. */
export function hexes(radius: number): Coord[] {
  const out: Coord[] = []
  for (let q = -radius; q <= radius; q++) {
    for (let r = -radius; r <= radius; r++) {
      if (distance({ q: 0, r: 0 }, { q, r }) <= radius) out.push({ q, r })
    }
  }
  return out
}

export function initials(label: string): string {
  return label
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p.charAt(0).toUpperCase())
    .join('')
}

/** Paints tokens and a walk preview onto the grid; hidden tokens only ever arrive for the DM. */
export function board(radius: number, tokens: LiveToken[], selected: string | null, path: Coord[] = []): GridCell[] {
  const at = new Map(tokens.map((t) => [`${String(t.q)},${String(t.r)}`, t]))
  const route = new Set(path.map((c) => `${String(c.q)},${String(c.r)}`))
  return hexes(radius).map((c) => {
    const k = `${String(c.q)},${String(c.r)}`
    const t = at.get(k)
    if (!t) return route.has(k) ? { ...c, tone: 'path', label: 'on the path' } : c
    const tone = t.hidden ? 'hidden' : t.kind === 'party' ? 'ally' : t.kind
    return { ...c, tone: t.id === selected ? 'selected' : tone, label: `${t.label}${t.hidden ? ' (hidden)' : ''}`, mark: initials(t.label) }
  })
}
