import { afterEach, describe, expect, it, vi } from 'vitest'
import * as sdk from '@/infrastructure/api/sdk.gen'
import { configureApi } from '@/infrastructure/http'
import { mountDemo } from '@/test/mountDemo'
import { createDemo } from '../demo'
import { ids } from '../ids'
import type { Persona, Viewer } from '../role'
import { personalId } from '../seeds/personal/ids'
import { personalHandlers } from './personal'

// Every call goes through the generated SDK, so each answer is also checked against the contract's
// schemas, exactly as the app checks it.

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function as(persona: Persona) {
  const demo = createDemo(persona)
  configureApi({ baseUrl: 'https://demo.invalid', fetch: demo.fetch })
  return demo
}

const status = (r: { response?: Response }) => r.response?.status
const [ash, brook, cedar, dune] = ids.players
const c = ids.campaign
const party = ids.conversation
const direct = personalId(0x201)
const notification = (n: number) => personalId(0x400 + n)
const dice = (n: number) => personalId(0x500 + n)
const published = personalId(0x701)

describe('signed out', () => {
  it('is refused everything that needs an Account, and still told the service status and sign-in methods', async () => {
    as('signed-out')
    const refused = await Promise.all([
      sdk.listNotifications(), sdk.getMe(), sdk.getDashboard(), sdk.search({ query: { q: 'fire' } }), sdk.listFriends(), sdk.listConversations(),
      sdk.listDiceSets(), sdk.getAccountHistory(), sdk.listAccessTokens(), sdk.getNotificationPreferences(), sdk.getUnseenReleaseNote(), sdk.listAdminAccounts(),
    ])
    expect(refused.map(status)).toEqual(refused.map(() => 401))
    expect((await sdk.getStatus()).data).toMatchObject({ service: 'grimoire', database: 'up' })
    expect((await sdk.getSignInMethods()).data).toEqual({})
  })
})

describe('the bell', () => {
  it('rings with the caller’s own Notifications only, newest first', async () => {
    as('player')
    const mine = (await sdk.listNotifications()).data
    expect(mine?.items.map((n) => n.id)).toEqual([notification(2), notification(1), notification(3)])
    expect(mine?.unread).toBe(2)
    as('dm')
    expect((await sdk.listNotifications()).data?.items.map((n) => n.kind)).toEqual(['friend_request', 'proposal', 'join_request'])
  })

  it('marks one read, refuses another’s, and clears them all', async () => {
    as('player')
    expect(status(await sdk.readNotification({ path: { notificationId: notification(1) } }))).toBe(204)
    expect((await sdk.listNotifications()).data?.unread).toBe(1)
    expect(status(await sdk.readNotification({ path: { notificationId: notification(4) } }))).toBe(404)
    await sdk.readAllNotifications()
    expect((await sdk.listNotifications()).data?.unread).toBe(0)
  })

  it('says who is signed in', async () => {
    as('dm')
    expect((await sdk.getMe()).data).toEqual({ subject: `account:${ids.dm}` })
  })
})

describe('Release Notes', () => {
  it('shows the newest one until it is seen', async () => {
    as('player')
    expect((await sdk.getUnseenReleaseNote()).data?.note?.version).toBe('1.5.0')
    expect(status(await sdk.seeReleaseNote({ path: { noteId: personalId(0x702) } }))).toBe(404)
    await sdk.seeReleaseNote({ path: { noteId: published } })
    expect((await sdk.getUnseenReleaseNote()).data).toEqual({})
    as('dm')
    expect((await sdk.getUnseenReleaseNote()).data).toEqual({})
  })

  it('lists drafts too, to an Admin only', async () => {
    as('admin')
    expect((await sdk.listReleaseNotes()).data?.items.map((n) => n.status)).toEqual(['draft', 'published'])
    as('dm')
    expect(status(await sdk.listReleaseNotes())).toBe(403)
  })
})

