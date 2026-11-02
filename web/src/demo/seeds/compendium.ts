import type { Core } from '../world'
import { seedBuilds } from './compendium/builds'
import { seedLibrary } from './compendium/library'
import { automation, entries, spells, srd52 } from './compendium/srd'

/** The compendium part of the seeded world: SRD 5.2 rules, and the Library built on top of them. */
export function seedCompendium(core: Core) {
  return {
    sources: [srd52],
    spells: structuredClone(spells),
    entries: structuredClone(entries),
    automation,
    builds: seedBuilds(),
    ...seedLibrary(core),
  }
}
