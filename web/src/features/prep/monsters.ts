import type { EncounterMonster } from '@/infrastructure/api/types.gen'

/** Reads "goblin x3, ogre" as three goblins and one ogre; null when a part is not a slug and count. */
export function parseMonsters(text: string): EncounterMonster[] | null {
  const out: EncounterMonster[] = []
  for (const part of text.split(',').map((p) => p.trim()).filter(Boolean)) {
    const m = /^([a-z0-9]+(?:-[a-z0-9]+)*)(?:\s*[x×]\s*(\d+))?$/i.exec(part)
    if (!m?.[1]) return null
    out.push({ monsterSlug: m[1].toLowerCase(), count: Number(m[2] ?? 1) })
  }
  return out
}

/** Writes monsters back as the DM types them. */
export function formatMonsters(monsters: EncounterMonster[] = []): string {
  return monsters.map((m) => (m.count === 1 ? m.monsterSlug : `${m.monsterSlug} x${String(m.count)}`)).join(', ')
}