describe('the Dashboard', () => {
  it('shows a Player their Character to level up and the Friend requests waiting', async () => {
    as('player')
    expect((await sdk.getDashboard()).data).toEqual({
      live: [],
      needs: [
        { kind: 'level_up', campaign: 'The Sunken Bell', title: 'Tamsin Reed can level up', path: `/campaigns/${c}/characters/${ids.characters[0]}/level-up` },
        { kind: 'friend_requests', title: '1 Friend Request to answer', path: '/friends' },
      ],
    })
  })

  it('shows the DM the Proposals to review, and an Admin outside the Campaign nothing', async () => {
    as('dm')
    expect((await sdk.getDashboard()).data?.needs.map((n) => n.kind)).toEqual(['proposals', 'friend_requests'])
    as('admin')
    expect((await sdk.getDashboard()).data?.needs).toEqual([])
  })

  it('counts the requests waiting, none from somebody the caller blocked', async () => {
    const demo = as('player')
    demo.world.personal.blocks.push({ id: personalId(0x901), by: ash, whom: cedar, at: '2026-11-01T00:00:00Z' })
    expect((await sdk.getDashboard()).data?.needs.map((n) => n.kind)).toEqual(['level_up'])
    demo.world.personal.blocks.length = 0
    demo.world.personal.friendRequests.push({ id: personalId(0x902), from: ids.dm, to: ash, at: '2026-11-01T00:00:00Z' })
    expect((await sdk.getDashboard()).data?.needs[1]?.title).toBe('2 Friend Requests to answer')
  })
})

describe('search', () => {
  const titles = async (q: string) => (await sdk.search({ query: { q } })).data?.hits.map((h) => `${h.group}:${h.title}`)

  it('finds compendium records for anyone signed in', async () => {
    as('admin')
    expect(await titles('fire')).toEqual(['compendium:Fireball'])
    expect(await titles('GOBLIN')).toEqual(['compendium:Goblin Warrior'])
  })

  it('finds the Campaign and its Characters only for its Members', async () => {
    as('player')
    const hits = (await sdk.search({ query: { q: 'sunken' } })).data?.hits
    expect(hits?.map((h) => h.preview)).toEqual(['You play in this Campaign'])
    expect(await titles('tamsin')).toEqual(['campaigns:Tamsin Reed'])
    as('dm')
    expect((await sdk.search({ query: { q: 'sunken' } })).data?.hits.map((h) => h.preview)).toEqual(['You are the DM of this Campaign'])
    as('admin')
    expect(await titles('tamsin')).toEqual([])
    expect(await titles('sunken')).toEqual([])
  })

  it('finds Friends, by Nickname or Username, and nobody else', async () => {
    as('player')
    expect(await titles('fenn')).toEqual(['people:Fennel'])
    as('dm')
    expect(await titles('fenn')).toEqual([])
    expect(await titles('cedar')).toEqual(['people:Cedar'])
  })

  it('refuses a query too short once trimmed', async () => {
    as('player')
    expect(status(await sdk.search({ query: { q: ' a ' } }))).toBe(422)
  })
})

