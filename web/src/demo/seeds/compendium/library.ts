import type { LibraryField, LibraryKind, LibraryRevision, Proposal, ProposalStep, SharedSubmission } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import type { Core } from '../../world'

// The Library side of the world: the DM's homebrew for The Sunken Bell, the Collections and links that
// bring it into the Campaign, the Shared Library, and the Proposals Players sent the DM.

/** The owner of the Shared Library's read-only copies; nobody signs in as it. */
export const sharedOwner = 'shared'

/** A Library Entry with its owner and every Revision; its revision number is its newest one's. */
export type DemoLibraryEntry = {
  id: string
  ownerId: string
  kind: LibraryKind
  name: string
  fields: LibraryField[]
  createdAt: string
  updatedAt: string
  revisions: LibraryRevision[]
}

/** A Collection; switchedOn says whether it brings its entries into the seeded Campaign. */
export type DemoCollection = { id: string; ownerId: string; name: string; description: string; entryIds: string[]; switchedOn: boolean }

/** An entry the DM linked into the seeded Campaign themselves, with its pin and Campaign Override. */
export type DemoLink = { entryId: string; pinnedRevision?: number; override: LibraryField[] }

export type DemoProposal = Proposal & { authorId: string; steps: ProposalStep[] }
export type DemoSubmission = SharedSubmission & { submitterId: string }

const own = (tail: string) => `0190c7a8-0000-7000-8000-${`5b${tail}`.padStart(12, '0')}`

/** Ids of the records this slice adds beyond those the page list opens. */
export const libraryIds = {
  npc: own('001'),
  shop: own('002'),
  /** The DM's own spell an Admin approved into the Shared Library. */
  lantern: own('003'),
  sharedSpell: own('011'),
  sharedTable: own('012'),
  homebrew: own('021'),
  bestiary: own('022'),
  otherProposal: own('031'),
  submission: own('041'),
  otherSubmission: own('042'),
}

const fields = (pairs: Record<string, string>): LibraryField[] =>
  Object.keys(pairs).sort().map((name) => ({ name, value: pairs[name] as string }))

function entry(id: string, ownerId: string, kind: LibraryKind, name: string, pairs: Record<string, string>, at: string, more: LibraryRevision[] = []): DemoLibraryEntry {
  const first: LibraryRevision = { no: 1, name, fields: fields(pairs), createdAt: at, origin: 'ui' }
  const revisions = [first, ...more]
  const last = revisions.at(-1) as LibraryRevision
  return { id, ownerId, kind, name: last.name, fields: last.fields, createdAt: at, updatedAt: last.createdAt, revisions }
}

const at = '2026-09-12T19:30:00Z'
const later = '2026-09-26T20:15:00Z'

