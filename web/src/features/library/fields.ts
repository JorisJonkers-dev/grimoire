import type { LibraryField } from '@/infrastructure/api/types.gen'

/** The rows worth saving: named, trimmed, the last value kept when a name repeats. */
export function cleanFields(rows: LibraryField[]): LibraryField[] {
  const out = new Map<string, string>()
  for (const r of rows) {
    const name = r.name.trim()
    if (name) out.set(name, r.value)
  }
  return [...out].map(([name, value]) => ({ name, value }))
}

/** Editable copies of fields, so editing never touches the cached response. */
export const copyFields = (fields: LibraryField[]): LibraryField[] => fields.map((f) => ({ ...f }))

export const kindNames: Record<string, string> = {
  creature: 'Creature', npc: 'NPC', location: 'Location', shop: 'Shop', item: 'Item', spell: 'Spell', table: 'Table', subclass: 'Subclass', class: 'Class',
}

export const statusNames: Record<string, string> = {
  pending: 'Waiting for the DM', changes_requested: 'Changes asked for', approved: 'Approved', declined: 'Declined',
}

export const stepNames: Record<string, string> = {
  submitted: 'Sent', resubmitted: 'Sent again', changes_requested: 'Changes asked for', approved: 'Approved', declined: 'Declined',
}

/** Each field on either side of a review: its value now, its proposed value, and whether they differ. */
export function compare(now: LibraryField[], proposed: LibraryField[]): { name: string; now?: string; proposed?: string; changed: boolean }[] {
  const names = [...new Set([...now.map((f) => f.name), ...proposed.map((f) => f.name)])].sort()
  return names.map((name) => {
    const a = now.find((f) => f.name === name)?.value
    const b = proposed.find((f) => f.name === name)?.value
    return { name, now: a, proposed: b, changed: a !== b }
  })
}
