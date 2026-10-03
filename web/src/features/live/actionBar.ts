import type { LiveToken } from '@/infrastructure/api/types.gen'

/** One thing a creature can do in play, as a tile: named kind:name so a layout can keep its place. */
export type Tile = { key: string; name: string; tip: string; testid: string; detail?: string; mastery?: string }
/** A place on a bar: the tile there, or only its name when the Character lacks it now. */
export type Slot = { key: string; name: string; tile?: Tile }
export type Layout = { bars: string[][]; quick: string[]; stowed: string[] }

/** A bar holds a tile for each of the keys 1 to 0; there are two bars and a quick bar of four. */
export const BAR_TILES = 10
export const BARS = 2
export const QUICK_TILES = 4

// What each mastery does, for the tooltips.
export const masteries: Record<string, string> = {
  cleave: 'Cleave: on a hit, attack a second creature next to the first, once a turn.',
  graze: 'Graze: on a miss, deal your ability modifier in damage.',
  nick: 'Nick: the off-hand attack is part of the Attack action.',
  push: 'Push: on a hit, push the target 10 ft away.',
  sap: 'Sap: on a hit, the target has Disadvantage on its next attack.',
  slow: 'Slow: on a damaging hit, the target loses 10 ft of Speed.',
  topple: 'Topple: on a hit, the target saves (Constitution) or falls Prone.',
  vex: 'Vex: on a damaging hit, your next attack against it has Advantage.',
}
// The 2024 actions besides Attack and Ready, in the order the rules list them.
export const actions = [
  { key: 'dash', name: 'Dash', tip: 'Gain extra movement equal to your Speed this turn.' },
  { key: 'disengage', name: 'Disengage', tip: 'Your movement provokes no Opportunity Attacks this turn.' },
  { key: 'dodge', name: 'Dodge', tip: 'Attacks against you have Disadvantage until your next turn.' },
  { key: 'help', name: 'Help', tip: 'Give an ally Advantage on their next check or attack.' },
  { key: 'hide', name: 'Hide', tip: 'DC 15 Dexterity (Stealth); on a success you are Invisible.' },
  { key: 'influence', name: 'Influence', tip: 'Sway a creature with a Charisma or Wisdom check.' },
  { key: 'magic', name: 'Magic', tip: 'Cast a spell or use a magic item.' },
  { key: 'search', name: 'Search', tip: 'Wisdom (Perception) to find what is hidden.' },
  { key: 'study', name: 'Study', tip: 'Intelligence to recall or work something out.' },
  { key: 'utilize', name: 'Utilize', tip: 'Use an object: open a door, pull a lever.' },
]
export const unarmed = [
  { key: 'grapple', name: 'Grapple' },
  { key: 'shove_push', name: 'Shove away' },
  { key: 'shove_prone', name: 'Shove down' },
]
export const areaSpells = [
  { slug: 'burning-hands', name: 'Burning Hands' },
  { slug: 'thunderwave', name: 'Thunderwave' },
  { slug: 'shatter', name: 'Shatter' },
  { slug: 'grease', name: 'Grease' },
  { slug: 'fireball', name: 'Fireball' },
  { slug: 'lightning-bolt', name: 'Lightning Bolt' },
  { slug: 'cone-of-cold', name: 'Cone of Cold' },
  { slug: 'spirit-guardians', name: 'Spirit Guardians' },
  { slug: 'wall-of-fire', name: 'Wall of Fire' },
]
export const summonings = [
  { slug: 'find-familiar', name: 'Find Familiar' },
  { slug: 'animate-dead', name: 'Animate Dead' },
]

type Attack = NonNullable<LiveToken['attacks']>[number]
export const signed = (n: number) => (n >= 0 ? `+${String(n)}` : String(n))
export const damageOf = (a: Attack) => (a.damage ? `${a.damage}${a.damageBonus ? signed(a.damageBonus) : ''}` : String(a.damageBonus))
export const reachOf = (a: Attack) =>
  [a.reachFt ? `reach ${String(a.reachFt)} ft` : '', a.rangeFt ? `range ${String(a.rangeFt)}/${String(a.longRangeFt)} ft` : ''].filter(Boolean).join(', ')

