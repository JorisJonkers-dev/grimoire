import type { Account, AdminAccountRow, NotificationKind, NotificationPreference } from '@/infrastructure/api/types.gen'
import { DemoProblem, notFound, type Handlers } from '../operations'
import { freshId, now } from '../seeds/personal/ids'
import type { AccountMeta } from '../seeds/personal/accounts'
import type { World } from '../world'
import { admin, signedIn } from './access'

// The Account itself, signing in and out, and what an Admin does to Accounts. Every way of signing in
// lands as the Player, Ash: the demo has one person to be, and the role switcher picks the others.

const kinds: NotificationKind[] = ['proposal', 'join_request', 'level_up', 'friend_request', 'conversation', 'session_reminder', 'release_note', 'security']

/** What each kind does when the Account chose nothing for it: the API's defaults. */
function preferencesOf(world: World, accountId: string): NotificationPreference[] {
  const chosen = world.personal.preferences[accountId] ?? []
  return kinds.map((kind) => chosen.find((p) => p.kind === kind) ?? { kind, inApp: true, push: kind !== 'release_note', email: kind === 'security' })
}

function enter(world: World): Account {
  world.persona = 'player'
  return world.accounts.player
}

// A link or invite works once; a second use is told it is gone, as the API tells it.
function spend(world: World, token: string): void {
  if (world.personal.spentTokens.includes(token)) throw new DemoProblem(410, 'This link has been used or has expired')
  world.personal.spentTokens.push(token)
}

const inHours = (hours: number) => new Date(Date.now() + hours * 3_600_000).toISOString()
const metaOf = (world: World, accountId: string) => world.personal.meta[accountId] as AccountMeta
const started = now()

/** The Account an Admin acts on; an Admin cannot act on their own Account this way. */
function target(world: World, who: Parameters<typeof admin>[0], accountId: string): Account {
  const me = admin(who)
  const account = world.accounts.all.find((a) => a.id === accountId)
  if (!account) throw notFound('Account')
  if (account.id === me.id) throw new DemoProblem(409, 'You cannot do that to your own Account')
  return account
}

