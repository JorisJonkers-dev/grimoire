import type { MentionRef, NotificationKind } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import type { Core } from '../../world'
import { personalId } from './ids'

// Who is Friends with whom, who asked whom, what they wrote to each other and what rang their bell.
// Ash (the Player) has Friends both in and out of the Campaign; Morgan (the DM) and the Admin have a
// few of their own, so each persona's Friends page and bell differ.

export type Friendship = { a: string; b: string; since: string }
export type FriendRequest = { id: string; from: string; to: string; at: string }
export type Block = { id: string; by: string; whom: string; at: string }
export type DemoConversation = {
  id: string
  title: string
  memberIds: string[]
  updatedAt: string
  /** Messages each member has not read yet. */
  unread: Record<string, number>
}
export type DemoMessage = { id: string; conversationId: string; authorId: string; body: string; at: string; mentions: MentionRef[] }
export type DemoNotification = {
  id: string
  accountId: string
  kind: NotificationKind
  title: string
  body: string
  actionLabel: string
  actionPath: string
  at: string
  read: boolean
}

export function seedSocial(core: Core) {
  const [ash, brook, cedar, dune] = ids.players
  const { dm, outsider: fennel, admin } = ids
  const c = core.campaign
  const friendships: Friendship[] = [
    { a: ash, b: brook, since: '2026-08-14T19:00:00Z' },
    { a: ash, b: fennel, since: '2026-06-02T12:30:00Z' },
    { a: brook, b: fennel, since: '2026-07-21T09:15:00Z' },
    { a: dm, b: cedar, since: '2026-05-30T20:45:00Z' },
    { a: dm, b: admin, since: '2026-04-11T08:00:00Z' },
  ]
  const friendRequests: FriendRequest[] = [
    { id: personalId(0x101), from: cedar, to: ash, at: '2026-10-28T17:20:00Z' },
    { id: personalId(0x102), from: ash, to: dune, at: '2026-10-25T11:05:00Z' },
    { id: personalId(0x103), from: dune, to: dm, at: '2026-10-30T21:40:00Z' },
  ]
  const blocks: Block[] = []
  const party = ids.conversation
  const direct = personalId(0x201)
  const dmCedar = personalId(0x202)
  const conversations: DemoConversation[] = [
    { id: party, title: 'Before the bell tolls', memberIds: [ash, brook, fennel], updatedAt: '2026-11-01T20:12:00Z', unread: { [ash]: 1, [brook]: 0, [fennel]: 2 } },
    { id: direct, title: '', memberIds: [ash, fennel], updatedAt: '2026-10-29T08:40:00Z', unread: { [ash]: 0, [fennel]: 0 } },
    { id: dmCedar, title: '', memberIds: [dm, cedar], updatedAt: '2026-10-31T15:00:00Z', unread: { [dm]: 0, [cedar]: 1 } },
  ]
  const tamsin: MentionRef = { kind: 'character', campaignId: c.id, id: ids.characters[0] }
  const messages: DemoMessage[] = [
    { id: personalId(0x301), conversationId: party, authorId: brook, body: 'The tide tables say the drowned chapel is dry for an hour after midnight.', at: '2026-11-01T19:58:00Z', mentions: [] },
    { id: personalId(0x302), conversationId: party, authorId: ash, body: '@Tamsin Reed can pick the vestry lock while the water is out.', at: '2026-11-01T20:05:00Z', mentions: [tamsin] },
    { id: personalId(0x303), conversationId: party, authorId: fennel, body: 'Bring rope. Last time the stairs were gone.', at: '2026-11-01T20:12:00Z', mentions: [] },
    { id: personalId(0x304), conversationId: direct, authorId: fennel, body: 'Still on for dice shopping on Saturday?', at: '2026-10-29T08:31:00Z', mentions: [] },
    { id: personalId(0x305), conversationId: direct, authorId: ash, body: 'Yes, the brass ones are calling me.', at: '2026-10-29T08:40:00Z', mentions: [] },
    { id: personalId(0x306), conversationId: dmCedar, authorId: dm, body: 'Your wizard hears the bell again tonight. Think about what Liri wants from it.', at: '2026-10-31T15:00:00Z', mentions: [] },
  ]
  const cp = `/campaigns/${c.id}`
  const notifications: DemoNotification[] = [
    { id: personalId(0x401), accountId: ash, kind: 'friend_request', title: 'Cedar wants to be Friends', body: '', actionLabel: 'Open', actionPath: '/friends', at: '2026-10-28T17:20:00Z', read: false },
    { id: personalId(0x402), accountId: ash, kind: 'conversation', title: 'Fennel in Before the bell tolls', body: 'Bring rope. Last time the stairs were gone.', actionLabel: 'Open', actionPath: `/conversations/${party}`, at: '2026-11-01T20:12:00Z', read: false },
    { id: personalId(0x403), accountId: ash, kind: 'level_up', title: 'Tamsin Reed can level up', body: `In ${c.name}`, actionLabel: 'Level up', actionPath: `${cp}/characters/${ids.characters[0]}/level-up`, at: '2026-10-27T22:00:00Z', read: true },
    { id: personalId(0x404), accountId: dm, kind: 'join_request', title: 'Wren Ashdown asks to join', body: `A Join Request for ${c.name}`, actionLabel: 'Review', actionPath: cp, at: '2026-10-01T18:00:00Z', read: false },
    { id: personalId(0x405), accountId: dm, kind: 'proposal', title: 'A Proposal waits for you', body: `In ${c.name}`, actionLabel: 'Review', actionPath: `${cp}/proposals/${ids.proposal}`, at: '2026-10-30T10:00:00Z', read: false },
    { id: personalId(0x406), accountId: dm, kind: 'friend_request', title: 'Dune wants to be Friends', body: '', actionLabel: 'Open', actionPath: '/friends', at: '2026-10-30T21:40:00Z', read: true },
    { id: personalId(0x407), accountId: admin, kind: 'security', title: 'Two-step sign-in turned on', body: 'Your Account now asks for a code from your authenticator app.', actionLabel: 'Open', actionPath: '/account', at: '2026-04-11T08:05:00Z', read: true },
  ]
  return { friendships, friendRequests, blocks, conversations, messages, notifications }
}
