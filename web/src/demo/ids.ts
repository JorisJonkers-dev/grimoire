// Stable ids for everything the seeded world holds, so a link into the demo keeps working across
// reloads and the page list can name a real record for every route.

const id = (tail: string) => `0190c7a8-0000-7000-8000-${tail.padStart(12, '0')}`

export const ids = {
  campaign: id('c001'),
  dm: id('a001'),
  players: [id('a002'), id('a003'), id('a004'), id('a005')] as [string, string, string, string],
  /** An Account with no Campaign: a friend of the Players, and the one an Admin opens. */
  outsider: id('a006'),
  admin: id('a009'),
  members: [id('b001'), id('b002'), id('b003'), id('b004'), id('b005')] as [string, string, string, string, string],
  characters: [id('d001'), id('d002'), id('d003'), id('d004')] as [string, string, string, string],
  /** The first Player's Character Draft, filed as the pending Join Request. */
  draft: id('d009'),
  invite: id('e001'),
  /** The link token of the seeded Campaign Invite: 43 URL-safe characters, as the API mints them. */
  inviteToken: 'SunkenBell-demo-invite-link-000000000000001',
  joinRequest: id('e002'),
  session: id('f001'),
  map: id('f101'),
  npc: id('f201'),
  proposal: id('f301'),
  conversation: id('f401'),
  /** One Library Entry per builder, all owned by the DM. */
  library: {
    spell: id('f501'),
    item: id('f502'),
    subclass: id('f503'),
    class: id('f504'),
    species: id('f505'),
    feat: id('f506'),
    background: id('f507'),
    condition: id('f508'),
    table: id('f509'),
    monster: id('f50a'),
  },
} as const

/** Compendium records the page list opens; they are SRD 5.2 content. */
export const slugs = { spell: 'fireball', entryKind: 'monster', entry: 'goblin-warrior' } as const