describe('Friends', () => {
  const page = async () => {
    const d = (await sdk.listFriends()).data
    const names = (list: { person: { nickname: string } }[] | undefined) => list?.map((x) => x.person.nickname)
    return { friends: names(d?.friends), incoming: names(d?.incoming), outgoing: names(d?.outgoing), blocked: names(d?.blocked) }
  }

  it('shows each caller their own Friends and requests', async () => {
    as('player')
    expect(await page()).toEqual({ friends: ['Brook', 'Fennel'], incoming: ['Cedar'], outgoing: ['Dune'], blocked: [] })
    as('dm')
    expect(await page()).toEqual({ friends: ['Cedar', 'Admin'], incoming: ['Dune'], outgoing: [], blocked: [] })
  })

  it('refuses a request to nobody, to oneself or to a Friend', async () => {
    as('player')
    expect(status(await sdk.sendFriendRequest({ body: { username: 'nobody' } }))).toBe(404)
    expect(status(await sdk.sendFriendRequest({ body: { username: 'ash' } }))).toBe(422)
    expect(status(await sdk.sendFriendRequest({ body: { username: 'Brook' } }))).toBe(409)
  })

  it('accepts at once a request they already sent, and sends a new one once', async () => {
    as('player')
    await sdk.sendFriendRequest({ body: { username: 'cedar' } })
    await sdk.sendFriendRequest({ body: { username: 'dune' } })
    await sdk.sendFriendRequest({ body: { username: 'morgan' } })
    expect(await page()).toEqual({ friends: ['Brook', 'Fennel', 'Cedar'], incoming: [], outgoing: ['Dune', 'Morgan'], blocked: [] })
  })

  it('accepts, declines, blocks, unblocks, withdraws and unfriends', async () => {
    const demo = as('dm')
    const asked = demo.world.personal.friendRequests.find((r) => r.to === ids.dm)?.id ?? ''
    expect(status(await sdk.acceptFriendRequest({ path: { requestId: personalId(0x102) } }))).toBe(404)
    await sdk.acceptFriendRequest({ path: { requestId: asked } })
    expect((await page()).friends).toEqual(['Cedar', 'Admin', 'Dune'])
    const player = as('player')
    await sdk.declineFriendRequest({ path: { requestId: personalId(0x101) }, body: { block: true } })
    expect(await page()).toMatchObject({ incoming: [], blocked: ['Cedar'] })
    // A blocked sender's later requests never show.
    player.world.personal.friendRequests.push({ id: personalId(0x903), from: cedar, to: ash, at: '2026-11-01T00:00:00Z' })
    expect((await page()).incoming).toEqual([])
    player.world.personal.friendRequests.pop()
    expect(status(await sdk.unblock({ path: { accountId: cedar } }))).toBe(204)
    expect(status(await sdk.unblock({ path: { accountId: cedar } }))).toBe(404)
    await sdk.cancelFriendRequest({ path: { requestId: personalId(0x102) } })
    expect(status(await sdk.cancelFriendRequest({ path: { requestId: personalId(0x102) } }))).toBe(404)
    await sdk.unfriend({ path: { accountId: brook } })
    expect(status(await sdk.unfriend({ path: { accountId: brook } }))).toBe(404)
    expect(await page()).toEqual({ friends: ['Fennel'], incoming: [], outgoing: [], blocked: [] })
  })

  it('ends a friendship from either side', async () => {
    as('admin')
    await sdk.unfriend({ path: { accountId: ids.dm } })
    expect((await page()).friends).toEqual([])
  })

  it('declines without blocking', async () => {
    as('player')
    await sdk.declineFriendRequest({ path: { requestId: personalId(0x101) }, body: {} })
    expect(await page()).toMatchObject({ incoming: [], blocked: [] })
  })
})

