import { entryKinds } from '@/features/compendium/kinds'
import type {
  AutomationCount, CompendiumGuides, LibraryEntry, LibraryEntryDetail, LibraryField, LibraryRevision, LinkedEntry, PartKind, Proposal, ProposalDetail,
  SharedSubmission, Spell,
} from '@/infrastructure/api/types.gen'
import served from '../../../../fixtures/part-kinds.json'
import { DemoProblem, notFound, type Handlers } from '../operations'
import type { Viewer } from '../role'
import { blank, type BuilderKind, type Builds } from '../seeds/compendium/builds'
import { sharedOwner, type DemoLibraryEntry, type DemoProposal, type DemoSubmission } from '../seeds/compendium/library'
import type { World } from '../world'
import { admin, dm, member, signedIn } from './access'

// The compendium is open to anyone, as in production. The Library is each Account's own: anyone
// else's entry looks like no entry at all, and a Shared Library copy is read-only to every caller.
// What a Campaign sees of the Library, and its Proposals, follow the API's DM and author rules.

const refuse = (reason: string) => new DemoProblem(422, reason)
const now = () => new Date().toISOString()
const byName = <T extends { name: string }>(a: T, b: T) => a.name.localeCompare(b.name)

/** A record without the demo's own bookkeeping, as the API would send it. */
function without<T extends object, K extends keyof T>(record: T, keys: K[]): Omit<T, K> {
  return Object.fromEntries(Object.entries(record).filter(([k]) => !keys.includes(k as K))) as Omit<T, K>
}

// ---- Compendium ---------------------------------------------------------------------------------

type Ruleset = { ruleset?: Spell['ruleset'] } | undefined
/** A request's query; without a query string the router sends none at all, so this reads it as empty. */
const given = <T extends object>(query: T | undefined): Partial<T> => ({ ...query })
const inRules = (query: Ruleset, ruleset: string) => !query?.ruleset || query.ruleset === ruleset
const named = (q: string | undefined, name: string) => !q || name.toLowerCase().includes(q.toLowerCase())

function guides(world: World, query: Ruleset): CompendiumGuides {
  const spells = world.compendium.spells.filter((s) => inRules(query, s.ruleset))
  const entries = world.compendium.entries.filter((e) => inRules(query, e.ruleset))
  const levels = [...new Set(spells.map((s) => s.level))].sort((a, b) => a - b)
  const monsters = entries.filter((e) => e.kind === 'monster')
  const magic = entries.filter((e) => e.kind === 'magic-item')
  const rarities = ['common', 'uncommon', 'rare', 'very-rare', 'legendary']
  return {
    spellsByLevel: levels.map((level) => ({
      level,
      spells: spells.filter((s) => s.level === level).sort(byName).map((s) => ({ slug: s.slug, name: s.name, school: s.school })),
    })),
    // Every seeded monster is a Challenge 1/4 creature; these are their attacks, worked by hand.
    attacksByChallenge: monsters.length > 0 ? [{ challenge: '1/4', monsters: monsters.length, attacks: 5, toHitLow: 4, toHit: 4, toHitHigh: 5, damage: 5 }] : [],
    lootTiers: [1, 2, 3, 4].map((tier) => ({ tier, fromLevel: [1, 5, 11, 17][tier - 1] as number, toLevel: [4, 10, 16, 20][tier - 1] as number, rarities: rarities.slice(0, tier + 1) })),
    lootByRarity: rarities
      .map((rarity, i) => ({ rarity, firstTier: Math.max(1, i), items: magic.filter((e) => e.facts.some((f) => f.label === 'Rarity' && f.value.toLowerCase() === rarity)).map((e) => ({ slug: e.slug, name: e.name })) }))
      .filter((r) => r.items.length > 0),
  }
}

