import type { LiveCombatant, LiveToken, Tactics } from '@/infrastructure/api/types.gen'

export const TACTICS: { value: Tactics; label: string }[] = [
  { value: 'auto', label: 'From Intelligence' },
  { value: 'simple', label: 'Simple' },
  { value: 'cunning', label: 'Cunning' },
  { value: 'off', label: 'Off' },
]

/** The creatures a DM runs: those with stats that no Player controls, by name so each keeps its place. */
export const runByDM = (tokens: LiveToken[]) =>
  tokens.filter((t) => t.kind !== 'object' && t.controllerId === undefined && t.attacks !== undefined).sort((a, b) => a.label.localeCompare(b.label) || a.id.localeCompare(b.id))

/** A creature's Suggested Action: a few words for under its token, the same for a screen reader, and as a sentence with its reason. */
export function suggested(c: LiveCombatant, token: LiveToken, target: string): { text: string; note: string; sentence: string } | undefined {
  const s = c.suggestion
  if (!s) return undefined
  const attack = s.attackNo === undefined ? undefined : token.attacks?.[s.attackNo]?.name
  const [text, what] = attack ? [`${attack} → ${target}`, `${attack} against ${target}`] : [`Close in → ${target}`, `Close in on ${target}`]
  return { text, note: `suggested: ${what}`, sentence: `${what}. ${s.reason}` }
}

/**
 * Which creatures the DM has in hand after tapping one. One at a time it switches; with several at once
 * a tap adds a creature or lets it go, and the last one always stays.
 */
export function taken(inHand: string[], id: string, several: boolean): string[] {
  if (!several) return [id]
  if (!inHand.includes(id)) return [...inHand, id]
  return inHand.length > 1 ? inHand.filter((x) => x !== id) : inHand
}