describe('Conversations', () => {
  it('lists only the caller’s own, newest first, with what they have not read', async () => {
    as('player')
    const mine = (await sdk.listConversations()).data?.items
    expect(mine?.map((x) => [x.id, x.unread, x.lastBody])).toEqual([
      [party, 1, 'Bring rope. Last time the stairs were gone.'],
      [direct, 0, 'Yes, the brass ones are calling me.'],
    ])
    as('dm')
    expect((await sdk.listConversations()).data?.items.map((x) => x.members.map((m) => m.nickname))).toEqual([['Morgan', 'Cedar']])
  })

  it('reads one to its members only, and marks it read', async () => {
    as('player')
    const messages = (await sdk.listMessages({ path: { conversationId: party } })).data?.items
    expect(messages?.map((m) => m.author.nickname)).toEqual(['Fennel', 'Ash', 'Brook'])
    expect(messages?.[1]?.mentions).toEqual([{ kind: 'character', campaignId: c, id: ids.characters[0], open: true, label: 'Tamsin Reed' }])
    expect((await sdk.listConversations()).data?.items[0]?.unread).toBe(0)
    as('dm')
    expect(status(await sdk.listMessages({ path: { conversationId: party } }))).toBe(404)
  })

  it('names a Mentioned Character only to a reader who may open it', () => {
    const world = createDemo('player').world
    const fennel: Viewer = { persona: 'player', account: world.accounts.outsider, role: null }
    const read = personalHandlers.listMessages({ who: fennel, world, path: { conversationId: party }, query: {}, body: undefined })
    expect(read.items[1]?.mentions).toEqual([{ kind: 'character', campaignId: c, id: ids.characters[0], open: false }])
  })

  it('finds a one-to-one again, starts a group with Friends only, and sends to it', async () => {
    as('player')
    expect((await sdk.startConversation({ body: { with: [ids.outsider] } })).data?.id).toBe(direct)
    expect(status(await sdk.startConversation({ body: { with: [dune] } }))).toBe(404)
    const group = (await sdk.startConversation({ body: { with: [brook, ids.outsider], title: 'Tide tables' } })).data?.id ?? ''
    const solo = (await sdk.startConversation({ body: { with: [brook] } })).data?.id ?? ''
    const fresh = (await sdk.listConversations()).data?.items.filter((x) => x.id === group || x.id === solo)
    expect(fresh?.map((x) => [x.title, x.lastBody, x.unread])).toEqual([['', '', 0], ['Tide tables', '', 0]])
    const sent = await sdk.sendMessage({ path: { conversationId: group }, body: { body: ' Low tide at midnight. ', mentions: [{ kind: 'character', campaignId: c, id: ids.characters[1] }] } })
    expect(sent.data).toMatchObject({ body: 'Low tide at midnight.', mentions: [{ open: true, label: 'Oswin Hale' }] })
    await sdk.sendMessage({ path: { conversationId: group }, body: { body: 'Bring lanterns.' } })
    expect((await sdk.listConversations()).data?.items[0]).toMatchObject({ id: group, lastBody: 'Bring lanterns.' })
  })

  it('refuses an empty message, a Mention the sender cannot open, and a stranger', async () => {
    as('player')
    const send = (body: string, id: string, campaignId = c) => sdk.sendMessage({ path: { conversationId: party }, body: { body, mentions: [{ kind: 'character', campaignId, id }] } })
    expect(status(await send('   ', ids.characters[0]))).toBe(422)
    expect(status(await send('Who?', ids.npc))).toBe(422)
    expect(status(await send('Who?', ids.characters[0], ids.session))).toBe(422)
    as('dm')
    expect(status(await sdk.sendMessage({ path: { conversationId: party }, body: { body: 'Hello' } }))).toBe(404)
  })

  it('offers the Characters of the caller’s Campaign to mention', async () => {
    as('player')
    expect((await sdk.listMentionables({ query: { q: 'ta' } })).data?.items.map((m) => m.name)).toEqual(['Tamsin Reed'])
    expect((await sdk.listMentionables()).data?.items).toHaveLength(4)
    as('admin')
    expect((await sdk.listMentionables({ query: { q: 'ta' } })).data?.items).toEqual([])
  })
})

