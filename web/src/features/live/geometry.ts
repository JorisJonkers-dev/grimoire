import type { LiveMap } from '@/infrastructure/api/types.gen'
import { type Coord, fromPixel, type Layout, type Point, toPixel } from '@/shared/hex'

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

/** How far apart neighbouring cells are, centre to centre; a grid drawn as squares has squares this wide. */
export const across = (size: number) => size * Math.sqrt(3)

/** The distance between two points on a picture, in cells of a grid of that size. */
export const cellsBetween = (size: number, a: Point, b: Point) => Math.hypot(b.x - a.x, b.y - a.y) / across(size)

/** The lines of a square grid over a picture, as an SVG path: one square is centred on the grid's origin. */
export function squareLines(l: Layout, width: number, height: number): string {
  const side = across(l.size)
  const first = (origin: number) => (((origin - side / 2) % side) + side) % side
  const at = (v: number) => String(Number(v.toFixed(1)))
  const out: string[] = []
  for (let x = first(l.origin.x); x <= width; x += side) out.push(`M${at(x)} 0V${String(height)}`)
  for (let y = first(l.origin.y); y <= height; y += side) out.push(`M0 ${at(y)}H${String(width)}`)
  return out.join(' ')
}
