import type { Account } from '@/infrastructure/api/types.gen'
import type { World } from './world'

/** Who the demo is being looked at as. */
export type Persona = 'signed-out' | 'player' | 'dm' | 'admin'

export const personas: { id: Persona; label: string }[] = [
  { id: 'signed-out', label: 'Signed out' },
  { id: 'player', label: 'Player' },
  { id: 'dm', label: 'DM' },
  { id: 'admin', label: 'Admin' },
]

/** The caller a handler answers, as the API's audience filter sees it. */
export type Viewer = {
  persona: Persona
  account: Account | null
  /** The caller's role in the seeded Campaign, if they are a Member of it. */
  role: 'dm' | 'player' | null
}

export function viewerOf(world: Pick<World, 'persona' | 'accounts'>): Viewer {
  const persona = world.persona
  switch (persona) {
    case 'signed-out':
      return { persona, account: null, role: null }
    case 'player':
      return { persona, account: world.accounts.player, role: 'player' }
    case 'dm':
      return { persona, account: world.accounts.dm, role: 'dm' }
    case 'admin':
      return { persona, account: world.accounts.admin, role: null }
  }
}