describe('Dice Sets', () => {
  const names = (list: { name: string }[] | undefined) => list?.map((s) => s.name)
  const design = { dice: { d20: { pattern: 'plain' as const, body: '#000000', numbers: '#ffffff' } } }

  it('lists the caller’s own and the one they roll with', async () => {
    as('player')
    const mine = (await sdk.listDiceSets()).data
    expect(names(mine?.items)).toEqual(['Tidewater', 'Lantern Glass'])
    expect(mine?.chosen).toBe(dice(1))
    as('admin')
    expect((await sdk.listDiceSets()).data).toEqual({ items: [] })
  })

  it('shares a Friend’s sets with Friends and anyone’s shared with everyone', async () => {
    as('player')
    expect(names((await sdk.listSharedDiceSets()).data?.items)).toEqual(['Ember', 'Moss and Stone'])
    as('dm')
    expect(names((await sdk.listSharedDiceSets()).data?.items)).toEqual(['Moss and Stone'])
  })

  it('makes, changes, shares, copies once, chooses and deletes', async () => {
    as('player')
    const made = (await sdk.createDiceSet({ body: { name: 'Bell Bronze', design } })).data
    expect(made).toMatchObject({ mine: true, copy: false, sharing: 'private', by: 'Ash' })
    const id = made?.id ?? ''
    expect((await sdk.editDiceSet({ path: { diceSetId: id }, body: { name: 'Bell Bronze II', design } })).data?.name).toBe('Bell Bronze II')
    expect((await sdk.shareDiceSet({ path: { diceSetId: id }, body: { sharing: 'everyone' } })).data?.sharing).toBe('everyone')
    const copy = (await sdk.copyDiceSet({ path: { diceSetId: dice(3) } })).data
    expect(copy).toMatchObject({ name: 'Ember', copy: true, by: 'Brook', mine: true })
    expect(status(await sdk.copyDiceSet({ path: { diceSetId: dice(3) } }))).toBe(409)
    expect(status(await sdk.copyDiceSet({ path: { diceSetId: dice(5) } }))).toBe(404)
    expect(status(await sdk.shareDiceSet({ path: { diceSetId: copy?.id ?? '' }, body: { sharing: 'friends' } }))).toBe(409)
    expect(status(await sdk.editDiceSet({ path: { diceSetId: copy?.id ?? '' }, body: { name: 'Mine now', design } }))).toBe(409)
    await sdk.chooseDiceSet({ body: { diceSetId: id } })
    expect((await sdk.listDiceSets()).data?.chosen).toBe(id)
    await sdk.deleteDiceSet({ path: { diceSetId: id } })
    expect((await sdk.listDiceSets()).data?.chosen).toBeUndefined()
    await sdk.deleteDiceSet({ path: { diceSetId: dice(2) } })
    expect(status(await sdk.deleteDiceSet({ path: { diceSetId: dice(3) } }))).toBe(404)
    expect(names((await sdk.listDiceSets()).data?.items)).toEqual(['Tidewater', 'Ember'])
    as('dm')
    expect(names((await sdk.listSharedDiceSets()).data?.items)).toEqual(['Moss and Stone'])
  })

  it('rolls plain once no set is chosen, and chooses only the caller’s own', async () => {
    as('player')
    expect(status(await sdk.chooseDiceSet({ body: { diceSetId: dice(3) } }))).toBe(404)
    await sdk.chooseDiceSet({ body: {} })
    expect((await sdk.listDiceSets()).data?.chosen).toBeUndefined()
  })

  it('shows the review queue to an Admin only', async () => {
    as('admin')
    expect((await sdk.listDiceSetsToReview()).data).toEqual({ items: [] })
    as('player')
    expect(status(await sdk.listDiceSetsToReview())).toBe(403)
  })
})

describe('the Account', () => {
  it('changes the profile, unless the Username or email is taken', async () => {
    as('player')
    expect((await sdk.updateAccount({ body: { username: 'ash.reed', nickname: 'Ash R.', email: 'ash@demo.invalid' } })).data?.nickname).toBe('Ash R.')
    expect((await sdk.getAccount()).data?.username).toBe('ash.reed')
    expect(status(await sdk.updateAccount({ body: { username: 'brook', nickname: 'Ash', email: 'ash@demo.invalid' } }))).toBe(409)
  })

  it('reads the caller’s own history, empty for an Account with none', async () => {
    const demo = as('player')
    expect((await sdk.getAccountHistory()).data?.items.map((e) => e.action)).toEqual(['new_sign_in', 'password_set', 'created'])
    Reflect.deleteProperty(demo.world.personal.history, ash)
    expect((await sdk.getAccountHistory()).data?.items).toEqual([])
  })

  it('lists, mints and revokes the caller’s own Access Tokens', async () => {
    as('dm')
    expect((await sdk.listAccessTokens()).data?.items.map((t) => t.lastUsedAt)).toEqual(['2026-10-30T09:12:00Z'])
    as('player')
    const minted = (await sdk.createAccessToken({ body: { name: 'Scripts', scopes: ['read', 'play'], days: 30 } })).data
    expect(minted?.token).toMatch(/^gmt_[A-Za-z0-9_-]{43}$/)
    const mine = (await sdk.listAccessTokens()).data?.items
    expect(mine?.map((t) => [t.name, t.lastUsedAt])).toEqual([['Sheet on my tablet', undefined], ['Scripts', undefined]])
    await sdk.revokeAccessToken({ path: { accessId: minted?.accessToken.id ?? '' } })
    expect(status(await sdk.revokeAccessToken({ path: { accessId: personalId(0x601) } }))).toBe(404)
    expect((await sdk.listAccessTokens()).data?.items).toHaveLength(1)
  })

  it('keeps Notification preferences, with the API’s defaults and security always in app', async () => {
    as('dm')
    const prefs = (await sdk.getNotificationPreferences()).data?.items
    expect(prefs).toHaveLength(8)
    expect(prefs?.find((p) => p.kind === 'release_note')).toEqual({ kind: 'release_note', inApp: true, push: false, email: false })
    expect(prefs?.find((p) => p.kind === 'security')).toEqual({ kind: 'security', inApp: true, push: true, email: true })
    const set = await sdk.setNotificationPreferences({ body: { items: [{ kind: 'proposal', inApp: true, push: false, email: true }] } })
    expect(set.data?.items.find((p) => p.kind === 'proposal')).toEqual({ kind: 'proposal', inApp: true, push: false, email: true })
    expect(status(await sdk.setNotificationPreferences({ body: { items: [{ kind: 'security', inApp: false, push: false, email: false }] } }))).toBe(422)
    as('player')
    const mine = await sdk.setNotificationPreferences({ body: { items: [{ kind: 'conversation', inApp: false, push: false, email: false }] } })
    expect(mine.data?.items.filter((p) => !p.push).map((p) => p.kind)).toEqual(['conversation', 'release_note'])
  })
})

