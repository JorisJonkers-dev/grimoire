import { describe, expect, it } from 'vitest'
import type { ActionEntry, Faction, Journal, Npc, Revision, RollRequest, RuleHooks, RuleVariant } from '@/infrastructure/api/types.gen'
import { createDemo, type Demo } from '../demo'
import { ids } from '../ids'
import type { Persona } from '../role'
import { prepIds } from '../seeds/prep/ids'
import { tierOf } from './prep'

const c = `/campaigns/${ids.campaign}`

// eslint-disable-next-line @typescript-eslint/no-unnecessary-type-parameters -- the test names the body it expects, as a typed client would
async function ask<T = unknown>(demo: Demo, method: string, path: string, body?: unknown): Promise<{ status: number; body: T }> {
  const init: RequestInit = { method, ...(body === undefined ? {} : { body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } }) }
  const res = await demo.fetch(`https://demo.invalid/api/v1${path}`, init)
  return { status: res.status, body: (res.status === 204 ? undefined : await res.json()) as T }
}

const as = (persona: Persona) => createDemo(persona)

describe('the DM’s prep is the DM’s alone', () => {
  const dmOnly = [
    `${c}/npcs`, `${c}/npcs/${ids.npc}`, `${c}/npcs/${ids.npc}/revisions`, `${c}/npcs/deleted`,
    `${c}/encounter-pools`, `${c}/encounter-tables`, `${c}/encounter-checks`, `${c}/locations`, `${c}/loot-tables`,
    `${c}/settlements`, `${c}/shops`, `${c}/activity`, `${c}/log`, `${c}/sessions/${ids.session}/log`, `${c}/maps`, `${c}/maps/${ids.map}`,
  ]

  it.each(dmOnly)('%s answers the DM and refuses a Player', async (path) => {
    expect((await ask(as('dm'), 'GET', path)).status).toBe(200)
    expect((await ask(as('player'), 'GET', path)).status).toBe(403)
  })

  it('tells a caller outside the Campaign it does not exist, and one signed out to sign in', async () => {
    expect((await ask(as('admin'), 'GET', `${c}/npcs`)).status).toBe(404)
    expect((await ask(as('signed-out'), 'GET', `${c}/npcs`)).status).toBe(401)
    expect((await ask(as('dm'), 'GET', `/campaigns/${ids.outsider}/npcs`)).status).toBe(404)
  })

  it('seeds a few records of each list', async () => {
    const dm = as('dm')
    for (const path of dmOnly.filter((p) => !p.includes('/deleted') && !/\/(npcs|maps)\/[^/]+$/.test(p))) {
      expect((await ask<unknown[]>(dm, 'GET', path)).body.length).toBeGreaterThan(0)
    }
  })

  it('says a record it does not have is not found', async () => {
    const dm = as('dm')
    for (const path of [`${c}/npcs/${ids.outsider}`, `${c}/npcs/${ids.outsider}/revisions`, `${c}/maps/${ids.outsider}`, `${c}/sessions/${ids.outsider}`, `${c}/sessions/${ids.outsider}/log`]) {
      expect((await ask(dm, 'GET', path)).status).toBe(404)
    }
  })

  it('reads an NPC with no history as having none', async () => {
    const dm = as('dm')
    dm.world.prep.npcRevisions = {}
    expect((await ask(dm, 'GET', `${c}/npcs/${ids.npc}/revisions`)).body).toEqual([])
  })

  it('cuts the Action Logs to the limit asked for', async () => {
    const dm = as('dm')
    expect((await ask<ActionEntry[]>(dm, 'GET', `${c}/log?limit=2`)).body).toHaveLength(2)
    expect((await ask<unknown[]>(dm, 'GET', `${c}/sessions/${ids.session}/log?limit=1`)).body).toHaveLength(1)
  })
})