export const accountHandlers = {
  getAccount: ({ who }) => signedIn(who),
  getMe: ({ who }) => ({ subject: `account:${signedIn(who).id}` }),
  getStatus: () => ({ service: 'grimoire' as const, version: 'demo', database: 'up' as const, startedAt: started }),
  updateAccount: ({ who, world, body }) => {
    const me = signedIn(who)
    if (world.accounts.all.some((a) => a.id !== me.id && (a.username === body.username || a.email === body.email))) throw new DemoProblem(409, 'That Username or email is taken')
    Object.assign(me, { username: body.username, nickname: body.nickname, email: body.email })
    return me
  },
  getAccountHistory: ({ who, world }) => ({ items: world.personal.history[signedIn(who).id] ?? [] }),
  listAccessTokens: ({ who, world }) => {
    const me = signedIn(who)
    return { items: world.personal.tokens.filter((t) => t.accountId === me.id).map((t) => ({ id: t.id, name: t.name, scopes: t.scopes, createdAt: t.createdAt, expiresAt: t.expiresAt, ...(t.lastUsedAt ? { lastUsedAt: t.lastUsedAt } : {}) })) }
  },
  createAccessToken: ({ who, world, body }) => {
    const me = signedIn(who)
    const accessToken = { id: freshId(), name: body.name, scopes: body.scopes, createdAt: now(), expiresAt: inHours(body.days * 24) }
    world.personal.tokens.push({ ...accessToken, accountId: me.id })
    return { token: `gmt_${`demo_${accessToken.id.replace(/-/g, '')}`.padEnd(43, '0')}`, accessToken }
  },
  revokeAccessToken: ({ who, world, path }) => {
    const me = signedIn(who)
    const at = world.personal.tokens.findIndex((t) => t.id === path.accessId && t.accountId === me.id)
    if (at < 0) throw notFound('Access Token')
    world.personal.tokens.splice(at, 1)
  },
  getNotificationPreferences: ({ who, world }) => ({ items: preferencesOf(world, signedIn(who).id) }),
  setNotificationPreferences: ({ who, world, body }) => {
    const me = signedIn(who)
    if (body.items.some((p) => p.kind === 'security' && !p.inApp)) throw new DemoProblem(422, 'Security Notifications always show in app')
    const kept = (world.personal.preferences[me.id] ?? []).filter((p) => !body.items.some((q) => q.kind === p.kind))
    world.personal.preferences[me.id] = [...kept, ...body.items]
    return { items: preferencesOf(world, me.id) }
  },

  // Signing in and out. The demo offers no external login: there is no provider to send anyone to.
  getSignInMethods: () => ({}),
  signIn: ({ world }) => enter(world),
  signOut: ({ world }) => {
    world.persona = 'signed-out'
  },
  requestSignInLink: () => undefined,
  useSignInLink: ({ world, body }) => {
    spend(world, body.token)
    return enter(world)
  },
  previewAccountInvite: ({ world, body }) => {
    if (world.personal.spentTokens.includes(body.token)) throw new DemoProblem(410, 'This invite has been used or has expired')
    return { expiresAt: inHours(72), admin: false }
  },
  acceptAccountInvite: ({ world, body }) => {
    spend(world, body.token)
    return enter(world)
  },
  // An external login comes back as one no Account has yet, so the choice between a new Account and
  // linking an existing one can be seen.
  finishOidc: () => ({ status: 'choose' as const, pending: { token: 'demo-external-login-waiting', email: 'rowan.mire@demo.invalid', username: 'rowan.mire', name: 'Rowan Mire' } }),
  createOidcAccount: ({ world }) => enter(world),
  linkOidcAccount: ({ world }) => enter(world),

  // What an Admin sees and does. Anyone else is refused, as the API refuses them.
  listAdminAccounts: ({ who, world }) => {
    admin(who)
    const accounts = world.accounts.all.map((a): AdminAccountRow => {
      const m = metaOf(world, a.id)
      return { id: a.id, username: a.username, nickname: a.nickname, email: a.email, admin: a.admin, status: m.disabled ? 'disabled' : 'active', createdAt: m.createdAt, ...(m.lastSeenAt ? { lastSeenAt: m.lastSeenAt } : {}) }
    })
    const invites = world.personal.accountInvites.map((i) => ({ id: i.id, admin: i.admin, status: i.open ? ('invited' as const) : ('expired' as const), createdAt: i.createdAt, expiresAt: i.expiresAt }))
    return { accounts, invites }
  },
  getAdminAccount: ({ who, world, path }) => {
    admin(who)
    const account = world.accounts.all.find((a) => a.id === path.accountId)
    if (!account) throw notFound('Account')
    const m = metaOf(world, account.id)
    const membership = world.members.find((x) => x.accountId === account.id)
    return {
      account,
      status: m.disabled ? ('disabled' as const) : ('active' as const),
      createdAt: m.createdAt,
      sessions: m.sessions,
      tokens: world.personal.tokens.filter((t) => t.accountId === account.id).length,
      campaigns: membership ? [{ id: world.campaign.id, name: world.campaign.name, role: membership.role }] : [],
      history: world.personal.history[account.id] ?? [],
    }
  },
  createAccountInvite: ({ who, world, body }) => {
    admin(who)
    const invite = { id: freshId(), admin: body.admin ?? false, createdAt: now(), expiresAt: inHours(body.hours), open: true }
    world.personal.accountInvites.unshift(invite)
    return { token: `invite_${invite.id.replace(/-/g, '')}`, expiresAt: invite.expiresAt }
  },
  setAccountDisabled: ({ who, world, path, body }) => {
    const account = target(world, who, path.accountId)
    metaOf(world, account.id).disabled = body.value
  },
  setAdminRole: ({ who, world, path, body }) => {
    target(world, who, path.accountId).admin = body.value
  },
  sendAdminSignInLink: ({ who, world, path }) => {
    const account = target(world, who, path.accountId)
    if (metaOf(world, account.id).disabled) throw new DemoProblem(409, 'A disabled Account gets no sign-in link')
  },
  listReleaseNotes: ({ who, world }) => {
    admin(who)
    return { items: [...world.personal.releaseNotes].sort((a, b) => b.createdAt.localeCompare(a.createdAt)) }
  },
} satisfies Handlers
