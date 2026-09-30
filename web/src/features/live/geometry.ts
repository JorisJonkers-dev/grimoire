import type { LiveMap } from '@/infrastructure/api/types.gen'
import { type Coord, fromPixel, type Layout, toPixel } from '@/shared/hex'

export const layoutOf = (m: Pick<LiveMap, 'hexSizePx' | 'originX' | 'originY'>): Layout => ({
  size: m.hexSizePx,
  origin: { x: m.originX, y: m.originY },
})

/** Every hex whose centre lies inside the picture; mirrors the server's list of a map's hexes. */
export function cellsFor(l: Layout, width: number, height: number): Coord[] {
  const corners = [
    fromPixel(l, { x: 0, y: 0 }),
    fromPixel(l, { x: width, y: 0 }),
    fromPixel(l, { x: 0, y: height }),
    fromPixel(l, { x: width, y: height }),
  ]
  const qs = corners.map((c) => c.q)
  const rs = corners.map((c) => c.r)
  const out: Coord[] = []
  for (let q = Math.min(...qs) - 1; q <= Math.max(...qs) + 1; q++) {
    for (let r = Math.min(...rs) - 1; r <= Math.max(...rs) + 1; r++) {
      const p = toPixel(l, { q, r })
      if (p.x >= 0 && p.y >= 0 && p.x < width && p.y < height) out.push({ q, r })
    }
  }
  return out
}

export const key = (c: Coord) => `${String(c.q)},${String(c.r)}`
