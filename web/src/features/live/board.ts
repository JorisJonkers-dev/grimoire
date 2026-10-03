import type { LiveSurface, LiveToken } from '@/infrastructure/api/types.gen'
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

/** A token as the screen reader and the token list name it, with what its audience knows of its health. */
export function describe(t: LiveToken): string {
  const temp = t.tempHp ? ` +${String(t.tempHp)} temp` : ''
  const health = t.hp !== undefined && t.hpMax !== undefined ? ` (${String(t.hp)}/${String(t.hpMax)} HP${temp})` : t.health ? ` (${t.health})` : ''
  const effects = t.effects?.length ? ` · ${t.effects.map((e) => e.name).join(', ')}` : ''
  const form = t.form ? ` as ${t.form}` : ''
  return `${t.label}${form}${t.hidden ? ' (hidden)' : ''}${health}${effects}`
}

/** The hexes every emanation on the board covers, around whoever carries it. */
export function emanations(tokens: LiveToken[]): Coord[] {
  return tokens.flatMap((t) => (t.effects ?? []).flatMap((e) => e.hexes ?? []))
}

export function initials(label: string): string {
  return label
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p.charAt(0).toUpperCase())
    .join('')
}

export type Ground = { surfaces?: LiveSurface[]; area?: Coord[]; zone?: Coord[]; reach?: Coord[]; danger?: Coord[] }

/** What a planned walk draws by leaving a hex. */
export const DANGER_NOTE = 'leaving here draws an opportunity attack'

/** What lies on a hex besides a token: a Surface, whether an area spell covers it, and whether an Encounter Zone does. */
export function groundNotes(k: string, surfaces: Map<string, LiveSurface>, area: Set<string>, zone: Set<string> = new Set(), reach: Set<string> = new Set()): string[] {
  const s = surfaces.get(k)
  return [s ? s.kind : '', area.has(k) ? 'in the area' : '', zone.has(k) ? 'in an encounter zone' : '', reach.has(k) ? 'watched' : ''].filter(Boolean)
}

/** Every hex of the Encounter Zones around their centres. */
export function zoneHexes(zones: { q: number; r: number; radiusHexes: number }[], cells: Coord[]): Coord[] {
  return cells.filter((c) => zones.some((z) => distance(z, c) <= z.radiusHexes))
}

/** Paints tokens, a walk preview, Surfaces and an area onto the grid; hidden tokens only ever arrive for the DM. */
export function board(radius: number, tokens: LiveToken[], selected: string | null, path: Coord[] = [], ground: Ground = {}): GridCell[] {
  const key = (c: Coord) => `${String(c.q)},${String(c.r)}`
  const at = new Map(tokens.map((t) => [key(t), t]))
  const route = new Set(path.map(key))
  const surfaces = new Map((ground.surfaces ?? []).map((s) => [key(s), s]))
  const area = new Set((ground.area ?? []).map(key))
  const zone = new Set((ground.zone ?? []).map(key))
  const reach = new Set((ground.reach ?? []).map(key))
  const danger = new Set((ground.danger ?? []).map(key))
  return hexes(radius).map((c) => {
    const k = key(c)
    const t = at.get(k)
    const notes = [...groundNotes(k, surfaces, area, zone, reach), danger.has(k) ? DANGER_NOTE : ''].filter(Boolean)
    if (!t) {
      const tone = danger.has(k) ? 'danger' : route.has(k) ? 'path' : area.has(k) ? 'area' : surfaces.has(k) ? `surface-${surfaces.get(k)?.kind ?? ''}` : zone.has(k) ? 'zone' : reach.has(k) ? 'watched' : undefined
      const label = [route.has(k) ? 'on the path' : '', ...notes].filter(Boolean).join(', ')
      return tone ? { ...c, tone, label } : c
    }
    const tone = t.hidden ? 'hidden' : t.kind === 'party' ? 'ally' : t.kind
    return { ...c, tone: t.id === selected ? 'selected' : tone, label: [describe(t), ...notes].join(', '), mark: initials(t.label) }
  })
}