const compendium = {
  listSpells: ({ world, query }) => {
    const q = given(query)
    return {
      items: world.compendium.spells
        .filter((s) => named(q.q, s.name) && inRules(q, s.ruleset))
        .filter((s) => q.level === undefined || s.level === q.level)
        .filter((s) => !q.school || s.school === q.school)
        .filter((s) => !q.class || s.classes.includes(q.class))
        .sort(byName)
        .map(({ slug, name, level, school, ruleset, ritual, concentration }) => ({ slug, name, level, school, ruleset, ritual, concentration })),
    }
  },
  getSpell: ({ world, path, query }) => {
    const found = world.compendium.spells.find((s) => s.slug === path.slug && inRules(query, s.ruleset))
    if (!found) throw notFound('Spell')
    return found
  },
  listEntries: ({ world, query }) => ({
    items: world.compendium.entries
      .filter((e) => e.kind === query.kind && named(query.q, e.name) && inRules(query, e.ruleset))
      .sort(byName)
      .map(({ kind, slug, name, subtitle, ruleset }) => ({ kind, slug, name, subtitle, ruleset })),
  }),
  getEntry: ({ world, path, query }) => {
    const found = world.compendium.entries.find((e) => e.kind === path.kind && e.slug === path.slug && inRules(query, e.ruleset))
    if (!found) throw notFound('Entry')
    return found
  },
  getAutomationCoverage: ({ world }) => {
    const records = [...world.compendium.spells.map((s) => ({ kind: 'spell', slug: s.slug })), ...world.compendium.entries]
    const level = (slug: string) => world.compendium.automation[slug] ?? 'full'
    return (['spell', ...entryKinds.map((k) => k.kind)] as AutomationCount['kind'][]).map((kind) => {
      const mine = records.filter((r) => r.kind === kind)
      const count = (l: string) => mine.filter((r) => level(r.slug) === l).length
      return { kind, total: mine.length, full: count('full'), partial: count('partial'), manual: count('manual') }
    })
  },
  listSources: ({ world }) => world.compendium.sources,
  getCompendiumGuides: ({ world, query }) => guides(world, query),
} satisfies Handlers

// ---- Library ------------------------------------------------------------------------------------

/** An entry's newest Revision; every entry has at least its first. */
const latest = (e: DemoLibraryEntry) => e.revisions.at(-1) as LibraryRevision

function shown(e: DemoLibraryEntry): LibraryEntry {
  const revision = latest(e).no
  return { id: e.id, kind: e.kind, name: e.name, fields: e.fields, revision, ...(e.ownerId === sharedOwner ? { shared: true } : {}), createdAt: e.createdAt, updatedAt: e.updatedAt }
}

/** One of the caller's own entries; anyone else's looks like no entry at all. */
function owned(world: World, who: Viewer, id: string): DemoLibraryEntry {
  const me = signedIn(who)
  const e = world.compendium.library.find((x) => x.id === id && x.ownerId === me.id)
  if (!e) throw notFound('Library entry')
  return e
}

/** One of the caller's entries, or a Shared Library copy: what the caller may open and link. */
function linkable(world: World, who: Viewer, id: string): DemoLibraryEntry {
  const me = signedIn(who)
  const e = world.compendium.library.find((x) => x.id === id && (x.ownerId === me.id || x.ownerId === sharedOwner))
  if (!e) throw notFound('Library entry')
  return e
}

/** The fields one side wins over the other on, name by name, in name order as the API sends them. */
function merged(base: LibraryField[], over: LibraryField[]): LibraryField[] {
  const out = new Map(base.map((f) => [f.name, f.value]))
  for (const f of over) out.set(f.name, f.value)
  return [...out].sort(([a], [b]) => a.localeCompare(b)).map(([name, value]) => ({ name, value }))
}

