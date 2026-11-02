import type { CharacterDraft, CharacterRevisionLine, Companion, GameClock, PlaySession, Retrain, Track } from '@/infrastructure/api/types.gen'
import { ids } from '../ids'
import type { Core } from '../world'
import { cid } from './characters/ids'
import { builds, packs, snapshotOf, stash } from './characters/party'
import type { Build } from './characters/sheet'

/** A Character an Account owns, apart from the Campaign Characters made from it. */
export type Owned = { id: string; ownerId: string; name: string; species: string; class: string; background: string; backstory: string }

/** A Track with its scores: the party's under "party", each Character's under its id. */
export type KeptTrack = Omit<Track, 'standings'> & { values: Record<string, number> }

const at = (day: number, hour = 19) => `2026-10-${String(day).padStart(2, '0')}T${String(hour).padStart(2, '0')}:00:00Z`

/** The characters part of the seeded world: sheets, Inventories, spells, retrains, Tracks and the rest. */
export function seedCharacters(core: Core) {
  const p1 = core.accounts.player
  const [dm, ash] = core.members as [Core['members'][number], Core['members'][number]]
  const [tamsin, oswin, liri, brannoc] = ids.characters
  const tamsinBuild = builds[tamsin] as Build
  // Before her retrain Tamsin had Expertise in Acrobatics; the DM approved swapping it for Perception.
  const before = snapshotOf({ ...tamsinBuild, picks: tamsinBuild.picks.map((p) => (p.value === 'perception' ? { ...p, value: 'acrobatics' } : p)) })
  const retrain: Retrain = {
    id: cid('401'), characterId: tamsin, status: 'approved', reason: 'Tamsin trusts her eyes more than her balance after the flooded stair.',
    requestedBy: ash.displayName, decidedBy: dm.displayName, createdAt: at(8, 20), decidedAt: at(9, 10), proposed: snapshotOf(tamsinBuild),
  }
  const revisions: Record<string, CharacterRevisionLine[]> = {
    [tamsin]: [{ no: 1, author: dm.displayName, createdAt: at(9, 10), build: before, retrainId: retrain.id }],
  }
  const owned: Owned[] = core.characters.map((c) => {
    const b = builds[c.id] as Build
    return { id: c.id, ownerId: c.ownerId, name: c.name, species: c.species, class: c.class, background: b.background, backstory: b.backstory }
  })
  // The Character the first Player filed to join with: built, but not yet in the Campaign.
  owned.push({ id: ids.draft, ownerId: p1.id, name: 'Wren Ashdown', species: 'gnome', class: 'bard', background: 'sage', backstory: "Tamsin's cousin, who sings the tide-tables." })
  const drafts: Record<string, CharacterDraft> = {
    [p1.id]: {
      step: 3,
      build: {
        name: 'Wren Ashdown', species: 'gnome', class: 'bard', background: 'sage', method: 'standard-array',
        base: { strength: 8, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 15 },
        bonus: { intelligence: 2, constitution: 1 }, skills: [], shield: false, weapons: [],
      },
    },
  }
  const companions: Companion[] = [
    { id: cid('201'), name: 'Bramble', kind: 'companion', monsterSlug: 'mastiff', controllerId: ash.id, sharesXp: false, hp: 5, notes: 'Bramble once belonged to the drowned keeper; it growls at the bay.', updatedAt: at(9) },
    { id: cid('202'), name: 'Pell Marrow', kind: 'hireling', monsterSlug: 'commoner', sharesXp: true, notes: 'Pell is paid by the smugglers to report on the party.', updatedAt: at(10) },
  ]
  // The Sessions already played; the live one, Session 4, belongs with its Map and tokens in the prep seed.
  const sessions: PlaySession[] = [
    { id: cid('301'), number: 1, status: 'ended', seq: 212, gridRadius: 20, startedAt: at(3), endedAt: at(3, 23) },
    { id: cid('302'), number: 2, status: 'ended', seq: 340, gridRadius: 20, startedAt: at(10), endedAt: at(10, 23) },
    { id: cid('303'), number: 3, status: 'ended', seq: 41, gridRadius: 20, startedAt: at(17), endedAt: at(17, 23) },
  ]
  const tracks: KeptTrack[] = [
    {
      id: cid('101'), name: 'Renown with the Bellwardens', scope: 'party', min: 0, max: 10, start: 0,
      thresholds: [
        { at: 3, rising: true, label: 'The Bellwardens open their archive to the party' },
        { at: 6, rising: true, label: 'A Bellwarden ship answers the party\'s call' },
      ],
      values: { party: 2 },
    },
    {
      id: cid('102'), name: 'Dread', scope: 'character', min: 0, max: 6, start: 0,
      thresholds: [
        { at: 3, rising: true, label: 'Dreams of the drowned bell', effect: 'frightened' },
        { at: 6, rising: true, label: 'Flees the shore at the next toll' },
      ],
      values: { [tamsin]: 1, [oswin]: 3, [liri]: 2, [brannoc]: 0 },
    },
  ]
  const clock: GameClock = { day: 14, minute: 19 * 60 }
  return {
    builds: structuredClone(builds),
    packs: structuredClone(packs),
    stash: structuredClone(stash),
    retrains: [retrain],
    revisions,
    owned,
    drafts,
    companions,
    sessions,
    tracks,
    clock,
    /** The link token of each open Campaign Invite: the API keeps only a hash, the demo the token. */
    inviteTokens: { [ids.invite]: ids.inviteToken } as Record<string, string>,
  }
}
