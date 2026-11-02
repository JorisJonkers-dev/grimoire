import type { Account } from '@/infrastructure/api/types.gen'
import { forbidden, notFound, unauthorized } from '../operations'
import type { Viewer } from '../role'
import type { World } from '../world'

/** The signed-in Account, or the 401 the API sends a caller without a session. */
export function signedIn(who: Viewer): Account {
  if (!who.account) throw unauthorized()
  return who.account
}

/** The caller's role in the seeded Campaign; a stranger to it is told it does not exist. */
export function member(who: Viewer, campaignId: string, world: Pick<World, 'campaign'>): 'dm' | 'player' {
  signedIn(who)
  if (campaignId !== world.campaign.id || !who.role) throw notFound('Campaign')
  return who.role
}

/** Only the DM of the Campaign may go on. */
export function dm(who: Viewer, campaignId: string, world: Pick<World, 'campaign'>): void {
  if (member(who, campaignId, world) !== 'dm') throw forbidden()
}

/** Only an Admin may go on. */
export function admin(who: Viewer): Account {
  const me = signedIn(who)
  if (!me.adminPowers) throw forbidden()
  return me
}
