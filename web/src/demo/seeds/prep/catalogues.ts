import type { FactionArchetype, HookPointSlug, RuleVariantOption } from '@/infrastructure/api/types.gen'

// The fixed tables the API keeps in code rather than in a Campaign: the built-in Rule Variants, the
// hook points a DM's own Rule Variant hangs on, and the generic Faction Archetypes. They are copied
// from the rules packages so the demo shows the same choices production does.

export type VariantInfo = { slug: string; name: string; description: string; automated: boolean; options: RuleVariantOption[] }

const onOff: RuleVariantOption[] = [{ value: 'off', label: 'Off' }, { value: 'on', label: 'On' }]

export const ruleVariantCatalogue: VariantInfo[] = [
  { slug: 'flanking', name: 'Flanking', description: 'A melee attack has Advantage when an ally who can act stands on the hex straight across the target.', options: onOff, automated: true },
  { slug: 'critical-fumble', name: 'Critical fumbles', description: 'A natural 1 on an attack roll is a fumble: something goes wrong for the attacker, as the DM rules.', options: onOff, automated: false },
  {
    slug: 'critical-hits', name: 'Critical hits', description: 'What a Critical Hit does to the damage dice.', automated: true,
    options: [{ value: 'double-dice', label: 'Roll every damage die twice' }, { value: 'max-dice', label: 'Roll the dice once and add the most they could show' }],
  },
  {
    slug: 'rests', name: 'Rest lengths', description: 'How long a rest takes on the Game Clock.', automated: true,
    options: [
      { value: 'standard', label: 'Standard: a Short Rest is 1 hour, a Long Rest 8 hours' },
      { value: 'gritty', label: 'Gritty: a Short Rest is 8 hours, a Long Rest 7 days' },
      { value: 'epic', label: 'Epic: a Short Rest is 5 minutes, a Long Rest 1 hour' },
    ],
  },
  {
    slug: 'short-rest-cap', name: 'Short Rest cap', description: 'How many Short Rests the party may take between two Long Rests.', automated: true,
    options: [{ value: 'none', label: 'No cap' }, { value: '1', label: 'One' }, { value: '2', label: 'Two' }, { value: '3', label: 'Three' }],
  },
  { slug: 'massive-damage', name: 'Massive damage', description: 'A creature that takes half its hit point maximum or more from one blow makes a DC 15 Constitution save or suffers system shock.', options: onOff, automated: false },
  { slug: 'hidden-death-saves', name: 'Hidden death saves', description: 'The DM rolls death saving throws out of sight, and only the DM knows how they stand.', options: onOff, automated: false },
  { slug: 'healing-surge', name: 'Healing surges', description: 'As an action, a Character spends up to half its Hit Dice to heal.', options: onOff, automated: false },
  { slug: 'slow-natural-healing', name: 'Slow natural healing', description: 'A Long Rest gives back no hit points; a Character spends Hit Dice during it instead.', options: onOff, automated: true },
  { slug: 'morale', name: 'Morale', description: 'A creature down to half its hit points, or whose side has lost half its number, makes a DC 10 Wisdom save or flees.', options: onOff, automated: false },
  {
    slug: 'encumbrance', name: 'Encumbrance', description: 'How a heavy load slows a creature.', automated: false,
    options: [
      { value: 'standard', label: 'Standard: past 15 lb a point of Strength, 5 feet' },
      { value: 'variant', label: 'Variant: 10 feet slower past 5 lb a point, 20 feet past 10 lb' },
      { value: 'off', label: 'Off: a load never slows anyone' },
    ],
  },
]

export const hookPoints: { slug: HookPointSlug; label: string }[] = [
  { slug: 'natural-1', label: 'On a natural 1 on an attack roll' },
  { slug: 'critical', label: 'On a natural 20 on an attack roll' },
  { slug: 'drop-to-0', label: 'On dropping to 0 hit points' },
  { slug: 'rest', label: 'On finishing a rest' },
  { slug: 'cast', label: 'On casting a spell' },
]

export const factionArchetypes: FactionArchetype[] = [
  { slug: 'arcane-college', name: 'Arcane college', description: 'Scholars and mages who teach, hoard and argue over magic behind well-warded doors.', goals: 'Gather lore and relics, guard dangerous knowledge, and keep magic in learned hands.' },
  { slug: 'city-watch', name: 'City watch', description: 'The sworn keepers of order in a town or city: patrols, gaolers and the officers over them.', goals: 'Keep the peace, enforce the law of the place, and answer to whoever pays their wage.' },
  { slug: 'cult', name: 'Cult', description: 'A hidden circle bound to a forbidden power, a doomed prophecy or a leader who promises both.', goals: 'Grow in secret, serve what they worship, and bring about what it has promised them.' },
  { slug: 'druid-circle', name: 'Druid circle', description: 'Wardens of a stretch of wild land who speak for its beasts, its trees and its seasons.', goals: 'Protect their land from those who would spoil it, and keep the old balance there.' },
  { slug: 'knightly-order', name: 'Knightly order', description: 'Sworn companions bound by an oath, a code and a long roll of honoured dead.', goals: 'Uphold their oath, defend those in their charge, and win renown worthy of the order.' },
  { slug: 'mercenary-company', name: 'Mercenary company', description: 'Soldiers for hire under a captain and a contract, loyal to their pay and to each other.', goals: 'Find paying work, finish what they are paid for, and keep the company whole.' },
  { slug: 'merchant-league', name: 'Merchant league', description: 'Trading houses in common cause, with caravans, warehouses and a long reach along the roads.', goals: 'Open markets, keep the roads safe for trade, and settle disputes in coin rather than blood.' },
  { slug: 'noble-house', name: 'Noble house', description: 'An old family with land, a name and retainers, and rivals of the same kind.', goals: "Raise the family's standing, hold its lands, and outlast the houses that oppose it." },
  { slug: 'smuggling-ring', name: 'Smuggling ring', description: 'Boatmen, carters and fixers who move what the law forbids past those who would tax or seize it.', goals: 'Keep their routes open and unseen, and their cargo moving at a profit.' },
  { slug: 'temple', name: 'Temple', description: 'The clergy and lay folk of a faith, with a house of worship and a place in the life around it.', goals: 'Serve their faith, tend to the faithful, and spread or defend what they hold sacred.' },
  { slug: 'thieves-guild', name: "Thieves' guild", description: 'Burglars, cutpurses and fences under one roof, with rules about whose streets are whose.', goals: 'Take their share of what moves through the city, and keep outsiders off their ground.' },
]