describe('signing in and out', () => {
  it('signs in as the Player, by password, and out again', async () => {
    const demo = as('signed-out')
    expect((await sdk.signIn({ body: { username: 'anyone', password: 'anything' } })).data).toMatchObject({ id: ash })
    expect(demo.world.persona).toBe('player')
    expect(status(await sdk.signOut())).toBe(204)
    expect(demo.world.persona).toBe('signed-out')
    expect(status(await sdk.requestSignInLink({ body: { email: 'ash@demo.invalid' } }))).toBe(204)
  })

  it('signs in with an emailed link once', async () => {
    const demo = as('signed-out')
    const token = 'a-sign-in-link-from-the-demo'
    expect((await sdk.useSignInLink({ body: { token } })).data).toMatchObject({ id: ash })
    expect(demo.world.persona).toBe('player')
    expect(status(await sdk.useSignInLink({ body: { token } }))).toBe(410)
  })

  it('sets up from an Account Invite once', async () => {
    const demo = as('signed-out')
    const token = 'an-account-invite-from-the-demo'
    expect((await sdk.previewAccountInvite({ body: { token } })).data?.admin).toBe(false)
    const setup = { token, username: 'rowan', nickname: 'Rowan', email: 'rowan@demo.invalid', password: 'a-long-password' }
    expect((await sdk.acceptAccountInvite({ body: setup })).data).toMatchObject({ id: ash })
    expect(demo.world.persona).toBe('player')
    expect(status(await sdk.previewAccountInvite({ body: { token } }))).toBe(410)
    expect(status(await sdk.acceptAccountInvite({ body: setup }))).toBe(410)
  })

  it('brings an external login back to choose an Account, then signs in', async () => {
    const demo = as('signed-out')
    const out = (await sdk.finishOidc({ body: { code: 'demo', state: 'state-from-the-provider' } })).data
    expect(out?.status).toBe('choose')
    const token = out?.pending?.token ?? ''
    expect((await sdk.createOidcAccount({ body: { token, username: 'rowan', nickname: 'Rowan' } })).data).toMatchObject({ id: ash })
    demo.world.persona = 'signed-out'
    expect((await sdk.linkOidcAccount({ body: { token, username: 'ash', password: 'anything' } })).data).toMatchObject({ id: ash })
    expect(demo.world.persona).toBe('player')
  })
})