/** Everything the seeded Campaign sees of the Library: linked by the DM, or brought in by a switched-on Collection. */
function linked(world: World): LinkedEntry[] {
  const { collections, links } = world.compendium
  const on = collections.filter((c) => c.switchedOn)
  const ids = new Set([...links.map((l) => l.entryId), ...on.flatMap((c) => c.entryIds)])
  return world.compendium.library
    .filter((e) => ids.has(e.id))
    .map((e) => {
      const link = links.find((l) => l.entryId === e.id)
      const pinned = e.revisions.find((r) => r.no === link?.pinnedRevision) ?? latest(e)
      const override = link?.override ?? []
      return {
        entry: shown(e), direct: Boolean(link), via: on.filter((c) => c.entryIds.includes(e.id)).map((c) => c.name).sort(),
        ...(link?.pinnedRevision ? { pinnedRevision: link.pinnedRevision } : {}),
        baseName: pinned.name, base: pinned.fields, override, fields: merged(pinned.fields, override),
      }
    })
    .sort((a, b) => a.entry.kind.localeCompare(b.entry.kind) || byName(a.entry, b.entry))
}

function detail(world: World, e: DemoLibraryEntry): LibraryEntryDetail {
  const use = linked(world).find((l) => l.entry.id === e.id)
  // Where others use a Shared Library copy is theirs to know.
  const uses = use && e.ownerId !== sharedOwner ? [{ campaignId: world.campaign.id, campaign: world.campaign.name, ...(use.pinnedRevision ? { pinnedRevision: use.pinnedRevision } : {}) }] : []
  return { entry: shown(e), revisions: [...e.revisions].reverse(), uses }
}

function revise(e: DemoLibraryEntry, name: string, fields: LibraryField[]) {
  const at = now()
  const no = latest(e).no + 1
  e.revisions.push({ no, name, fields, createdAt: at, origin: 'ui' })
  e.name = name
  e.fields = fields
  e.updatedAt = at
}

function cleanName(name: string): string {
  const out = name.trim()
  if (!out) throw refuse('give it a name of up to 80 characters')
  return out
}

function addEntry(world: World, ownerId: string, kind: DemoLibraryEntry['kind'], name: string, fields: LibraryField[]): DemoLibraryEntry {
  const at = now()
  const clean = merged(fields, [])
  const e: DemoLibraryEntry = { id: crypto.randomUUID(), ownerId, kind, name, fields: clean, createdAt: at, updatedAt: at, revisions: [{ no: 1, name, fields: clean, createdAt: at, origin: 'ui' }] }
  world.compendium.library.push(e)
  return e
}

const collectionOf = (c: World['compendium']['collections'][number], withSwitch: boolean) => ({
  id: c.id, name: c.name, description: c.description, entryIds: c.entryIds, mine: true, ...(withSwitch ? { switchedOn: c.switchedOn } : {}),
})

const submissionOf = (s: DemoSubmission): SharedSubmission => without(s, ['submitterId'])

