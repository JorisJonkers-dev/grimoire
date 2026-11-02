import { afterEach, describe, expect, it, vi } from 'vitest'
import type {
  CompendiumGuides, Entry, EntryPage, LibraryCollection, LibraryEntry, LibraryEntryDetail, LinkedEntry, PartKind, Proposal, ProposalDetail,
  SharedSubmission, Spell, SpellBuild, SpellPage,
} from '@/infrastructure/api/types.gen'
import { mountDemo } from '@/test/mountDemo'
import { createDemo, type Demo } from '../demo'
import { ids } from '../ids'
import { viewerOf, type Persona } from '../role'
import { libraryIds } from '../seeds/compendium/library'
import { compendiumHandlers } from './compendium'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

type Answer<T> = { status: number; body: T }

/** Asks the demo as the API would be asked, as whoever the world is looked at as now. */
function client(demo: Demo) {
  return async <T>(method: string, path: string, body?: unknown): Promise<Answer<T>> => {
    const res = await demo.fetch(`https://demo.invalid/api/v1${path}`, {
      method,
      ...(body === undefined ? {} : { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } }),
    })
    return { status: res.status, body: (res.status === 204 ? undefined : await res.json()) as T }
  }
}

function as(persona: Persona) {
  const demo = createDemo(persona)
  return { demo, ask: client(demo), become: (p: Persona) => (demo.world.persona = p) }
}

const c = `/campaigns/${ids.campaign}`
const lib = ids.library

describe('the compendium, open to anyone', () => {
  it('lists spells by name, level, school, class and ruleset, signed out', async () => {
    const { ask } = as('signed-out')
    const names = async (q: string) => (await ask<SpellPage>('GET', `/compendium/spells${q}`)).body.items.map((s) => s.slug)
    expect(await names('')).toEqual(['cure-wounds', 'detect-magic', 'fire-bolt', 'fireball', 'hold-person', 'magic-missile', 'shield'])
    expect(await names('?q=FIRE')).toEqual(['fire-bolt', 'fireball'])
    expect(await names('?level=1&school=abjuration')).toEqual(['cure-wounds', 'shield'])
    expect(await names('?class=cleric')).toEqual(['cure-wounds', 'detect-magic', 'hold-person'])
    expect(await names('?ruleset=srd-2014')).toEqual([])
  })

  it('opens a spell and its conditions, and refuses one it does not hold', async () => {
    const { ask } = as('signed-out')
    const hold = await ask<Spell>('GET', '/compendium/spells/hold-person')
    expect(hold.body.mentions.map((m) => m.slug)).toEqual(['paralyzed'])
    expect((await ask<Spell>('GET', '/compendium/spells/fireball?ruleset=srd-2024')).body.damageRoll).toBe('8d6')
    expect((await ask('GET', '/compendium/spells/fireball?ruleset=srd-2014')).status).toBe(404)
    expect((await ask('GET', '/compendium/spells/wish')).status).toBe(404)
  })

  it('holds entries of every kind the compendium shows, and opens them', async () => {
    const { ask } = as('signed-out')
    for (const kind of ['class', 'species', 'background', 'feat', 'weapon', 'armor', 'item', 'magic-item', 'monster', 'condition']) {
      expect((await ask<EntryPage>('GET', `/compendium/entries?kind=${kind}`)).body.items.length).toBeGreaterThan(0)
    }
    expect((await ask<EntryPage>('GET', '/compendium/entries?kind=monster&q=goblin')).body.items.map((e) => e.slug)).toEqual(['goblin-warrior'])
    const goblin = await ask<Entry>('GET', '/compendium/entries/monster/goblin-warrior?ruleset=srd-2024')
    expect(goblin.body.sections.map((s) => s.title)).toContain('Nimble Escape')
    expect((await ask('GET', '/compendium/entries/monster/beholder')).status).toBe(404)
  })

  it('counts automation per kind, names its source, and works the guides out of its own records', async () => {
    const { ask } = as('signed-out')
    const coverage = (await ask<{ kind: string; total: number; full: number; partial: number; manual: number }[]>('GET', '/compendium/automation')).body
    expect(coverage.find((k) => k.kind === 'spell')).toEqual({ kind: 'spell', total: 7, full: 5, partial: 1, manual: 1 })
    expect((await ask<{ license: string }[]>('GET', '/compendium/sources')).body.map((s) => s.license)).toEqual(['CC-BY-4.0'])
    const guides = (await ask<CompendiumGuides>('GET', '/compendium/guides')).body
    expect(guides.spellsByLevel.map((l) => l.level)).toEqual([0, 1, 2, 3])
    expect(guides.attacksByChallenge.map((g) => g.monsters)).toEqual([3])
    expect(guides.lootByRarity.map((r) => [r.rarity, r.items.map((i) => i.slug)])).toEqual([['common', ['potion-of-healing']], ['uncommon', ['bag-of-holding']]])
    const old = (await ask<CompendiumGuides>('GET', '/compendium/guides?ruleset=srd-2014')).body
    expect([old.spellsByLevel, old.attacksByChallenge, old.lootByRarity]).toEqual([[], [], []])
  })
})