describe('NPCs are kept with a Revision for every change', () => {
  it('creates, changes and deletes an NPC, each attributed to the DM', async () => {
    const dm = as('dm')
    const made = await ask<Npc>(dm, 'POST', `${c}/npcs`, { name: 'Sella the net-mender', disposition: 'friendly' })
    expect(made.body).toMatchObject({ name: 'Sella the net-mender', title: '', dmNotes: '' })
    const id = made.body.id
    const changed = await ask<Npc>(dm, 'PUT', `${c}/npcs/${id}`, { name: 'Sella', title: 'Net-mender', description: 'Quick hands.', dmNotes: 'Hears everything.', disposition: 'neutral' })
    expect(changed.body).toMatchObject({ name: 'Sella', title: 'Net-mender', dmNotes: 'Hears everything.', disposition: 'neutral' })
    expect((await ask<Npc>(dm, 'PUT', `${c}/npcs/${id}`, { name: 'Sella', disposition: 'neutral' })).body).toMatchObject({ title: '', dmNotes: '' })
    expect((await ask(dm, 'DELETE', `${c}/npcs/${id}`)).status).toBe(204)
    expect((await ask<Npc[]>(dm, 'GET', `${c}/npcs`)).body.map((n) => n.id)).not.toContain(id)
    expect((await ask<{ id: string }[]>(dm, 'GET', `${c}/npcs/deleted`)).body.map((n) => n.id)).toEqual([id])
    expect(dm.world.prep.npcRevisions[id]?.map((r: Revision) => [r.no, r.action, r.author])).toEqual([[4, 'delete', 'Morgan'], [3, 'update', 'Morgan'], [2, 'update', 'Morgan'], [1, 'create', 'Morgan']])
  })

  it('refuses a Player and an NPC it does not have', async () => {
    expect((await ask(as('player'), 'POST', `${c}/npcs`, { name: 'Nobody', disposition: 'neutral' })).status).toBe(403)
    const dm = as('dm')
    expect((await ask(dm, 'PUT', `${c}/npcs/${ids.outsider}`, { name: 'Nobody', disposition: 'neutral' })).status).toBe(404)
    expect((await ask(dm, 'DELETE', `${c}/npcs/${ids.outsider}`)).status).toBe(404)
  })
})

describe('Factions show a Player tiers and shared reasons, never numbers', () => {
  it('gives the DM every secret and every change', async () => {
    const list = (await ask<Faction[]>(as('dm'), 'GET', `${c}/factions`)).body
    expect(list.map((f) => [f.name, f.tier])).toEqual([['The Bell Wardens', 'friendly'], ['The Gullrock Boatmen', 'neutral'], ['Bellhaven Harbour Watch', 'unfriendly']])
    expect(list.every((f) => f.dm !== undefined)).toBe(true)
    const watch = list[2] as Faction
    expect(watch.personal).toEqual([{ characterId: ids.characters[1], character: 'Oswin Hale', tier: 'hostile', score: -65 }])
    expect(watch.changes.map((ch) => ch.dm?.delta)).toEqual([-35, -30])
    expect(list[1]?.changes.map((ch) => ch.status)).toEqual(['pending'])
  })

  it('leaves goals, notes, scores, pending changes and unshared reasons out for a Player', async () => {
    const list = (await ask<Faction[]>(as('player'), 'GET', `${c}/factions`)).body
    const text = JSON.stringify(list)
    for (const secret of ['goals', 'territory', 'notes', 'score', 'delta', 'Captain Odo takes a cut', 'Seen talking to the Boatmen']) expect(text).not.toContain(secret)
    const [wardens, boatmen, watch] = list as [Faction, Faction, Faction]
    expect(wardens.changes).toEqual([expect.objectContaining({ rose: true, reason: 'Pulled the drowned bell-ringer from the surf.' })])
    // The Player's own Character stands with the Boatmen; the pending change is the DM's to decide.
    expect(boatmen.personal).toEqual([{ characterId: ids.characters[0], character: 'Tamsin Reed', tier: 'friendly' }])
    expect(boatmen.changes).toEqual([])
    // Another Player's Character's change and Standing stay with that Player.
    expect(watch.personal).toEqual([])
    expect(watch.changes).toEqual([expect.objectContaining({ rose: false, reason: '' })])
  })

  it('reads every score as its tier', () => {
    expect([-60, -59, -20, -19, 19, 20, 59, 60].map(tierOf)).toEqual(['hostile', 'unfriendly', 'unfriendly', 'neutral', 'neutral', 'friendly', 'friendly', 'allied'])
  })

  it('lets the DM add and remove a Faction, and only the DM', async () => {
    const dm = as('dm')
    const made = await ask<Faction>(dm, 'POST', `${c}/factions`, { name: ' The Gull Choir ' })
    expect(made.body).toMatchObject({ name: 'The Gull Choir', archetype: '', tier: 'neutral', dm: { goals: '', score: 0 } })
    const full = await ask<Faction>(dm, 'POST', `${c}/factions`, { name: 'The Lamplighters', archetype: 'merchant-league', goals: 'Light the coast road.', territory: 'The lighthouse.', notes: 'Fond of the party.' })
    expect(full.body.dm).toEqual({ goals: 'Light the coast road.', territory: 'The lighthouse.', notes: 'Fond of the party.', score: 0 })
    expect((await ask(dm, 'DELETE', `${c}/factions/${full.body.id}`)).status).toBe(204)
    expect((await ask(dm, 'DELETE', `${c}/factions/${made.body.id}`)).status).toBe(204)
    expect((await ask<Faction[]>(dm, 'GET', `${c}/factions`)).body).toHaveLength(3)
    expect((await ask(dm, 'DELETE', `${c}/factions/${made.body.id}`)).status).toBe(404)
    expect((await ask(as('player'), 'POST', `${c}/factions`, { name: 'Mine' })).status).toBe(403)
  })

  it('offers every signed-in caller the archetypes', async () => {
    expect((await ask<unknown[]>(as('admin'), 'GET', '/faction-archetypes')).body).toHaveLength(11)
    expect((await ask(as('signed-out'), 'GET', '/faction-archetypes')).status).toBe(401)
  })
})

