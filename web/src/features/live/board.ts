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

const keyOf = (c: Coord) => `${String(c.q)},${String(c.r)}`

/** The tokens on each hex, the one it shows first: a rider sits on top of its mount. */
export function stacks(tokens: LiveToken[]): Map<string, LiveToken[]> {
  const at = new Map<string, LiveToken[]>()
  for (const t of tokens) {
    const there = at.get(keyOf(t)) ?? []
    at.set(keyOf(t), t.mountId ? [t, ...there] : [...there, t])
  }
  return at
}

/** The token of a hex that comes after one of them: the first when none is named, none after the last. */
export function after(at: Map<string, LiveToken[]>, c: Coord, id: string | null | undefined): LiveToken | undefined {
  const there = at.get(keyOf(c)) ?? []
  return there[there.findIndex((t) => t.id === id) + 1]
}

/** The tokens of one hex as the screen reader names them: a rider, then what it rides. */
export const describeStack = (there: LiveToken[]) => there.map(describe).join(there[0]?.mountId ? ', riding ' : ', with ')

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

/** What the DM sees written under a token: a few words, and the same in full for a screen reader. */
export type Captions = Record<string, { text: string; note: string }>
export type Ground = { surfaces?: LiveSurface[]; area?: Coord[]; zone?: Coord[]; reach?: Coord[]; danger?: Coord[]; captions?: Captions }

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
  const key = keyOf
  const at = stacks(tokens)
  const route = new Set(path.map(key))
  const surfaces = new Map((ground.surfaces ?? []).map((s) => [key(s), s]))
  const area = new Set((ground.area ?? []).map(key))
  const zone = new Set((ground.zone ?? []).map(key))
  const reach = new Set((ground.reach ?? []).map(key))
  const danger = new Set((ground.danger ?? []).map(key))
  return hexes(radius).map((c) => {
    const k = key(c)
    const there = at.get(k) ?? []
    const t = there[0]
    const notes = [...groundNotes(k, surfaces, area, zone, reach), danger.has(k) ? DANGER_NOTE : ''].filter(Boolean)
    if (!t) {
      const tone = danger.has(k) ? 'danger' : route.has(k) ? 'path' : area.has(k) ? 'area' : surfaces.has(k) ? `surface-${surfaces.get(k)?.kind ?? ''}` : zone.has(k) ? 'zone' : reach.has(k) ? 'watched' : undefined
      const label = [route.has(k) ? 'on the path' : '', ...notes].filter(Boolean).join(', ')
      return tone ? { ...c, tone, label } : c
    }
    const tone = t.hidden ? 'hidden' : t.kind === 'party' ? 'ally' : t.kind
    const caption = ground.captions?.[t.id]
    const picked = there.some((o) => o.id === selected)
    return { ...c, tone: picked ? 'selected' : tone, label: [describeStack(there), ...notes, caption?.note ?? ''].filter(Boolean).join(', '), mark: initials(t.label), caption: caption?.text, captionKey: t.id }
  })
}
