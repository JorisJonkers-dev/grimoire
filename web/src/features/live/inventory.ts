import type { LiveContainer } from '@/infrastructure/api/types.gen'

// A Character's Inventory, and every bag in it, names its owner; nothing else is anyone's.
function shared(c: LiveContainer, me: string): boolean {
  return c.ownerId ? c.ownerId === me : c.kind !== 'character'
}

/** Whether a member may take from a Container: the DM always, anyone from the stash or a drop, a Player from their own Characters. */
export function canTake(c: LiveContainer, dm: boolean, me: string): boolean {
  return dm || shared(c, me)
}

/** Whether a member may put into a Container: never a drop; the DM anywhere else, anyone the stash, a Player their own Characters. */
export function canPut(c: LiveContainer, dm: boolean, me: string): boolean {
  return c.kind !== 'loot_drop' && (dm || shared(c, me))
}

/** "Climbing Line ×1 · 3 charges · neck · attuned", or "Anvil · unidentified" for what the party cannot read. */
export function instanceLabel(i: LiveContainer['instances'][number]): string {
  const parts = [i.count > 1 ? `${i.name} ×${String(i.count)}` : i.name]
  if (i.charges !== undefined) parts.push(`${String(i.charges)} ${i.charges === 1 ? 'charge' : 'charges'}`)
  if (i.slot) parts.push(i.slot.replace('_', ' '))
  if (i.attuned) parts.push('attuned')
  if (!i.identified) parts.push('unidentified')
  return parts.join(' · ')
}

/** "301.2 / 120 lb" for a Character, "12 lb" for anything else. */
export function load(c: LiveContainer): string {
  const w = Math.round(c.weightLb * 10) / 10
  return c.capacityLb ? `${String(w)} / ${String(c.capacityLb)} lb` : `${String(w)} lb`
}

/** What is being dragged, as the drop target reads it back. */
export type Dragged = { from: string; itemSlug?: string; coin?: LiveContainer['coins'][number]['coin']; count: number }
