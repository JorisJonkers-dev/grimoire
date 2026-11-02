import { ids } from './ids'
import type { Persona } from './role'
import { seedCharacters } from './seeds/characters'
import { seedCompendium } from './seeds/compendium'
import { seedPersonal } from './seeds/personal'
import { seedPrep } from './seeds/prep'
import type { Account, Campaign, CampaignSummary, Invite, Member } from '@/infrastructure/api/types.gen'

// The one seeded world the demo starts from: a Campaign with its DM, four Players and their
// Characters, a pending Campaign Invite and a pending Join Request. It lives in memory; a reload
// starts it afresh.

const at = '2026-10-01T18:00:00Z'

function account(id: string, username: string, nickname: string, extra: Partial<Account> = {}): Account {
  return { id, username, nickname, email: `${username}@demo.invalid`, admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false, ...extra }
}

/** A Character a Player built and filed to join the Campaign; the DM has not answered yet. */
export type JoinRequest = { id: string; characterId: string; accountId: string; characterName: string; filedAt: string }

export type DemoCharacter = {
  id: string
  ownerId: string
  name: string
  species: string
  class: string
  level: number
  hpCurrent: number
  hpMax: number
}

/** The people, the Campaign and its Members: what every area of the world is built around. */
export function seedCore(persona: Persona = 'player') {
  const dm = account(ids.dm, 'morgan', 'Morgan')
  const [p1, p2, p3, p4] = ids.players
  const playerAccounts = [
    account(p1, 'ash', 'Ash'),
    account(p2, 'brook', 'Brook'),
    account(p3, 'cedar', 'Cedar'),
    account(p4, 'dune', 'Dune'),
  ]
  // An Admin holds their powers only with two-step sign-in on, as in production.
  const admin = account(ids.admin, 'admin', 'Admin', { admin: true, adminPowers: true, twoStep: true, recoveryCodesLeft: 8 })
  const outsider = account(ids.outsider, 'fennel', 'Fennel')
  const people = [dm, ...playerAccounts]
  const members = ids.members.map((id, i) => {
    const who = people[i] as Account
    return { id, accountId: who.id, displayName: who.nickname, role: who === dm ? 'dm' : 'player', joinedAt: at } satisfies Omit<Member, 'isMe'> & { accountId: string }
  })
  const characters: DemoCharacter[] = [
    { id: ids.characters[0], ownerId: p1, name: 'Tamsin Reed', species: 'halfling', class: 'rogue', level: 3, hpCurrent: 21, hpMax: 24 },
    { id: ids.characters[1], ownerId: p2, name: 'Oswin Hale', species: 'human', class: 'fighter', level: 3, hpCurrent: 31, hpMax: 31 },
    { id: ids.characters[2], ownerId: p3, name: 'Liri Vael', species: 'elf', class: 'wizard', level: 3, hpCurrent: 9, hpMax: 17 },
    { id: ids.characters[3], ownerId: p4, name: 'Brannoc Stone', species: 'dwarf', class: 'cleric', level: 3, hpCurrent: 27, hpMax: 27 },
  ]
  const invites: Invite[] = [{ id: ids.invite, createdAt: at, expiresAt: '2026-12-31T23:59:59Z', createdBy: dm.nickname }]
  const joinRequests: JoinRequest[] = [{ id: ids.joinRequest, characterId: ids.draft, accountId: p1, characterName: 'Wren Ashdown', filedAt: at }]
  return {
    /** Who is looking: the role switcher sets it, and signing in or out changes it as it would. */
    persona,
    accounts: { dm, admin, player: playerAccounts[0] as Account, players: playerAccounts, outsider, all: [...people, outsider, admin] },
    campaign: { id: ids.campaign, name: 'The Sunken Bell', ruleset: 'srd-2024' as const, createdAt: at, startingLevel: 3 },
    members,
    characters,
    invites,
    joinRequests,
  }
}

export type Core = ReturnType<typeof seedCore>

/** The whole seeded world: the core, and each area's records built around it. */
export function seedWorld(persona: Persona = 'player') {
  const core = seedCore(persona)
  return { ...core, personal: seedPersonal(core), compendium: seedCompendium(core), characterData: seedCharacters(core), prep: seedPrep(core) }
}

export type World = ReturnType<typeof seedWorld>

/** The Campaign as one of its Members sees it: their own role and Membership marked. */
export function campaignFor(world: World, accountId: string): Campaign {
  const mine = world.members.find((m) => m.accountId === accountId) as World['members'][number]
  const members: Member[] = world.members.map((m) => ({ id: m.id, displayName: m.displayName, role: m.role, joinedAt: m.joinedAt, isMe: m.accountId === accountId }))
  const me = members.find((m) => m.isMe) as Member
  return { ...summaryFor(world, mine.role), me, members }
}

export function summaryFor(world: World, role: Member['role']): CampaignSummary {
  return { ...world.campaign, myRole: role, memberCount: world.members.length }
}
