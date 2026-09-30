/** Hex geometry only; every rule lives on the server (ADR-0004). Pinned to the Go layout by fixtures/hex-geometry.json. */
export type Coord = { q: number; r: number }
export type Point = { x: number; y: number }
export type Layout = { size: number; origin: Point }

const SQRT3 = Math.sqrt(3)

export function toPixel(l: Layout, c: Coord): Point {
  return { x: l.size * (SQRT3 * c.q + (SQRT3 / 2) * c.r) + l.origin.x, y: l.size * 1.5 * c.r + l.origin.y }
}

function round(fq: number, fr: number): Coord {
  const fs = -fq - fr
  let q = Math.round(fq)
  let r = Math.round(fr)
  const s = Math.round(fs)
  const dq = Math.abs(q - fq)
  const dr = Math.abs(r - fr)
  const ds = Math.abs(s - fs)
  if (dq > dr && dq > ds) q = -r - s
  else if (dr > ds) r = -q - s
  return { q: q + 0, r: r + 0 }
}

export function fromPixel(l: Layout, p: Point): Coord {
  const px = (p.x - l.origin.x) / l.size
  const py = (p.y - l.origin.y) / l.size
  return round((SQRT3 / 3) * px - py / 3, (2 / 3) * py)
}

export function distance(a: Coord, b: Coord): number {
  const dq = a.q - b.q
  const dr = a.r - b.r
  return (Math.abs(dq) + Math.abs(dr) + Math.abs(dq + dr)) / 2
}

/** The six corners of a hex, for drawing. */
export function corners(l: Layout, c: Coord): Point[] {
  const centre = toPixel(l, c)
  return [0, 1, 2, 3, 4, 5].map((i) => {
    const angle = (Math.PI / 180) * (60 * i - 30)
    return { x: centre.x + l.size * Math.cos(angle), y: centre.y + l.size * Math.sin(angle) }
  })
}

export function line(a: Coord, b: Coord): Coord[] {
  const n = distance(a, b)
  const eps = 1e-6
  const out: Coord[] = []
  for (let i = 0; i <= n; i++) {
    const t = n === 0 ? 0 : i / n
    out.push(round(a.q + eps + (b.q - a.q) * t, a.r + eps + (b.r - a.r) * t))
  }
  return out
}
