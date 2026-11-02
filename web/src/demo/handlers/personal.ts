import type { Account, DashboardNeed, DiceSet, MentionView, Person, SearchHit } from '@/infrastructure/api/types.gen'
import { slugs } from '../ids'
import { DemoProblem, notFound, type Handlers } from '../operations'
import type { Viewer } from '../role'
import type { DemoDiceSet } from '../seeds/personal/dice'
import { freshId, now } from '../seeds/personal/ids'
import type { DemoMessage } from '../seeds/personal/social'
import type { World } from '../world'
import { admin, signedIn } from './access'

// What belongs to the signed-in Account wherever it is: its bell, Dashboard and search, its Friends,
// Conversations and Dice Sets. Each answer holds only what that Account may see.

const conflict = (title: string) => new DemoProblem(409, title)
const invalid = (title: string) => new DemoProblem(422, title)

function person(world: World, accountId: string): Person {
  const a = world.accounts.all.find((x) => x.id === accountId) as Account
  return { id: a.id, username: a.username, nickname: a.nickname }
}

const friendsOf = (world: World, me: string) =>
  world.personal.friendships.flatMap((f) => (f.a === me ? [{ id: f.b, since: f.since }] : f.b === me ? [{ id: f.a, since: f.since }] : []))
const areFriends = (world: World, a: string, b: string) => friendsOf(world, a).some((f) => f.id === b)

function befriend(world: World, a: string, b: string): void {
  world.personal.friendships.push({ a, b, since: now() })
}

function removeRequest(world: World, find: (r: World['personal']['friendRequests'][number]) => boolean): World['personal']['friendRequests'][number] {
  const at = world.personal.friendRequests.findIndex(find)
  if (at < 0) throw notFound('Friend request')
  return world.personal.friendRequests.splice(at, 1)[0] as World['personal']['friendRequests'][number]
}

// A Conversation is found only by its members; anyone else is told there is none.
function conversationOf(world: World, me: string, conversationId: string) {
  const c = world.personal.conversations.find((x) => x.id === conversationId && x.memberIds.includes(me))
  if (!c) throw notFound('Conversation')
  return c
}

// A Mention shows its name only to a reader who may open what it points at.
function mentionView(world: World, who: Viewer, m: DemoMessage['mentions'][number]): MentionView {
  const character = world.characters.find((c) => c.id === m.id)
  const open = who.role !== null && m.campaignId === world.campaign.id && character !== undefined
  return { kind: m.kind, campaignId: m.campaignId, id: m.id, open, ...(open ? { label: character.name } : {}) }
}

function messageOut(world: World, who: Viewer, m: DemoMessage) {
  return { id: m.id, author: person(world, m.authorId), body: m.body, at: m.at, mentions: m.mentions.map((x) => mentionView(world, who, x)) }
}

function diceOut(me: string, s: DemoDiceSet): DiceSet {
  return { id: s.id, name: s.name, design: s.design, hasImage: false, sharing: s.sharing, review: 'none', mine: s.ownerId === me, copy: s.copyOf !== undefined, by: s.by, updatedAt: s.updatedAt }
}

const mySet = (world: World, me: string, diceSetId: string) => {
  const s = world.personal.diceSets.find((x) => x.id === diceSetId && x.ownerId === me)
  if (!s) throw notFound('Dice Set')
  return s
}

// Sets of others the caller may copy: a Friend's shared with Friends or everyone, anyone's shared
// with everyone. None carries a picture, so none waits on an Admin.
const sharedWith = (world: World, me: string) =>
  world.personal.diceSets.filter((s) => s.ownerId !== me && !s.copyOf && (s.sharing === 'everyone' || (s.sharing === 'friends' && areFriends(world, me, s.ownerId))))

const newestFirst = <T extends { at: string }>(list: T[]) => [...list].sort((a, b) => b.at.localeCompare(a.at))

const counted = (n: number, one: string, many: string) => `${String(n)} ${n === 1 ? one : many}`

/** Compendium records search can find; they are the SRD 5.2 records the demo opens. */
const compendium: SearchHit[] = [
  { group: 'compendium', kind: 'spell', title: 'Fireball', preview: 'Level 3 evocation spell', path: `/compendium/spells/${slugs.spell}` },
  { group: 'compendium', kind: 'monster', title: 'Goblin Warrior', preview: 'Small fey · CR 1/4', path: `/compendium/${slugs.entryKind}/${slugs.entry}` },
]

