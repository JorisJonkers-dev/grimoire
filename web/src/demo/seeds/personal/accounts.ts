import type { AccessToken, AccountEvent, NotificationPreference, ReleaseNote } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import { personalId } from './ids'

// What surrounds each Account: when it was made and last seen, its history, its Access Tokens and
// its Notification preferences, and what an Admin sees beyond that: the Account Invites and the
// Release Notes.

export type AccountMeta = { createdAt: string; lastSeenAt?: string; disabled: boolean; sessions: number }
export type DemoToken = AccessToken & { accountId: string }
export type AccountInviteRecord = { id: string; admin: boolean; createdAt: string; expiresAt: string; open: boolean }

export function seedAccounts() {
  const [ash, brook, cedar, dune] = ids.players
  const { dm, outsider: fennel, admin } = ids
  const meta: Record<string, AccountMeta> = {
    [admin]: { createdAt: '2026-04-10T08:00:00Z', lastSeenAt: '2026-11-02T09:00:00Z', disabled: false, sessions: 1 },
    [dm]: { createdAt: '2026-04-11T19:00:00Z', lastSeenAt: '2026-11-01T22:30:00Z', disabled: false, sessions: 2 },
    [ash]: { createdAt: '2026-05-02T18:10:00Z', lastSeenAt: '2026-11-01T20:05:00Z', disabled: false, sessions: 1 },
    [brook]: { createdAt: '2026-05-02T18:40:00Z', lastSeenAt: '2026-11-01T19:58:00Z', disabled: false, sessions: 1 },
    [cedar]: { createdAt: '2026-05-03T12:00:00Z', lastSeenAt: '2026-10-31T15:30:00Z', disabled: false, sessions: 1 },
    [dune]: { createdAt: '2026-05-04T17:25:00Z', disabled: false, sessions: 0 },
    [fennel]: { createdAt: '2026-06-01T10:00:00Z', lastSeenAt: '2026-10-29T08:31:00Z', disabled: false, sessions: 1 },
  }
  const made = (accountId: string, by = 'Admin'): AccountEvent => ({ at: (meta[accountId] as AccountMeta).createdAt, actor: by, action: 'created', detail: 'Set up from an Account Invite' })
  const history: Record<string, AccountEvent[]> = {
    [admin]: [
      { at: '2026-04-11T08:05:00Z', actor: 'Admin', action: 'two_step_on', detail: '' },
      { at: '2026-04-10T08:00:00Z', actor: 'Admin', action: 'admin_granted', detail: 'The first Account is an Admin' },
      made(admin, 'Grimoire'),
    ],
    [dm]: [
      { at: '2026-11-01T18:55:00Z', actor: 'Morgan', action: 'new_sign_in', detail: 'Firefox on Linux' },
      { at: '2026-04-11T19:02:00Z', actor: 'Morgan', action: 'password_set', detail: '' },
      made(dm),
    ],
    [ash]: [
      { at: '2026-10-20T18:30:00Z', actor: 'Ash', action: 'new_sign_in', detail: 'Safari on iPhone' },
      { at: '2026-05-02T18:12:00Z', actor: 'Ash', action: 'password_set', detail: '' },
      made(ash),
    ],
  }
  const tokens: DemoToken[] = [
    { id: personalId(0x601), accountId: dm, name: 'Prep scripts', scopes: ['read', 'build'], createdAt: '2026-09-01T10:00:00Z', expiresAt: '2027-03-01T10:00:00Z', lastUsedAt: '2026-10-30T09:12:00Z' },
    { id: personalId(0x602), accountId: ash, name: 'Sheet on my tablet', scopes: ['read'], createdAt: '2026-10-05T17:00:00Z', expiresAt: '2027-01-03T17:00:00Z' },
  ]
  /** Only the choices an Account made; every other one is the default, as in the API. */
  const preferences: Record<string, NotificationPreference[]> = {
    [ash]: [{ kind: 'session_reminder', inApp: true, push: true, email: true }],
  }
  const releaseNotes: ReleaseNote[] = [
    {
      id: personalId(0x701), version: '1.5.0', title: 'Dice of your own', status: 'published',
      body: '- Make a Dice Set and roll with it at the table.\n- Share a set with your Friends, or take a copy of theirs.\n- Conversations now link the Characters you mention.',
      createdAt: '2026-10-20T09:00:00Z', updatedAt: '2026-10-21T09:00:00Z', publishAt: '2026-10-21T09:00:00Z',
    },
    {
      id: personalId(0x702), version: '1.6.0', title: 'Calendars for every Campaign', status: 'draft',
      body: 'Plan the next Session where the whole table can see it.', createdAt: '2026-10-31T14:00:00Z', updatedAt: '2026-10-31T14:00:00Z',
    },
  ]
  /** The Release Notes each Account has dismissed. */
  const seenNotes: Record<string, string[]> = { [dm]: [personalId(0x701)], [admin]: [personalId(0x701)] }
  const accountInvites: AccountInviteRecord[] = [
    { id: personalId(0x801), admin: false, createdAt: '2026-10-30T12:00:00Z', expiresAt: '2026-11-02T12:00:00Z', open: true },
    { id: personalId(0x802), admin: false, createdAt: '2026-09-01T12:00:00Z', expiresAt: '2026-09-04T12:00:00Z', open: false },
  ]
  /** Account Invite and sign-in link tokens that were used up; they never work again. */
  const spentTokens: string[] = []
  return { meta, history, tokens, preferences, releaseNotes, seenNotes, accountInvites, spentTokens }
}
