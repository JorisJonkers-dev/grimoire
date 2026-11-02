// Ids for the prep records the page list never opens directly. They sit in their own range, apart
// from the shared ids, so no other area of the seeded world can collide with them.

/** The seeded list's entry at an index the seed itself laid out, so it is always there. */
export const at = <T>(list: readonly T[], index: number): T => list[index] as T

export const pid = (tail: string) => `0190c7a8-0000-7000-8a00-${tail.padStart(12, '0')}`

export const prepIds = {
  npcs: { ivel: pid('101'), odo: pid('102') },
  locations: { bellhaven: pid('201'), ferry: pid('202'), chapel: pid('203') },
  settlements: { bellhaven: pid('211'), ferry: pid('212') },
  shops: { rope: pid('221'), remedies: pid('222') },
  pools: { shore: pid('301'), dead: pid('302') },
  tables: { road: pid('311') },
  checks: { first: pid('321') },
  loot: { purse: pid('331') },
  factions: { wardens: pid('401'), smugglers: pid('402'), watch: pid('403') },
  changes: [pid('411'), pid('412'), pid('413'), pid('414')] as const,
  quests: { bell: pid('501'), ferryman: pid('502'), secret: pid('503') },
  lore: { drowning: pid('511'), chart: pid('512'), letter: pid('513') },
  hooks: { deck: pid('601') },
  recipes: { potion: pid('701'), oil: pid('702') },
  downtime: [pid('711'), pid('712')] as const,
  vehicles: { gull: pid('801'), wagon: pid('802') },
  parts: [pid('811'), pid('812'), pid('813'), pid('814'), pid('815'), pid('816'), pid('817')] as const,
  worldMap: pid('901'),
  rolls: [pid('911'), pid('912')] as const,
  revisions: [pid('921'), pid('922'), pid('923'), pid('924')] as const,
  tokens: { goblin: pid('a01'), ghoul: pid('a02'), party: [pid('a11'), pid('a12'), pid('a13'), pid('a14')] as const },
} as const
