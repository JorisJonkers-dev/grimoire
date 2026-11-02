import type { Handlers } from '../operations'
import { accountHandlers } from './account'
import { campaignHandlers } from './campaigns'
import { charactersHandlers } from './characters'
import { compendiumHandlers } from './compendium'
import { personalHandlers } from './personal'
import { prepHandlers } from './prep'

/** Every operation the demo answers. */
export const handlers: Handlers = {
  ...accountHandlers,
  ...campaignHandlers,
  ...personalHandlers,
  ...compendiumHandlers,
  ...charactersHandlers,
  ...prepHandlers,
}