export const personalHandlers = {
  listNotifications: ({ who, world }) => {
    const me = signedIn(who).id
    const mine = newestFirst(world.personal.notifications.filter((n) => n.accountId === me))
    return { items: mine.slice(0, 50).map((n) => ({ id: n.id, kind: n.kind, title: n.title, body: n.body, actionLabel: n.actionLabel, actionPath: n.actionPath, at: n.at, read: n.read })), unread: mine.filter((n) => !n.read).length }
  },
  readNotification: ({ who, world, path }) => {
    const me = signedIn(who).id
    const n = world.personal.notifications.find((x) => x.id === path.notificationId && x.accountId === me)
    if (!n) throw notFound('Notification')
    n.read = true
  },
  readAllNotifications: ({ who, world }) => {
    const me = signedIn(who).id
    for (const n of world.personal.notifications) if (n.accountId === me) n.read = true
  },
  getUnseenReleaseNote: ({ who, world }) => {
    const seen = world.personal.seenNotes[signedIn(who).id] ?? []
    const note = world.personal.releaseNotes.find((n) => n.status === 'published' && !seen.includes(n.id))
    return note ? { note } : {}
  },
  seeReleaseNote: ({ who, world, path }) => {
    const me = signedIn(who).id
    if (!world.personal.releaseNotes.some((n) => n.id === path.noteId && n.status === 'published')) throw notFound('Release Note')
    world.personal.seenNotes[me] = [...(world.personal.seenNotes[me] ?? []), path.noteId]
  },

  getDashboard: ({ who, world }) => {
    const me = signedIn(who).id
    const c = world.campaign
    const needs: DashboardNeed[] = []
    if (who.role === 'player') {
      for (const ch of world.characters.filter((x) => x.ownerId === me)) {
        needs.push({ kind: 'level_up', campaign: c.name, title: `${ch.name} can level up`, path: `/campaigns/${c.id}/characters/${ch.id}/level-up` })
      }
    }
    if (who.role === 'dm') needs.push({ kind: 'proposals', campaign: c.name, title: '1 Proposal to review', path: `/campaigns/${c.id}/proposals` })
    const asked = world.personal.friendRequests.filter((r) => r.to === me && !world.personal.blocks.some((b) => b.by === me && b.whom === r.from)).length
    if (asked > 0) needs.push({ kind: 'friend_requests', title: `${counted(asked, 'Friend Request', 'Friend Requests')} to answer`, path: '/friends' })
    return { live: [], needs }
  },

  // Only what the caller may open: the compendium, their own Campaign and its Characters, and their
  // Friends. Nothing of a Campaign they are not in, and nobody who is not a Friend.
  search: ({ who, world, query }) => {
    const me = signedIn(who).id
    const q = query.q.trim()
    if (q.length < 2 || q.length > 80) throw invalid('Search for 2 to 80 characters')
    const has = (...names: string[]) => names.some((n) => n.toLowerCase().includes(q.toLowerCase()))
    const c = world.campaign
    const campaigns: SearchHit[] = who.role
      ? [
          ...(has(c.name) ? [{ group: 'campaigns' as const, kind: 'campaign', title: c.name, preview: who.role === 'dm' ? 'You are the DM of this Campaign' : 'You play in this Campaign', path: `/campaigns/${c.id}` }] : []),
          ...world.characters.filter((ch) => has(ch.name)).slice(0, 5).map((ch) => ({ group: 'campaigns' as const, kind: 'character', title: ch.name, preview: `Level ${String(ch.level)} ${ch.class} in ${c.name}`, path: `/campaigns/${c.id}/characters/${ch.id}` })),
        ]
      : []
    const people = friendsOf(world, me)
      .map((f) => person(world, f.id))
      .filter((p) => has(p.nickname, p.username))
      .slice(0, 5)
      .map((p): SearchHit => ({ group: 'people', kind: 'friend', title: p.nickname, preview: `Friend · @${p.username}`, path: '/friends' }))
    return { hits: [...compendium.filter((h) => has(h.title)), ...campaigns, ...people] }
  },

  listFriends: ({ who, world }) => {
    const me = signedIn(who).id
    const blocked = world.personal.blocks.filter((b) => b.by === me)
    const entry = (id: string, accountId: string, at: string) => ({ id, person: person(world, accountId), at })
    return {
      friends: friendsOf(world, me).map((f) => ({ person: person(world, f.id), since: f.since })),
      incoming: world.personal.friendRequests.filter((r) => r.to === me && !blocked.some((b) => b.whom === r.from)).map((r) => entry(r.id, r.from, r.at)),
      outgoing: world.personal.friendRequests.filter((r) => r.from === me).map((r) => entry(r.id, r.to, r.at)),
      blocked: blocked.map((b) => entry(b.id, b.whom, b.at)),
    }
  },
  // A request they already sent is accepted at once; one to somebody who blocked the sender is
  // answered the same and never shows.
  sendFriendRequest: ({ who, world, body }) => {
    const me = signedIn(who).id
    const them = world.accounts.all.find((a) => a.username === body.username.trim().toLowerCase())
    if (!them) throw notFound('Account')
    if (them.id === me) throw invalid('That is you')
    if (areFriends(world, me, them.id)) throw conflict('Already Friends')
    const theirs = world.personal.friendRequests.find((r) => r.from === them.id && r.to === me)
    if (theirs) {
      removeRequest(world, (r) => r === theirs)
      befriend(world, me, them.id)
      return
    }
    if (world.personal.friendRequests.some((r) => r.from === me && r.to === them.id)) return
    world.personal.friendRequests.push({ id: freshId(), from: me, to: them.id, at: now() })
  },
  acceptFriendRequest: ({ who, world, path }) => {
    const me = signedIn(who).id
    const r = removeRequest(world, (x) => x.id === path.requestId && x.to === me)
    befriend(world, me, r.from)
  },
  declineFriendRequest: ({ who, world, path, body }) => {
    const me = signedIn(who).id
    const r = removeRequest(world, (x) => x.id === path.requestId && x.to === me)
    if (body.block) world.personal.blocks.push({ id: freshId(), by: me, whom: r.from, at: now() })
  },
  cancelFriendRequest: ({ who, world, path }) => {
    const me = signedIn(who).id
    removeRequest(world, (x) => x.id === path.requestId && x.from === me)
  },
  unfriend: ({ who, world, path }) => {
    const me = signedIn(who).id
    const at = world.personal.friendships.findIndex((f) => (f.a === me && f.b === path.accountId) || (f.b === me && f.a === path.accountId))
    if (at < 0) throw notFound('Friend')
    world.personal.friendships.splice(at, 1)
  },
  unblock: ({ who, world, path }) => {
    const me = signedIn(who).id
    const at = world.personal.blocks.findIndex((b) => b.by === me && b.whom === path.accountId)
    if (at < 0) throw notFound('Block')
    world.personal.blocks.splice(at, 1)
  },

  listConversations: ({ who, world }) => {
    const me = signedIn(who).id
    const mine = world.personal.conversations.filter((c) => c.memberIds.includes(me)).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
    const last = (id: string) => newestFirst(world.personal.messages.filter((m) => m.conversationId === id))[0]?.body.slice(0, 120) ?? ''
    return {
      items: mine.map((c) => ({ id: c.id, title: c.title, members: c.memberIds.map((id) => person(world, id)), updatedAt: c.updatedAt, unread: c.unread[me] ?? 0, lastBody: last(c.id) })),
    }
  },
  // With Friends only; a one-to-one Conversation that exists already is found again.
  startConversation: ({ who, world, body }) => {
    const me = signedIn(who).id
    if (!body.with.every((id) => areFriends(world, me, id))) throw notFound('Friend')
    const members = [me, ...body.with.filter((id) => id !== me)]
    const direct = members.length === 2 && world.personal.conversations.find((c) => c.memberIds.length === 2 && members.every((id) => c.memberIds.includes(id)))
    if (direct) return { id: direct.id }
    const id = freshId()
    world.personal.conversations.push({ id, title: body.title ?? '', memberIds: members, updatedAt: now(), unread: {} })
    return { id }
  },
  listMessages: ({ who, world, path }) => {
    const me = signedIn(who).id
    const c = conversationOf(world, me, path.conversationId)
    c.unread[me] = 0
    const items = newestFirst(world.personal.messages.filter((m) => m.conversationId === c.id)).slice(0, 50)
    return { items: items.map((m) => messageOut(world, who, m)) }
  },
  // Each Mention must be something the sender may open themselves.
  sendMessage: ({ who, world, path, body }) => {
    const me = signedIn(who).id
    const c = conversationOf(world, me, path.conversationId)
    const text = body.body.trim()
    const mentions = body.mentions ?? []
    if (!text || !mentions.every((m) => mentionView(world, who, m).open)) throw invalid('That message cannot be sent')
    const m: DemoMessage = { id: freshId(), conversationId: c.id, authorId: me, body: text, at: now(), mentions }
    world.personal.messages.push(m)
    c.updatedAt = m.at
    for (const id of c.memberIds) if (id !== me) c.unread[id] = (c.unread[id] ?? 0) + 1
    return messageOut(world, who, m)
  },
  // Characters in the caller's Campaign. Locations belong to the maps of a Campaign the caller runs.
  listMentionables: ({ who, world, query }) => {
    signedIn(who)
    // Without letters the query string is left off altogether.
    const q = (query?.q ?? '').toLowerCase()
    const c = world.campaign
    const characters = who.role ? world.characters.filter((ch) => ch.name.toLowerCase().includes(q)) : []
    return { items: characters.map((ch) => ({ kind: 'character' as const, id: ch.id, name: ch.name, campaignId: c.id, campaignName: c.name })) }
  },

  listDiceSets: ({ who, world }) => {
    const me = signedIn(who).id
    const chosen = world.personal.chosenDice[me]
    return { items: world.personal.diceSets.filter((s) => s.ownerId === me).map((s) => diceOut(me, s)), ...(chosen ? { chosen } : {}) }
  },
  listSharedDiceSets: ({ who, world }) => {
    const me = signedIn(who).id
    return { items: sharedWith(world, me).map((s) => diceOut(me, s)) }
  },
  listDiceSetsToReview: ({ who }) => {
    admin(who)
    return { items: [] }
  },
  createDiceSet: ({ who, world, body }) => {
    const me = signedIn(who)
    const s: DemoDiceSet = { id: freshId(), ownerId: me.id, name: body.name, design: body.design, sharing: 'private', by: me.nickname, updatedAt: now() }
    world.personal.diceSets.push(s)
    return diceOut(me.id, s)
  },
  editDiceSet: ({ who, world, path, body }) => {
    const me = signedIn(who).id
    const s = mySet(world, me, path.diceSetId)
    if (s.copyOf) throw conflict('A copy cannot be changed')
    Object.assign(s, { name: body.name, design: body.design, updatedAt: now() })
    return diceOut(me, s)
  },
  chooseDiceSet: ({ who, world, body }) => {
    const me = signedIn(who).id
    if (body.diceSetId) world.personal.chosenDice[me] = mySet(world, me, body.diceSetId).id
    else Reflect.deleteProperty(world.personal.chosenDice, me)
  },
  shareDiceSet: ({ who, world, path, body }) => {
    const me = signedIn(who).id
    const s = mySet(world, me, path.diceSetId)
    if (s.copyOf) throw conflict('A copy cannot be shared on')
    s.sharing = body.sharing
    return diceOut(me, s)
  },
  copyDiceSet: ({ who, world, path }) => {
    const me = signedIn(who).id
    const original = sharedWith(world, me).find((s) => s.id === path.diceSetId)
    if (!original) throw notFound('Dice Set')
    if (world.personal.diceSets.some((s) => s.ownerId === me && s.copyOf === original.id)) throw conflict('A set is copied once')
    const copy: DemoDiceSet = { ...original, id: freshId(), ownerId: me, sharing: 'private', copyOf: original.id, updatedAt: now() }
    world.personal.diceSets.push(copy)
    return diceOut(me, copy)
  },
  deleteDiceSet: ({ who, world, path }) => {
    const me = signedIn(who).id
    const s = mySet(world, me, path.diceSetId)
    world.personal.diceSets.splice(world.personal.diceSets.indexOf(s), 1)
    if (world.personal.chosenDice[me] === s.id) Reflect.deleteProperty(world.personal.chosenDice, me)
  },
} satisfies Handlers

