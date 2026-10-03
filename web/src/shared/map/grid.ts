import { type Coord, type Layout, toPixel } from '@/shared/hex'

/** A hex as drawn: its tone, what it says, the token's initials, and a caption under the token, named by `captionKey`. */
export type GridCell = Coord & { tone?: string; label?: string; mark?: string; caption?: string; captionKey?: string }

/** The drawn area of a grid of hexes: every centre, padded by one hex. */
export function gridBox(cells: Coord[], size: number) {
  const layout: Layout = { size, origin: { x: 0, y: 0 } }
  const centres = cells.map((c) => toPixel(layout, c))
  const xs = centres.map((c) => c.x)
  const ys = centres.map((c) => c.y)
  const x = Math.min(...xs, 0) - size
  const y = Math.min(...ys, 0) - size
  return { x, y, w: Math.max(...xs, 0) + size - x, h: Math.max(...ys, 0) + size - y }
}