describe('the Journal hides hidden Quests and locked Lore from a Player', () => {
  it('gives the DM all of it, with the items that unlock it', async () => {
    const j = (await ask<Journal>(as('dm'), 'GET', `${c}/journal`)).body
    expect(j.dm).toBe(true)
    expect(j.quests.map((q) => q.status)).toEqual(['completed', 'active', 'hidden'])
    expect(j.lore.map((l) => [l.unlocked, l.itemSlug])).toEqual([[true, ''], [false, 'tide-chart'], [false, 'ferrymans-letter']])
    expect(j.readable).toEqual(['tide-chart'])
  })

  it('leaves a hidden Quest and a locked Lore entry out whole for a Player', async () => {
    const j = (await ask<Journal>(as('player'), 'GET', `${c}/journal`)).body
    expect(j.dm).toBe(false)
    expect(j.quests.map((q) => q.status)).toEqual(['completed', 'active'])
    expect(j.lore).toEqual([expect.objectContaining({ title: 'The drowning of old Bellhaven', unlocked: true })])
    const text = JSON.stringify(j)
    for (const secret of ['What the Wardens buried', 'sunk on purpose', 'chapel stair stands dry', 'itemSlug']) expect(text).not.toContain(secret)
    // The Player carries the ferryman's letter and shares the tide chart in the Party Stash.
    expect(j.readable).toEqual(['tide-chart', 'ferrymans-letter'])
  })

  it('lets the DM keep Quests', async () => {
    const dm = as('dm')
    const made = await ask<{ id: string; summary: string; steps: unknown[] }>(dm, 'POST', `${c}/quests`, { name: 'Calm the gulls', status: 'active' })
    expect(made.body).toMatchObject({ summary: '', steps: [] })
    const id = made.body.id
    expect((await ask(dm, 'PUT', `${c}/quests/${id}`, { name: 'Calm the gulls', summary: 'They scream at night.', status: 'hidden', steps: [{ text: 'Ask Mother Ivel', done: false }] })).status).toBe(204)
    expect(dm.world.prep.quests.find((q) => q.id === id)).toMatchObject({ status: 'hidden', summary: 'They scream at night.' })
    expect((await ask(dm, 'PUT', `${c}/quests/${id}`, { name: 'Calm the gulls', status: 'completed' })).status).toBe(204)
    expect(dm.world.prep.quests.find((q) => q.id === id)).toMatchObject({ status: 'completed', summary: '', steps: [] })
    dm.world.persona = 'player'
    expect((await ask(dm, 'DELETE', `${c}/quests/${id}`)).status).toBe(403)
    dm.world.persona = 'dm'
    expect((await ask(dm, 'DELETE', `${c}/quests/${id}`)).status).toBe(204)
    expect((await ask(dm, 'DELETE', `${c}/quests/${id}`)).status).toBe(404)
    expect((await ask(dm, 'PUT', `${c}/quests/${id}`, { name: 'Gone', status: 'active' })).status).toBe(404)
  })
})

describe('rolls are marked for whoever may roll them', () => {
  it('lets a Player roll only their own', async () => {
    const rolls = (await ask<RollRequest[]>(as('player'), 'GET', `${c}/rolls`)).body
    expect(rolls.map((r) => [r.purpose, r.mine, r.canRoll])).toEqual([['Athletics: haul the chapel door', false, false], ['Stealth: slip past the watch', true, true]])
  })

  it('lets the DM roll any, and cuts to the limit asked for', async () => {
    const rolls = (await ask<RollRequest[]>(as('dm'), 'GET', `${c}/rolls?limit=1`)).body
    expect(rolls.map((r) => [r.mine, r.canRoll])).toEqual([[false, true]])
  })

  it('opens the Session for every Member', async () => {
    expect((await ask(as('player'), 'GET', `${c}/sessions/${ids.session}`)).body).toMatchObject({ id: ids.session, status: 'live' })
    expect((await ask(as('admin'), 'GET', `${c}/sessions/${ids.session}`)).status).toBe(404)
  })
})

