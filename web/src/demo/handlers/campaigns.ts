import type { TrackStanding } from '@/infrastructure/api/types.gen'
import { notFound, type Handlers } from '../operations'
import { campaignFor, summaryFor } from '../world'
import { dm, member, signedIn } from './access'

// A token as an invite link carries it: 32 random bytes, base64url without padding.
function newToken(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(32))
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

const DAY = 24 * 60 * 60 * 1000

export const campaignHandlers = {
  listCampaigns: ({ who, world }) => {
    signedIn(who)
    return { items: who.role ? [summaryFor(world, who.role)] : [] }
  },
  getCampaign: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    return campaignFor(world, signedIn(who).id)
  },

  listInvites: ({ who, world, path }) => {
    dm(who, path.campaignId, world)
    return world.invites
  },
  createInvite: ({ who, world, path }) => {
    dm(who, path.campaignId, world)
    const now = new Date()
    const invite = { id: crypto.randomUUID(), createdAt: now.toISOString(), expiresAt: new Date(now.getTime() + 7 * DAY).toISOString(), createdBy: signedIn(who).nickname }
    const token = newToken()
    world.invites.push(invite)
    world.characterData.inviteTokens[invite.id] = token
    return { ...invite, token }
  },
  revokeInvite: ({ who, world, path }) => {
    dm(who, path.campaignId, world)
    const at = world.invites.findIndex((i) => i.id === path.inviteId)
    if (at < 0) throw notFound('Campaign Invite')
    world.invites.splice(at, 1)
    return undefined
  },
  // Anyone signed in may see where a link leads; a link that is revoked or unknown leads nowhere.
  previewInvite: ({ who, world, body }) => {
    signedIn(who)
    const invite = world.invites.find((i) => world.characterData.inviteTokens[i.id] === body.token)
    if (!invite) throw notFound('Campaign Invite')
    return { campaignName: world.campaign.name, invitedBy: invite.createdBy }
  },

  listSessions: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    return [world.prep.live.session, ...world.characterData.sessions].sort((a, b) => b.number - a.number)
  },
  // Every Member sees the party's allies; the notes on them are the DM's alone.
  listCompanions: ({ who, world, path }) => {
    const role = member(who, path.campaignId, world)
    return world.characterData.companions.map((c) => (role === 'dm' ? c : { ...c, notes: '' }))
  },
  // The DM sees every score and the thresholds; a Player the party's score and their own Characters'.
  listTracks: ({ who, world, path }) => {
    const role = member(who, path.campaignId, world)
    const me = signedIn(who).id
    const tracks = world.characterData.tracks.map(({ values, thresholds, ...t }) => {
      const standings: TrackStanding[] =
        t.scope === 'party'
          ? [{ name: '', value: values.party ?? t.start }]
          : world.characters.filter((c) => role === 'dm' || c.ownerId === me).map((c) => ({ characterId: c.id, name: c.name, value: values[c.id] ?? t.start }))
      return { ...t, ...(role === 'dm' && thresholds ? { thresholds } : {}), standings }
    })
    return { dm: role === 'dm', tracks }
  },
} satisfies Handlers
