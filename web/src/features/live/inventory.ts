import type { LiveContainer } from '@/infrastructure/api/types.gen'

/** Whether a member may take from a Container: the DM always, anyone from the stash or a drop, a Player from their own Characters. */
export function canTake(c: LiveContainer, dm: boolean, me: string): boolean {
  return dm || c.kind !== 'character' || c.ownerId === me
}

/** Whether a member may put into a Container: never a drop; the DM anywhere else, anyone the stash, a Player their own Characters. */
export function canPut(c: LiveContainer, dm: boolean, me: string): boolean {
  return c.kind !== 'loot_drop' && (dm || c.kind === 'party_stash' || c.ownerId === me)
}

/** "301.2 / 120 lb" for a Character, "12 lb" for anything else. */
export function load(c: LiveContainer): string {
  const w = Math.round(c.weightLb * 10) / 10
  return c.capacityLb ? `${String(w)} / ${String(c.capacityLb)} lb` : `${String(w)} lb`
}

/** What is being dragged, as the drop target reads it back. */
export type Dragged = { from: string; itemSlug?: string; coin?: LiveContainer['coins'][number]['coin']; count: number }