const library = {
  listLibraryEntries: ({ world, who, query }) => {
    const me = signedIn(who)
    return world.compendium.library
      .filter((e) => e.ownerId === me.id && (!given(query).kind || e.kind === given(query).kind))
      .sort((a, b) => a.kind.localeCompare(b.kind) || byName(a, b))
      .map(shown)
  },
  createLibraryEntry: ({ world, who, body }) => shown(addEntry(world, signedIn(who).id, body.kind, cleanName(body.name), body.fields)),
  getLibraryEntry: ({ world, who, path }) => detail(world, linkable(world, who, path.entryId)),
  updateLibraryEntry: ({ world, who, path, body }) => {
    const e = owned(world, who, path.entryId)
    revise(e, cleanName(body.name), merged(body.fields, []))
    return detail(world, e)
  },
  restoreLibraryRevision: ({ world, who, path }) => {
    const e = owned(world, who, path.entryId)
    const old = e.revisions.find((r) => r.no === path.revisionNo)
    if (!old) throw notFound('Revision')
    revise(e, old.name, old.fields)
    return detail(world, e)
  },
  listLibraryCollections: ({ world, who }) => {
    const me = signedIn(who)
    return world.compendium.collections.filter((c) => c.ownerId === me.id).sort(byName).map((c) => collectionOf(c, false))
  },
  createLibraryCollection: ({ world, who, body }) => {
    const c = { id: crypto.randomUUID(), ownerId: signedIn(who).id, name: cleanName(body.name), description: body.description ?? '', entryIds: [], switchedOn: false }
    world.compendium.collections.push(c)
    return collectionOf(c, false)
  },
  listCampaignCollections: ({ world, who, path }) => {
    dm(who, path.campaignId, world)
    const me = signedIn(who)
    return world.compendium.collections.filter((c) => c.ownerId === me.id).sort(byName).map((c) => collectionOf(c, true))
  },
  listLinkedEntries: ({ world, who, path }) => {
    dm(who, path.campaignId, world)
    return linked(world)
  },
  linkLibraryEntry: ({ world, who, path, body }) => {
    dm(who, path.campaignId, world)
    const e = linkable(world, who, body.entryId)
    if (!world.compendium.links.some((l) => l.entryId === e.id)) world.compendium.links.push({ entryId: e.id, override: [] })
    return linked(world).find((l) => l.entry.id === e.id) as LinkedEntry
  },
  unlinkLibraryEntry: ({ world, who, path }) => {
    dm(who, path.campaignId, world)
    const at = world.compendium.links.findIndex((l) => l.entryId === path.entryId)
    if (at < 0) throw notFound('Link')
    world.compendium.links.splice(at, 1)
    return undefined
  },
  listSharedEntries: ({ world, who, query }) => {
    signedIn(who)
    return world.compendium.library.filter((e) => e.ownerId === sharedOwner && (!given(query).kind || e.kind === given(query).kind)).sort(byName).map(shown)
  },
  listMySubmissions: ({ world, who }) => {
    const me = signedIn(who)
    return world.compendium.submissions.filter((s) => s.submitterId === me.id).map(submissionOf)
  },
  listSharedSubmissions: ({ world, who }) => {
    admin(who)
    return world.compendium.submissions.map(submissionOf)
  },
} satisfies Handlers

// ---- Builders -----------------------------------------------------------------------------------

/** What a builder opens: one of the caller's entries or a Shared Library copy, of the builder's kind. */
function build<K extends BuilderKind>(world: World, who: Viewer, id: string, kind: K): Builds[K] & { entry: LibraryEntry } {
  const e = linkable(world, who, id)
  if (e.kind !== kind) throw refuse(`the builder opens only entries of kind ${kind}`)
  const builds = world.compendium.builds[kind] as Record<string, Builds[K]>
  return { ...(builds[id] ?? blank[kind]), entry: shown(e) }
}

const builders = {
  listPartKinds: ({ who }) => {
    signedIn(who)
    return served.kinds as PartKind[]
  },
  getSpellBuild: ({ world, who, path }) => build(world, who, path.entryId, 'spell'),
  getItemBuild: ({ world, who, path }) => build(world, who, path.entryId, 'item'),
  getSubclassBuild: ({ world, who, path }) => build(world, who, path.entryId, 'subclass'),
  getClassBuild: ({ world, who, path }) => build(world, who, path.entryId, 'class'),
  getSpeciesBuild: ({ world, who, path }) => build(world, who, path.entryId, 'species'),
  getFeatBuild: ({ world, who, path }) => build(world, who, path.entryId, 'feat'),
  getBackgroundBuild: ({ world, who, path }) => build(world, who, path.entryId, 'background'),
  getConditionBuild: ({ world, who, path }) => build(world, who, path.entryId, 'condition'),
  getRollTableBuild: ({ world, who, path }) => build(world, who, path.entryId, 'table'),
  getMonsterBuild: ({ world, who, path }) => build(world, who, path.entryId, 'creature'),
} satisfies Handlers

// ---- Proposals ----------------------------------------------------------------------------------

const proposalOf = (p: DemoProposal): Proposal => without(p, ['authorId', 'steps'])

