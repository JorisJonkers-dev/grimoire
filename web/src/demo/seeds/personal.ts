import type { Core } from '../world'
import { seedAccounts } from './personal/accounts'
import { seedDice } from './personal/dice'
import { seedSocial } from './personal/social'

/** The personal part of the seeded world: each Account's own things, its Friends and Conversations. */
export function seedPersonal(core: Core) {
  return { ...seedSocial(core), ...seedDice(), ...seedAccounts() }
}

export type Personal = ReturnType<typeof seedPersonal>