/** Every tile a token has, in the usual order: its attacks, the actions, unarmed strikes, moves, spells and summons. */
export function tilesOf(token: LiveToken): Tile[] {
  const attacks = (token.attacks ?? []).map((a, i) => ({
    key: `attack:${a.name}`, name: a.name, testid: `attack-${String(i)}`, mastery: a.mastery,
    tip: `${a.name}: ${signed(a.toHit)} to hit, ${damageOf(a)} ${a.damageType ?? ''}`,
    detail: `${signed(a.toHit)} · ${damageOf(a)} · ${reachOf(a)}`,
  }))
  const moves = [
    ...(token.kind === 'party'
      ? [{ key: 'move:swap', name: 'Swap weapons', testid: 'swap-weapons', tip: 'Put your weapons away and draw your other set: each costs the equip of an attack or your free interaction; a shield takes your action.' }]
      : []),
    { key: 'move:jump', name: 'Jump', testid: 'jump', tip: 'Leap as far as your Strength score in feet with a run-up.' },
    { key: 'move:mount', name: token.mountId ? 'Dismount' : 'Mount', testid: 'ride', tip: 'Get onto a willing creature next to you, or off the one you ride: half your Speed.' },
    { key: 'move:throw', name: 'Throw', testid: 'throw', tip: 'Throw the creature you grapple, or a barrel or chest next to you.' },
    { key: 'move:misty-step', name: 'Misty Step', testid: 'misty-step', tip: 'Bonus Action: teleport up to 30 feet to a free hex.' },
  ]
  const all = [
    ...attacks,
    ...actions.map((a) => ({ key: `action:${a.key}`, name: a.name, testid: `action-${a.key}`, tip: a.tip })),
    ...unarmed.map((u) => ({ key: `unarmed:${u.key}`, name: u.name, testid: `unarmed-${u.key}`, tip: `${u.name}: an Unarmed Strike.` })),
    ...moves,
    ...areaSpells.map((s) => ({ key: `spell:${s.slug}`, name: s.name, testid: `spell-${s.slug}`, tip: `Aim ${s.name}.` })),
    ...summonings.map((s) => ({ key: `summon:${s.slug}`, name: s.name, testid: `summon-${s.slug}`, tip: `Place what ${s.name} calls.` })),
  ]
  // Two attacks of one name share a tile: a layout keeps one place for it.
  return all.filter((t, i) => all.findIndex((o) => o.key === t.key) === i)
}

/** The layout a Character starts from: the usual order across both bars, the first four on the quick bar. */
export function defaultLayout(tiles: Tile[]): Layout {
  const keys = tiles.map((t) => t.key)
  return { bars: [keys.slice(0, BAR_TILES), keys.slice(BAR_TILES, BARS * BAR_TILES)], quick: keys.slice(0, QUICK_TILES), stowed: keys.slice(BARS * BAR_TILES) }
}

const nameOf = (key: string) => key.slice(key.indexOf(':') + 1)

/**
 * Lays a token's tiles out as its player arranged them. A tile the token lacks now keeps its place,
 * greyed; a tile the layout has never seen joins the end of the bars, or waits in the drawer when
 * both are full.
 */
export function arrange(layout: Layout, tiles: Tile[]): { bars: Slot[][]; quick: Slot[]; stowed: Tile[] } {
  const byKey = new Map(tiles.map((t) => [t.key, t]))
  const slot = (key: string): Slot => ({ key, name: byKey.get(key)?.name ?? nameOf(key), tile: byKey.get(key) })
  const bars = Array.from({ length: BARS }, (_, i) => (layout.bars[i] ?? []).map(slot))
  const placed = new Set([...layout.bars.flat(), ...layout.stowed])
  const stowed = tiles.filter((t) => layout.stowed.includes(t.key))
  for (const t of tiles.filter((x) => !placed.has(x.key))) {
    const bar = bars.find((b) => b.length < BAR_TILES)
    if (bar) bar.push(slot(t.key))
    else stowed.push(t)
  }
  return { bars, quick: layout.quick.map(slot), stowed }
}

const without = (layout: Layout, key: string): Layout => ({ bars: layout.bars.map((b) => b.filter((k) => k !== key)), quick: layout.quick, stowed: layout.stowed.filter((k) => k !== key) })

/**
 * Puts a tile at a place on a bar, from wherever it was. On a full bar a tile from the other bar trades
 * places with the one there; a tile from the drawer has to wait for room.
 */
export function move(layout: Layout, key: string, bar: number, at: number): Layout {
  const next = without(layout, key)
  const bars = Array.from({ length: BARS }, (_, i) => [...(next.bars[i] ?? [])])
  const target = bars[bar]
  if (!target) return layout
  if (target.length < BAR_TILES) {
    target.splice(Math.min(at, target.length), 0, key)
    return { ...next, bars }
  }
  const from = layout.bars.findIndex((b) => b.includes(key))
  const there = Math.min(at, BAR_TILES - 1)
  const displaced = target[there]
  if (from < 0 || displaced === undefined) return layout
  bars[from]?.splice(layout.bars[from]?.indexOf(key) ?? 0, 0, displaced)
  target[there] = key
  return { ...next, bars }
}

/** Takes a tile off the bars and the quick bar, into the drawer. */
export function stow(layout: Layout, key: string): Layout {
  const next = without(layout, key)
  return { ...next, quick: next.quick.filter((k) => k !== key), stowed: [...next.stowed, key] }
}

/** Brings a tile back from the drawer to the end of the first bar with room. */
export function unstow(layout: Layout, key: string): Layout {
  const bar = Array.from({ length: BARS }, (_, i) => layout.bars[i] ?? []).findIndex((b) => b.length < BAR_TILES)
  return bar < 0 ? layout : move(layout, key, bar, BAR_TILES)
}

/** Puts a tile on the quick bar, or takes it off; the quick bar holds four. */
export function pin(layout: Layout, key: string): Layout {
  if (layout.quick.includes(key)) return { ...layout, quick: layout.quick.filter((k) => k !== key) }
  return layout.quick.length >= QUICK_TILES ? layout : { ...layout, quick: [...layout.quick, key] }
}