describe('Rule Variants show every Member how the table plays', () => {
  it('reads what the Campaign chose, and the rules’ own way for the rest', async () => {
    const list = (await ask<RuleVariant[]>(as('player'), 'GET', `${c}/rule-variants`)).body
    const value = (slug: string) => list.find((v) => v.slug === slug)?.value
    expect([value('flanking'), value('short-rest-cap'), value('rests'), value('critical-hits')]).toEqual(['on', '2', 'standard', 'double-dice'])
  })

  it('lets the DM switch them, and only the DM', async () => {
    const dm = as('dm')
    expect((await ask(dm, 'PUT', `${c}/rule-variants`, { choices: [{ slug: 'rests', value: 'gritty' }] })).status).toBe(204)
    expect((await ask<RuleVariant[]>(dm, 'GET', `${c}/rule-variants`)).body.find((v) => v.slug === 'rests')?.value).toBe('gritty')
    expect((await ask(as('player'), 'PUT', `${c}/rule-variants`, { choices: [] })).status).toBe(403)
  })

  it('shows the DM’s own Rule Variants to everyone, and the Roll Tables to the DM', async () => {
    const player = (await ask<RuleHooks>(as('player'), 'GET', `${c}/rule-hooks`)).body
    expect([player.dm, player.hooks.length, player.points.length, player.tables]).toEqual([false, 1, 5, []])
    const dm = as('dm')
    expect((await ask<RuleHooks>(dm, 'GET', `${c}/rule-hooks`)).body.tables).toEqual([{ id: ids.library.table, name: 'Shipboard mishaps' }])
    const byTable = await ask<{ id: string; tableName?: string }>(dm, 'POST', `${c}/rule-hooks`, { name: 'Overboard', hook: 'drop-to-0', rollTableId: ids.library.table })
    expect(byTable.body.tableName).toBe('Shipboard mishaps')
    const byEffect = await ask<{ id: string; effect?: string; rollTableId?: string }>(dm, 'POST', `${c}/rule-hooks`, { name: 'Seasick', hook: 'rest', effect: 'poisoned' })
    expect(byEffect.body).toMatchObject({ effect: 'poisoned' })
    expect(byEffect.body.rollTableId).toBeUndefined()
    expect((await ask(dm, 'DELETE', `${c}/rule-hooks/${byTable.body.id}`)).status).toBe(204)
    expect((await ask(dm, 'DELETE', `${c}/rule-hooks/${byTable.body.id}`)).status).toBe(404)
    expect((await ask(as('player'), 'DELETE', `${c}/rule-hooks/${prepIds.hooks.deck}`)).status).toBe(403)
  })
})

describe('downtime and vehicles', () => {
  it('marks only a Player’s own Characters as theirs to spend', async () => {
    const player = (await ask<{ dm: boolean; characters: { id: string; mine: boolean }[] }>(as('player'), 'GET', `${c}/downtime`)).body
    expect(player.dm).toBe(false)
    expect(player.characters.filter((ch) => ch.mine).map((ch) => ch.id)).toEqual([ids.characters[0]])
    const dm = (await ask<{ dm: boolean; characters: { mine: boolean }[] }>(as('dm'), 'GET', `${c}/downtime`)).body
    expect([dm.dm, dm.characters.every((ch) => ch.mine)]).toEqual([true, true])
  })

  it('counts a Character the downtime ledger has not met as having no days', async () => {
    const dm = as('dm')
    dm.world.prep.downtime.days = {}
    const body = (await ask<{ characters: { days: number }[] }>(dm, 'GET', `${c}/downtime`)).body
    expect(body.characters.map((ch) => ch.days)).toEqual([0, 0, 0, 0])
  })

  it('shows every Member the vehicles, and tells the DM they are theirs to change', async () => {
    expect((await ask<{ dm: boolean; vehicles: unknown[] }>(as('player'), 'GET', `${c}/vehicles`)).body).toMatchObject({ dm: false, vehicles: [expect.anything(), expect.anything()] })
    expect((await ask<{ dm: boolean }>(as('dm'), 'GET', `${c}/vehicles`)).body.dm).toBe(true)
  })
})