export function seedLibrary(core: Core) {
  const dm = core.accounts.dm.id
  const [ash, brook] = ids.players
  const lib = ids.library
  const entries: DemoLibraryEntry[] = [
    entry(lib.spell, dm, 'spell', 'Bellwater Toll', { level: '2', school: 'evocation' }, at, [
      { no: 2, name: 'Bellwater Toll', fields: fields({ level: '2', school: 'evocation', notes: 'Deafens on a failed save.' }), createdAt: later, origin: 'ui' },
    ]),
    entry(lib.item, dm, 'item', 'Drowned Bell Clapper', { rarity: 'uncommon', attunement: 'yes' }, at),
    entry(lib.subclass, dm, 'subclass', 'Oath of the Sunken Bell', { class: 'paladin' }, at),
    entry(lib.class, dm, 'class', 'Bellwarden', { 'hit die': 'd10' }, at),
    entry(lib.species, dm, 'species', 'Tideborn', { size: 'medium' }, at),
    entry(lib.feat, dm, 'feat', 'Harbour Guard', { category: 'general' }, at),
    entry(lib.background, dm, 'background', 'Salvage Diver', { feat: 'Skilled' }, at),
    entry(lib.condition, dm, 'condition', 'Waterlogged', { ends: 'on a rest' }, at),
    entry(lib.table, dm, 'table', 'Wreck Salvage', { dice: '1d6' }, at),
    entry(lib.monster, dm, 'creature', 'Drowned Bell-Ringer', { challenge: '2', type: 'undead' }, at),
    entry(libraryIds.npc, dm, 'npc', 'Harbourmaster Quill', { role: 'Keeps the tide ledger of Saltmere', attitude: 'wary' }, at),
    entry(libraryIds.shop, dm, 'shop', 'The Brass Anchor', { owner: 'Ida Wick', stock: 'rope, lanterns, diving bells' }, at),
    entry(libraryIds.lantern, dm, 'spell', 'Lantern of Low Tide', { level: '1', school: 'illusion' }, at),
    entry(libraryIds.sharedSpell, sharedOwner, 'spell', 'Lantern of Low Tide', { level: '1', school: 'illusion' }, at),
    entry(libraryIds.sharedTable, sharedOwner, 'table', 'Harbour Rumours', { dice: '1d8' }, at),
  ]
  const collections: DemoCollection[] = [
    { id: libraryIds.homebrew, ownerId: dm, name: 'Sunken Bell homebrew', description: 'Rules made for the drowned town of Saltmere.', entryIds: [lib.subclass, lib.species, lib.background, lib.condition], switchedOn: true },
    { id: libraryIds.bestiary, ownerId: dm, name: 'Tide bestiary', description: 'Creatures of the bay, not yet met.', entryIds: [lib.monster], switchedOn: false },
  ]
  const links: DemoLink[] = [
    { entryId: lib.spell, pinnedRevision: 1, override: [] },
    { entryId: lib.item, override: fields({ attunement: 'no' }) },
    { entryId: libraryIds.npc, override: [] },
  ]
  const proposals: DemoProposal[] = [
    {
      id: ids.proposal, authorId: ash, authorName: core.accounts.player.nickname, kind: 'item', name: "Tamsin's Tide Charm",
      fields: fields({ rarity: 'common', effect: 'Glows faintly when water is within 30 feet.' }), note: 'Found it in the bell tower, I think?',
      status: 'pending', message: '', createdAt: later, updatedAt: later,
      steps: [{ no: 1, action: 'submitted', message: 'Found it in the bell tower, I think?', by: core.accounts.player.nickname, createdAt: later }],
    },
    {
      id: libraryIds.otherProposal, authorId: brook, authorName: 'Brook', kind: 'spell', name: 'Bellwater Toll',
      fields: fields({ level: '2', school: 'evocation', notes: 'Deafens and pushes 10 feet on a failed save.' }), note: 'A push would suit a bell.',
      status: 'changes_requested', message: 'Keep the push to 5 feet.', baseEntryId: lib.spell, createdAt: at, updatedAt: later,
      steps: [
        { no: 1, action: 'submitted', message: 'A push would suit a bell.', by: 'Brook', createdAt: at },
        { no: 2, action: 'changes_requested', message: 'Keep the push to 5 feet.', by: core.accounts.dm.nickname, createdAt: later },
      ],
    },
  ]
  const submissions: DemoSubmission[] = [
    {
      id: libraryIds.submission, submitterId: dm, entryId: lib.table, revision: 1, kind: 'table', name: 'Wreck Salvage', fields: fields({ dice: '1d6' }),
      note: 'Useful for any coastal wreck.', status: 'pending', ipNote: '', message: '', createdAt: later,
    },
    {
      id: libraryIds.otherSubmission, submitterId: dm, entryId: libraryIds.lantern, revision: 1, kind: 'spell', name: 'Lantern of Low Tide',
      fields: fields({ level: '1', school: 'illusion' }), note: '', status: 'approved', ipClear: true, ipNote: 'Original text, no SRD names.', message: 'Thanks!',
      sharedEntryId: libraryIds.sharedSpell, createdAt: at, decidedAt: later,
    },
  ]
  return { library: entries, collections, links, proposals, submissions }
}
