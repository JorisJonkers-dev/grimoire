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
  creature: 'Creature', npc: 'NPC', location: 'Location', shop: 'Shop', item: 'Item', spell: 'Spell', table: 'Table',
}