/** A Proposal its author or a DM of the Campaign may open; to anyone else it does not exist. */
function readable(world: World, who: Viewer, campaignId: string, id: string): { p: DemoProposal; role: 'dm' | 'player' } {
  const role = member(who, campaignId, world)
  const p = world.compendium.proposals.find((x) => x.id === id && (role === 'dm' || x.authorId === signedIn(who).id))
  if (!p) throw notFound('Proposal')
  return { p, role }
}

function proposalDetail(world: World, p: DemoProposal): ProposalDetail {
  const current = p.baseEntryId ? linked(world).find((l) => l.entry.id === p.baseEntryId) : undefined
  return { proposal: proposalOf(p), steps: p.steps, ...(current ? { current } : {}) }
}

/** The Campaign Collection an approval files into, made and switched on the first time one needs it. */
function home(world: World, ownerId: string) {
  const name = 'Campaign Collection'
  let c = world.compendium.collections.find((x) => x.name === name && x.ownerId === ownerId)
  if (!c) {
    c = { id: crypto.randomUUID(), ownerId, name, description: 'Entries made or approved for this Campaign.', entryIds: [], switchedOn: true }
    world.compendium.collections.push(c)
  }
  return c
}

/** Puts an approved Proposal in the DM's Library: a new Revision of their own entry it changes, a new entry otherwise. */
function adopt(world: World, who: Viewer, p: DemoProposal) {
  const me = signedIn(who)
  const base = world.compendium.library.find((e) => e.id === p.baseEntryId && e.ownerId === me.id)
  if (base) {
    revise(base, p.name, p.fields)
    p.entryId = base.id
    return
  }
  const e = addEntry(world, me.id, p.kind, p.name, p.fields)
  p.entryId = e.id
  home(world, me.id).entryIds.push(e.id)
}

const actions = { approve: 'approved', request_changes: 'changes_requested', decline: 'declined' } as const

const proposals = {
  listProposals: ({ world, who, path }) => {
    const role = member(who, path.campaignId, world)
    const me = signedIn(who)
    return world.compendium.proposals
      .filter((p) => role === 'dm' || p.authorId === me.id)
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
      .map(proposalOf)
  },
  getProposal: ({ world, who, path }) => proposalDetail(world, readable(world, who, path.campaignId, path.proposalId).p),
  createProposal: ({ world, who, path, body }) => {
    member(who, path.campaignId, world)
    const me = signedIn(who)
    if (body.baseEntryId && !linked(world).some((l) => l.entry.id === body.baseEntryId)) throw notFound('Library entry')
    const at = now()
    const note = body.note ?? ''
    const p: DemoProposal = {
      id: crypto.randomUUID(), authorId: me.id, authorName: me.nickname, kind: body.kind, name: cleanName(body.name), fields: merged(body.fields, []), note,
      status: 'pending', message: '', ...(body.baseEntryId ? { baseEntryId: body.baseEntryId } : {}), createdAt: at, updatedAt: at,
      steps: [{ no: 1, action: 'submitted', message: note, by: me.nickname, createdAt: at }],
    }
    world.compendium.proposals.push(p)
    return proposalOf(p)
  },
  reviewProposal: ({ world, who, path, body }) => {
    const { p, role } = readable(world, who, path.campaignId, path.proposalId)
    if (role !== 'dm') throw new DemoProblem(403, 'Forbidden')
    if (p.status !== 'pending') throw refuse('this Proposal is not waiting for a review')
    const message = body.message?.trim() ?? ''
    if (body.action === 'request_changes' && !message) throw refuse('say what to change')
    const status = actions[body.action]
    if (body.action === 'approve') {
      p.name = body.name?.trim() || p.name
      p.fields = body.fields ? merged(body.fields, []) : p.fields
      adopt(world, who, p)
    }
    const at = now()
    p.status = status
    p.message = message
    p.updatedAt = at
    p.steps.push({ no: p.steps.length + 1, action: status, message, by: signedIn(who).nickname, createdAt: at })
    return proposalDetail(world, p)
  },
} satisfies Handlers

export const compendiumHandlers = { ...compendium, ...library, ...builders, ...proposals } satisfies Handlers
