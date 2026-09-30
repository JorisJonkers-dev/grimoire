import { describe, expect, it } from 'vitest'
import golden from '../../../fixtures/hex-geometry.json'
import { corners, distance, fromPixel, line, toPixel } from './hex'

describe('hex geometry agrees with the server', () => {
  for (const l of golden.layouts) {
    const layout = { size: l.size, origin: { x: l.originX, y: l.originY } }
    it(`places centres and picks hexes for size ${String(l.size)}`, () => {
      for (const c of l.centres) {
        const p = toPixel(layout, { q: c.q, r: c.r })
        expect(p.x).toBeCloseTo(c.x, 3)
        expect(p.y).toBeCloseTo(c.y, 3)
      }
      for (const p of l.picks) {
        expect(fromPixel(layout, { x: p.x, y: p.y })).toEqual({ q: p.q, r: p.r })
      }
    })
  }

  it('draws lines and measures distance', () => {
    for (const g of golden.lines) {
      const a = { q: g.from[0] ?? 0, r: g.from[1] ?? 0 }
      const b = { q: g.to[0] ?? 0, r: g.to[1] ?? 0 }
      expect(distance(a, b)).toBe(g.distance)
      expect(line(a, b).map((c) => [c.q, c.r])).toEqual(g.hexes)
    }
  })

  it('puts six corners one size from the centre', () => {
    const layout = { size: 10, origin: { x: 0, y: 0 } }
    const pts = corners(layout, { q: 1, r: 0 })
    const centre = toPixel(layout, { q: 1, r: 0 })
    expect(pts).toHaveLength(6)
    for (const p of pts) expect(Math.hypot(p.x - centre.x, p.y - centre.y)).toBeCloseTo(10, 9)
  })
})
