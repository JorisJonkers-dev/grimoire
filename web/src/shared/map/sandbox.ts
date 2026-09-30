import type { HexCell, HexOccupant } from '@/infrastructure/api/types.gen'
import { type Coord, distance } from '@/shared/hex'

/** A small fixed battlefield for trying movement and sight previews. */
export function sandbox(): { cells: HexCell[]; occupants: HexOccupant[] } {
  const cells: HexCell[] = []
  for (let q = -4; q <= 4; q++) {
    for (let r = -4; r <= 4; r++) {
      if (distance({ q: 0, r: 0 }, { q, r }) > 4) continue
      const wall = r === -1 && q >= -1 && q <= 1
      const mud = q === 2 && r >= -1
      cells.push({ q, r, ...(wall ? { blocked: true, blocksSight: true } : {}), ...(mud ? { difficult: true } : {}) })
    }
  }
  const occupants: HexOccupant[] = [
    { q: -2, r: 2, side: 'ally' },
    { q: 1, r: -3, side: 'enemy' },
  ]
  return { cells, occupants }
}

export const key = (c: Coord) => `${String(c.q)},${String(c.r)}`