describe('the Library, each Account its own', () => {
  it('lists the DM their entries and nobody else them', async () => {
    const { ask, become } = as('dm')
    const mine = (await ask<LibraryEntry[]>('GET', '/library')).body
    expect(mine.map((e) => e.id)).toEqual(expect.arrayContaining(Object.values(lib)))
    expect((await ask<LibraryEntry[]>('GET', '/library?kind=creature')).body.map((e) => e.name)).toEqual(['Drowned Bell-Ringer'])
    become('player')
    expect((await ask<LibraryEntry[]>('GET', '/library')).body).toEqual([])
    expect((await ask('GET', `/library/${lib.spell}`)).status).toBe(404)
    become('signed-out')
    expect((await ask('GET', '/library')).status).toBe(401)
  })

  it('shows an entry with its Revisions, newest first, and the Campaigns it is in', async () => {
    const { ask } = as('dm')
    const spell = (await ask<LibraryEntryDetail>('GET', `/library/${lib.spell}`)).body
    expect(spell.revisions.map((r) => r.no)).toEqual([2, 1])
    expect(spell.uses).toEqual([{ campaignId: ids.campaign, campaign: 'The Sunken Bell', pinnedRevision: 1 }])
    expect((await ask<LibraryEntryDetail>('GET', `/library/${lib.item}`)).body.uses).toEqual([{ campaignId: ids.campaign, campaign: 'The Sunken Bell' }])
    expect((await ask<LibraryEntryDetail>('GET', `/library/${libraryIds.lantern}`)).body.uses).toEqual([])
  })

  it('creates, edits and restores an entry as new Revisions, for its owner alone', async () => {
    const { demo, ask, become } = as('dm')
    expect((await ask('POST', '/library', { kind: 'npc', name: '  ', fields: [] })).status).toBe(422)
    const made = (await ask<LibraryEntry>('POST', '/library', { kind: 'npc', name: ' Old Mother Kelp ', fields: [{ name: 'role', value: 'sea witch' }] })).body
    expect([made.name, made.revision]).toEqual(['Old Mother Kelp', 1])
    const edited = (await ask<LibraryEntryDetail>('PUT', `/library/${made.id}`, { name: 'Mother Kelp', fields: [{ name: 'role', value: 'hag' }] })).body
    expect([edited.entry.name, edited.entry.revision]).toEqual(['Mother Kelp', 2])
    // The router reads a number in a path as text, so a restore is asked of its handler directly.
    const restore = (revisionNo: number) =>
      compendiumHandlers.restoreLibraryRevision({ path: { entryId: made.id, revisionNo }, query: undefined, body: undefined, world: demo.world, who: viewerOf(demo.world) })
    const back = restore(1)
    expect([back.entry.name, back.entry.revision, back.entry.fields]).toEqual(['Old Mother Kelp', 3, [{ name: 'role', value: 'sea witch' }]])
    expect(() => restore(9)).toThrow('Revision not found')
    become('player')
    expect((await ask('PUT', `/library/${made.id}`, { name: 'Mine now', fields: [] })).status).toBe(404)
  })

  it('keeps each Account its Collections', async () => {
    const { ask, become } = as('dm')
    expect((await ask<LibraryCollection[]>('GET', '/library/collections')).body.map((x) => [x.name, x.switchedOn])).toEqual([['Sunken Bell homebrew', undefined], ['Tide bestiary', undefined]])
    expect((await ask<LibraryCollection>('POST', '/library/collections', { name: 'Saltmere', description: 'The town.' })).body.description).toBe('The town.')
    expect((await ask<LibraryCollection>('POST', '/library/collections', { name: 'Loose ends' })).body).toMatchObject({ description: '', entryIds: [], mine: true })
    become('player')
    expect((await ask<LibraryCollection[]>('GET', '/library/collections')).body).toEqual([])
  })
})

