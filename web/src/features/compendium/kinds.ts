import type { EntryKind } from '@/infrastructure/api/types.gen'

export const entryKinds: readonly { kind: EntryKind; label: string }[] = [
  { kind: 'class', label: 'Classes' },
  { kind: 'species', label: 'Species' },
  { kind: 'background', label: 'Backgrounds' },
  { kind: 'feat', label: 'Feats' },
  { kind: 'weapon', label: 'Weapons' },
  { kind: 'armor', label: 'Armor' },
  { kind: 'item', label: 'Equipment' },
  { kind: 'magic-item', label: 'Magic items' },
  { kind: 'monster', label: 'Monsters' },
  { kind: 'condition', label: 'Conditions' },
]

export function kindLabel(kind: string): string | undefined {
  return entryKinds.find((k) => k.kind === kind)?.label
}

export function isEntryKind(kind: string): kind is EntryKind {
  return kindLabel(kind) !== undefined
}
