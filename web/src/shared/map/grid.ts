import type { Coord } from '@/shared/hex'

export type GridCell = Coord & { tone?: string; label?: string; mark?: string }