describe("a Campaign's Library, the DM's alone", () => {
  it('shows the DM what the Campaign sees, linked or through a switched-on Collection', async () => {
    const { ask } = as('dm')
    const seen = (await ask<LinkedEntry[]>('GET', `${c}/library`)).body
    const spell = seen.find((l) => l.entry.id === lib.spell) as LinkedEntry
    expect([spell.direct, spell.pinnedRevision, spell.baseName, spell.base.length]).toEqual([true, 1, 'Bellwater Toll', 2])
    const item = seen.find((l) => l.entry.id === lib.item) as LinkedEntry
    expect(item.fields).toEqual([{ name: 'attunement', value: 'no' }, { name: 'rarity', value: 'uncommon' }])
    expect(seen.find((l) => l.entry.id === lib.species)).toMatchObject({ direct: false, via: ['Sunken Bell homebrew'], override: [] })
    expect(seen.some((l) => l.entry.id === lib.monster)).toBe(false)
    expect((await ask<LibraryCollection[]>('GET', `${c}/collections`)).body.map((x) => [x.name, x.switchedOn])).toEqual([['Sunken Bell homebrew', true], ['Tide bestiary', false]])
  })

  it('refuses a Player, and a stranger is told there is no such Campaign', async () => {
    const { ask, become } = as('player')
    expect((await ask('GET', `${c}/library`)).status).toBe(403)
    expect((await ask('GET', `${c}/collections`)).status).toBe(403)
    expect((await ask('POST', `${c}/library`, { entryId: lib.spell })).status).toBe(403)
    become('admin')
    expect((await ask('GET', `${c}/library`)).status).toBe(404)
    become('signed-out')
    expect((await ask('GET', `${c}/library`)).status).toBe(401)
  })

  it('links a Shared Library copy in once and unlinks it', async () => {
    const { ask } = as('dm')
    const first = (await ask<LinkedEntry>('POST', `${c}/library`, { entryId: libraryIds.sharedSpell })).body
    expect([first.direct, first.entry.shared]).toEqual([true, true])
    await ask('POST', `${c}/library`, { entryId: libraryIds.sharedSpell })
    expect((await ask<LinkedEntry[]>('GET', `${c}/library`)).body.filter((l) => l.entry.id === libraryIds.sharedSpell)).toHaveLength(1)
    // Where a shared copy is used is nobody's business but the Campaign's own.
    expect((await ask<LibraryEntryDetail>('GET', `/library/${libraryIds.sharedSpell}`)).body.uses).toEqual([])
    expect((await ask('DELETE', `${c}/library/${libraryIds.sharedSpell}`)).status).toBe(204)
    expect((await ask('DELETE', `${c}/library/${libraryIds.sharedSpell}`)).status).toBe(404)
  })
})

describe('the Shared Library', () => {
  it('shows every signed-in caller the shared copies, read-only', async () => {
    const { ask, become } = as('player')
    expect((await ask<LibraryEntry[]>('GET', '/shared-library')).body.map((e) => [e.name, e.shared])).toEqual([['Harbour Rumours', true], ['Lantern of Low Tide', true]])
    expect((await ask<LibraryEntry[]>('GET', '/shared-library?kind=table')).body.map((e) => e.name)).toEqual(['Harbour Rumours'])
    expect((await ask<LibraryEntryDetail>('GET', `/library/${libraryIds.sharedTable}`)).body.entry.shared).toBe(true)
    become('signed-out')
    expect((await ask('GET', '/shared-library')).status).toBe(401)
  })

  it("lists a caller's own requests to share, and every request to an Admin alone", async () => {
    const { ask, become } = as('dm')
    expect((await ask<SharedSubmission[]>('GET', '/shared-library/submissions')).body.map((s) => s.status)).toEqual(['pending', 'approved'])
    expect((await ask('GET', '/admin/shared-library')).status).toBe(403)
    become('player')
    expect((await ask<SharedSubmission[]>('GET', '/shared-library/submissions')).body).toEqual([])
    become('admin')
    const all = (await ask<SharedSubmission[]>('GET', '/admin/shared-library')).body
    expect(all).toHaveLength(2)
    expect(all[0]).not.toHaveProperty('submitterId')
  })
})

