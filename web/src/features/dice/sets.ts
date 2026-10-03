import type { DiceSet, DieLook, DieLooks, DiePlacement, LiveDiceLook } from '@/infrastructure/api/types.gen'

export const DIE_TYPES = ['d4', 'd6', 'd8', 'd10', 'd12', 'd20', 'd100'] as const
export type DieType = (typeof DIE_TYPES)[number]
export const PATTERNS = ['plain', 'marble', 'speckled', 'stripes'] as const

/** How many faces each die unwraps to; a d100 is rolled on a ten-sided die. */
export const SHEET_FACES: Record<DieType, number> = { d4: 4, d6: 6, d8: 8, d10: 10, d12: 12, d20: 20, d100: 10 }
/** The dice as they come: what a die type wears until a Dice Set dresses it. */
export const PLAIN: DieLook = { pattern: 'plain', body: '#7a1f1a', numbers: '#f3d27a' }
/** Where a picture starts when it is first put on a die: centred, as wide as the sheet. */
export const CENTRED: DiePlacement = { x: 0.5, y: 0.5, scale: 1, rotation: 0 }

/** The unwrapped faces of a die sit in a square grid on its sheet, in the order the die counts them. */
export const sheetCols = (faces: number) => Math.ceil(Math.sqrt(faces))
export const cellOf = (i: number, cols: number) => ({ col: i % cols, row: Math.floor(i / cols) })

/** The look a Dice Set gives a die of so many faces, if it dresses that die at all. */
export function lookFor(look: LiveDiceLook | undefined, faces: number): DieLook | undefined {
  const type = DIE_TYPES.find((t) => t === `d${String(faces)}`)
  return type ? look?.dice[type] : undefined
}

/** A Dice Set as a roll wears it. */
export function lookOf(set: DiceSet): LiveDiceLook {
  return set.imageUrl ? { dice: set.design.dice, imageUrl: set.imageUrl } : { dice: set.design.dice }
}

/** Every die type dressed: a set's own looks, and the plain look where it has none. */
export function everyDie(dice: DieLooks): Record<DieType, DieLook> {
  return Object.fromEntries(DIE_TYPES.map((t) => [t, { ...(dice[t] ?? PLAIN) }])) as Record<DieType, DieLook>
}

/** Where a placed picture sits on the sheet, as the preview and the 3D dice both draw it. */
export function pictureStyle(p: DiePlacement): Record<string, string> {
  const pct = (n: number) => `${String(Math.round(n * 1000) / 10)}%`
  return { left: pct(p.x), top: pct(p.y), width: pct(p.scale), transform: `translate(-50%, -50%) rotate(${String(p.rotation)}deg)` }
}

/** What the owner is told about a set shared with everyone. */
export function reviewNote(set: DiceSet): string {
  if (set.copy || set.sharing !== 'everyone') return ''
  return { none: '', pending: 'Waiting for an Admin to check its picture; until then only your Friends see it.', approved: 'An Admin approved its picture.', rejected: 'An Admin turned its picture down; only your Friends see it.' }[set.review]
}