describe('Admin', () => {
  it('lists every Account and Invite to an Admin, and refuses anyone else', async () => {
    as('admin')
    const list = (await sdk.listAdminAccounts()).data
    expect(list?.accounts.map((a) => a.username)).toEqual(['morgan', 'ash', 'brook', 'cedar', 'dune', 'fennel', 'admin'])
    expect(list?.accounts.find((a) => a.username === 'dune')?.lastSeenAt).toBeUndefined()
    expect(list?.invites.map((i) => i.status)).toEqual(['invited', 'expired'])
    as('dm')
    expect(status(await sdk.listAdminAccounts())).toBe(403)
    expect(status(await sdk.getAdminAccount({ path: { accountId: ash } }))).toBe(403)
  })

  it('reads one Account with its Campaigns, tokens and history', async () => {
    as('admin')
    expect((await sdk.getAdminAccount({ path: { accountId: ash } })).data).toMatchObject({
      status: 'active', tokens: 1, campaigns: [{ id: c, name: 'The Sunken Bell', role: 'player' }],
    })
    expect((await sdk.getAdminAccount({ path: { accountId: ids.outsider } })).data).toMatchObject({ campaigns: [], history: [], tokens: 0 })
    expect(status(await sdk.getAdminAccount({ path: { accountId: ids.session } }))).toBe(404)
  })

  it('invites, disables, makes Admin and emails a sign-in link, never to the Admin’s own Account', async () => {
    as('admin')
    expect((await sdk.createAccountInvite({ body: { hours: 24 } })).data?.token).toMatch(/^invite_/)
    await sdk.createAccountInvite({ body: { hours: 24, admin: true, email: 'fern@example.com' } })
    const invites = (await sdk.listAdminAccounts()).data?.invites
    expect(invites?.map((i) => i.admin)).toEqual([true, false, false, false])
    expect(invites?.map((i) => i.email)).toEqual(['fern@example.com', undefined, 'wren@example.com', undefined])
    expect(status(await sdk.setAccountDisabled({ path: { accountId: ids.admin }, body: { value: true } }))).toBe(409)
    expect(status(await sdk.setAccountDisabled({ path: { accountId: ids.session }, body: { value: true } }))).toBe(404)
    expect(status(await sdk.sendAdminSignInLink({ path: { accountId: brook } }))).toBe(204)
    await sdk.setAccountDisabled({ path: { accountId: brook }, body: { value: true } })
    expect(status(await sdk.sendAdminSignInLink({ path: { accountId: brook } }))).toBe(409)
    expect((await sdk.getAdminAccount({ path: { accountId: brook } })).data?.status).toBe('disabled')
    expect((await sdk.listAdminAccounts()).data?.accounts.find((a) => a.id === brook)?.status).toBe('disabled')
    await sdk.setAdminRole({ path: { accountId: cedar }, body: { value: true } })
    expect((await sdk.getAdminAccount({ path: { accountId: cedar } })).data?.account.admin).toBe(true)
  })
})

describe('the pages that open from a link', () => {
  const alerts = (w: Awaited<ReturnType<typeof mountDemo>>['wrapper']) => w.findAll('[role="alert"]').map((a) => a.text())

  it('sets up an Account from an invite link', async () => {
    const { wrapper, demo } = await mountDemo('/account-invite#an-account-invite-from-the-demo', 'signed-out')
    expect(alerts(wrapper)).toEqual([])
    expect(wrapper.find('[data-testid="account-setup"]').exists()).toBe(true)
    expect(demo.unhandled).toEqual([])
  })

  it('signs in from an emailed link and offers a new password', async () => {
    const { wrapper, demo } = await mountDemo('/sign-in-link#a-sign-in-link-from-the-demo', 'signed-out')
    expect(alerts(wrapper)).toEqual([])
    expect(wrapper.find('[data-testid="new-password"]').exists()).toBe(true)
    expect(demo.world.persona).toBe('player')
  })

  it('brings an external login back to the choice of Account', async () => {
    const { wrapper } = await mountDemo('/oidc/callback?code=demo&state=state-from-the-provider', 'signed-out')
    expect(alerts(wrapper)).toEqual([])
    expect(wrapper.find('[data-testid="oidc-choose"]').exists()).toBe(true)
  })

  it('signs in from the sign-in page and shows the Player as signed in', async () => {
    const { wrapper, demo, settle } = await mountDemo('/sign-in', 'signed-out')
    await wrapper.find('input[autocomplete="username"]').setValue('ash')
    await wrapper.find('input[type="password"]').setValue('anything')
    await wrapper.find('form').trigger('submit')
    await settle()
    expect(demo.world.persona).toBe('player')
    expect(wrapper.find('[data-testid="demo-as-player"]').attributes('checked')).toBeDefined()
  })
})