describe('the builders', () => {
  const routes = [
    ['spells', lib.spell, 'Bellwater Toll'], ['items', lib.item, 'Drowned Bell Clapper'], ['subclasses', lib.subclass, 'Oath of the Sunken Bell'],
    ['classes', lib.class, 'Bellwarden'], ['species', lib.species, 'Tideborn'], ['feats', lib.feat, 'Harbour Guard'],
    ['backgrounds', lib.background, 'Salvage Diver'], ['conditions', lib.condition, 'Waterlogged'], ['roll-tables', lib.table, 'Wreck Salvage'],
    ['monsters', lib.monster, 'Drowned Bell-Ringer'],
  ] as const

  it('opens each of the DM entries in its own builder, and no other', async () => {
    const { ask } = as('dm')
    for (const [route, id, name] of routes) {
      const built = await ask<{ entry: LibraryEntry; design: unknown }>('GET', `/builders/${route}/${id}`)
      expect([built.status, built.body.entry.name]).toEqual([200, name])
    }
    expect((await ask('GET', `/builders/monsters/${lib.spell}`)).status).toBe(422)
  })

  it('opens a new entry where its builder starts, and a shared copy read-only', async () => {
    const { ask } = as('dm')
    const kinds = { spells: 'spell', items: 'item', subclasses: 'subclass', classes: 'class', species: 'species', feats: 'feat', backgrounds: 'background', conditions: 'condition', 'roll-tables': 'table', monsters: 'creature' } as const
    for (const [route, kind] of Object.entries(kinds)) {
      const made = (await ask<LibraryEntry>('POST', '/library', { kind, name: 'Fresh', fields: [] })).body
      expect((await ask<{ entry: LibraryEntry }>('GET', `/builders/${route}/${made.id}`)).body.entry.revision).toBe(1)
    }
    const shared = (await ask<SpellBuild>('GET', `/builders/spells/${libraryIds.sharedSpell}`)).body
    expect([shared.entry?.shared, shared.design.parts]).toEqual([true, []])
  })

  it("keeps the DM's builds from a Player, and serves the kinds of Part to anyone signed in", async () => {
    const { ask, become } = as('player')
    expect((await ask('GET', `/builders/spells/${lib.spell}`)).status).toBe(404)
    expect((await ask<PartKind[]>('GET', '/builders/part-kinds')).body.length).toBeGreaterThan(0)
    become('signed-out')
    expect((await ask('GET', '/builders/part-kinds')).status).toBe(401)
  })
})

describe('Proposals: every one for a DM, a Player their own', () => {
  it('lists and opens them by audience', async () => {
    const { ask, become } = as('dm')
    expect((await ask<Proposal[]>('GET', `${c}/proposals`)).body.map((p) => p.id)).toEqual([ids.proposal, libraryIds.otherProposal])
    const other = (await ask<ProposalDetail>('GET', `${c}/proposals/${libraryIds.otherProposal}`)).body
    expect([other.steps.length, other.current?.entry.id]).toEqual([2, lib.spell])
    become('player')
    expect((await ask<Proposal[]>('GET', `${c}/proposals`)).body.map((p) => [p.id, p.authorName])).toEqual([[ids.proposal, 'Ash']])
    const own = (await ask<ProposalDetail>('GET', `${c}/proposals/${ids.proposal}`)).body
    expect([own.proposal.status, own.current]).toEqual(['pending', undefined])
    expect(own.proposal).not.toHaveProperty('authorId')
    expect((await ask('GET', `${c}/proposals/${libraryIds.otherProposal}`)).status).toBe(404)
    become('admin')
    expect((await ask('GET', `${c}/proposals`)).status).toBe(404)
  })

  it('lets a Player propose a new entry or a change to one the Campaign sees', async () => {
    const { ask } = as('player')
    const fresh = await ask<Proposal>('POST', `${c}/proposals`, { kind: 'item', name: 'Kelp Rope', fields: [] })
    expect([fresh.status, fresh.body.note, fresh.body.status]).toEqual([200, '', 'pending'])
    const change = await ask<Proposal>('POST', `${c}/proposals`, { kind: 'species', name: 'Tideborn', fields: [], note: 'Gills?', baseEntryId: lib.species })
    expect([change.body.baseEntryId, change.body.note]).toEqual([lib.species, 'Gills?'])
    expect((await ask('POST', `${c}/proposals`, { kind: 'creature', name: 'Bell-Ringer', fields: [], baseEntryId: lib.monster })).status).toBe(404)
  })

  it('lets the DM approve, ask for changes or decline, and a Player none of it', async () => {
    const { ask, become } = as('player')
    const propose = async (body: object) => (await ask<Proposal>('POST', `${c}/proposals`, { kind: 'item', name: 'Kelp Rope', fields: [], ...body })).body.id
    const review = (id: string, body: object) => ask<ProposalDetail>('POST', `${c}/proposals/${id}/review`, body)
    const kelp = await propose({})
    const net = await propose({ name: 'Kelp Net' })
    const change = await propose({ kind: 'background', name: 'Salvage Diver', baseEntryId: lib.background, fields: [{ name: 'feat', value: 'Alert' }] })
    const nope = await propose({ name: 'Bottomless Rope' })
    expect((await review(ids.proposal, { action: 'approve' })).status).toBe(403)

    become('dm')
    const approved = (await review(kelp, { action: 'approve' })).body
    expect([approved.proposal.status, approved.steps.map((s) => s.action)]).toEqual(['approved', ['submitted', 'approved']])
    expect((await review(kelp, { action: 'approve' })).status).toBe(422)
    const edited = (await review(net, { action: 'approve', name: 'Kelp Net (fine)', fields: [{ name: 'weight', value: '3 lb.' }] })).body.proposal
    expect([edited.name, edited.fields]).toEqual(['Kelp Net (fine)', [{ name: 'weight', value: '3 lb.' }]])
    const home = (await ask<LibraryCollection[]>('GET', '/library/collections')).body.find((x) => x.name === 'Campaign Collection')
    expect(home?.entryIds).toEqual([approved.proposal.entryId, edited.entryId])
    expect((await ask<LinkedEntry[]>('GET', `${c}/library`)).body.some((l) => l.entry.id === edited.entryId)).toBe(true)

    const adopted = (await review(change, { action: 'approve', message: 'Fair.' })).body.proposal
    expect(adopted.entryId).toBe(lib.background)
    expect((await ask<LibraryEntryDetail>('GET', `/library/${lib.background}`)).body.entry.revision).toBe(2)

    expect((await review(nope, { action: 'request_changes' })).status).toBe(422)
    expect((await review(nope, { action: 'request_changes', message: ' Shorter, please. ' })).body.proposal).toMatchObject({ status: 'changes_requested', message: 'Shorter, please.' })
    expect((await review(ids.proposal, { action: 'decline' })).body.proposal.status).toBe('declined')
  })
})

describe('the pages', () => {
  it('show a DM every Proposal and a Player only their own', async () => {
    const dmView = await mountDemo(`${c}/proposals`, 'dm')
    expect(dmView.wrapper.text()).toContain('Bellwater Toll')
    expect(dmView.wrapper.text()).toContain("Tamsin's Tide Charm")
    dmView.wrapper.unmount()
    const playerView = await mountDemo(`${c}/proposals`, 'player')
    expect(playerView.wrapper.text()).toContain("Tamsin's Tide Charm")
    expect(playerView.wrapper.text()).not.toContain('Bellwater Toll')
    playerView.wrapper.unmount()
  })

  it('open a spell and a monster from the compendium signed out', async () => {
    const spell = await mountDemo('/compendium/spells/fireball', 'signed-out')
    expect(spell.wrapper.text()).toContain('20-foot-radius Sphere')
    spell.wrapper.unmount()
    const monster = await mountDemo('/compendium/monster/goblin-warrior', 'signed-out')
    expect(monster.wrapper.text()).toContain('Nimble Escape')
    monster.wrapper.unmount()
  })
})
