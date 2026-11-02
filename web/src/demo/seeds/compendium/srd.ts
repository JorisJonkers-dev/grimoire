import type { ConditionRef, Entry, EntryKind, Source, Spell } from '@/infrastructure/api/types.gen'

// A small slice of the System Reference Document 5.2, enough to browse every kind the compendium
// shows. The text is SRD 5.2 (CC BY 4.0), shortened where an entry runs long; nothing here comes from
// outside the SRD.

export const srd52: Source = {
  key: 'srd-2024',
  title: 'System Reference Document 5.2',
  rulesetYear: 2024,
  license: 'CC-BY-4.0',
  attribution:
    'This work includes material from the System Reference Document 5.2 ("SRD 5.2") by Wizards of the Coast LLC, available at https://www.dndbeyond.com/srd. The SRD 5.2 is licensed under the Creative Commons Attribution 4.0 International License, available at https://creativecommons.org/licenses/by/4.0/legalcode.',
  url: 'https://www.dndbeyond.com/srd',
}

const conditionText = {
  incapacitated: ['Incapacitated', "You can't take any action, Bonus Action, or Reaction. Your Concentration is broken, and you can't speak."],
  paralyzed: ['Paralyzed', "You have the Incapacitated condition and your Speed is 0. You automatically fail Strength and Dexterity saving throws, attack rolls against you have Advantage, and a hit from within 5 feet is a Critical Hit."],
  poisoned: ['Poisoned', 'You have Disadvantage on attack rolls and ability checks.'],
  prone: ['Prone', 'Your only movement options are to crawl or to spend half your Speed to stand up. You have Disadvantage on attack rolls. An attack roll against you has Advantage if the attacker is within 5 feet of you; otherwise it has Disadvantage.'],
  frightened: ['Frightened', "You have Disadvantage on ability checks and attack rolls while the source of fear is within line of sight, and you can't willingly move closer to it."],
} satisfies Record<string, [string, string]>

type ConditionSlug = keyof typeof conditionText

/** A condition as the text that names it links to it. */
function mention(slug: ConditionSlug): ConditionRef {
  const [name, description] = conditionText[slug]
  return { slug, name, description }
}

const spell = (s: Omit<Spell, 'ruleset' | 'scaling' | 'mentions' | 'damageTypes' | 'attackRoll'> & Partial<Spell>): Spell => ({
  ruleset: 'srd-2024', scaling: [], mentions: [], damageTypes: [], attackRoll: false, ...s,
})

export const spells: Spell[] = [
  spell({
    slug: 'fire-bolt', name: 'Fire Bolt', level: 0, school: 'evocation', ritual: false, concentration: false,
    castingTime: 'Action', rangeText: '120 feet', rangeFeet: 120, verbal: true, somatic: true, material: false, duration: 'Instantaneous',
    description: 'You hurl a mote of fire at a creature or an object within range. Make a ranged spell attack against the target. On a hit, the target takes 1d10 Fire damage. A flammable object hit by this spell starts burning if it isn\'t being worn or carried.',
    higherLevel: 'The damage increases by 1d10 when you reach levels 5 (2d10), 11 (3d10), and 17 (4d10).',
    classes: ['sorcerer', 'wizard'], damageTypes: ['fire'], attackRoll: true, damageRoll: '1d10',
    scaling: [{ kind: 'character', level: 5, damageRoll: '2d10' }, { kind: 'character', level: 11, damageRoll: '3d10' }, { kind: 'character', level: 17, damageRoll: '4d10' }],
  }),
  spell({
    slug: 'cure-wounds', name: 'Cure Wounds', level: 1, school: 'abjuration', ritual: false, concentration: false,
    castingTime: 'Action', rangeText: 'Touch', verbal: true, somatic: true, material: false, duration: 'Instantaneous',
    description: 'A creature you touch regains a number of Hit Points equal to 2d8 plus your spellcasting ability modifier.',
    higherLevel: 'The healing increases by 2d8 for each spell slot level above 1.',
    classes: ['bard', 'cleric', 'druid', 'paladin', 'ranger'],
  }),
  spell({
    slug: 'detect-magic', name: 'Detect Magic', level: 1, school: 'divination', ritual: true, concentration: true,
    castingTime: 'Action or Ritual', rangeText: 'Self', verbal: true, somatic: true, material: false, duration: 'up to 10 minutes',
    description: 'For the duration, you sense the presence of magical effects within 30 feet of yourself. If you sense such effects, you can take the Magic action to see a faint aura around any visible creature or object in the area that bears the magic, and if an effect was created by a spell, you learn the spell\'s school of magic.',
    classes: ['bard', 'cleric', 'druid', 'paladin', 'ranger', 'sorcerer', 'warlock', 'wizard'],
  }),
  spell({
    slug: 'magic-missile', name: 'Magic Missile', level: 1, school: 'evocation', ritual: false, concentration: false,
    castingTime: 'Action', rangeText: '120 feet', rangeFeet: 120, verbal: true, somatic: true, material: false, duration: 'Instantaneous',
    description: 'You create three glowing darts of magical force. Each dart strikes a creature of your choice that you can see within range. A dart deals 1d4 + 1 Force damage to its target. The darts all strike simultaneously, and you can direct them to hit one creature or several.',
    higherLevel: 'The spell creates one more dart for each spell slot level above 1.',
    classes: ['sorcerer', 'wizard'], damageTypes: ['force'], damageRoll: '1d4+1',
  }),
  spell({
    slug: 'shield', name: 'Shield', level: 1, school: 'abjuration', ritual: false, concentration: false,
    castingTime: 'Reaction, which you take when you are hit by an attack roll or targeted by the Magic Missile spell', rangeText: 'Self',
    verbal: true, somatic: true, material: false, duration: '1 round',
    description: 'An imperceptible barrier of magical force protects you. Until the start of your next turn, you have a +5 bonus to AC, including against the triggering attack, and you take no damage from Magic Missile.',
    classes: ['sorcerer', 'wizard'],
  }),
  spell({
    slug: 'hold-person', name: 'Hold Person', level: 2, school: 'enchantment', ritual: false, concentration: true,
    castingTime: 'Action', rangeText: '60 feet', rangeFeet: 60, verbal: true, somatic: true, material: true, materialText: 'a straightened piece of iron',
    duration: 'up to 1 minute',
    description: 'Choose a Humanoid that you can see within range. The target must succeed on a Wisdom saving throw or have the Paralyzed condition for the duration. At the end of each of its turns, the target repeats the save, ending the spell on itself on a success.',
    higherLevel: 'You can target one additional Humanoid for each spell slot level above 2.',
    classes: ['bard', 'cleric', 'druid', 'sorcerer', 'warlock', 'wizard'], saveAbility: 'wisdom', mentions: [mention('paralyzed')],
  }),
  spell({
    slug: 'fireball', name: 'Fireball', level: 3, school: 'evocation', ritual: false, concentration: false,
    castingTime: 'Action', rangeText: '150 feet', rangeFeet: 150, verbal: true, somatic: true, material: true, materialText: 'a ball of bat guano and sulfur',
    duration: 'Instantaneous',
    description: 'A bright streak flashes from you to a point you choose within range and then blossoms with a low roar into a fiery explosion. Each creature in a 20-foot-radius Sphere centered on that point makes a Dexterity saving throw, taking 8d6 Fire damage on a failed save or half as much damage on a successful one. Flammable objects in the area that aren\'t being worn or carried start burning.',
    higherLevel: 'The damage increases by 1d6 for each spell slot level above 3.',
    classes: ['sorcerer', 'wizard'], damageTypes: ['fire'], saveAbility: 'dexterity', damageRoll: '8d6',
    scaling: [4, 5, 6, 7, 8, 9].map((level) => ({ kind: 'slot' as const, level, damageRoll: `${String(level + 5)}d6` })),
  }),
]

const entry = (kind: EntryKind, slug: string, name: string, subtitle: string, facts: [string, string][], sections: [string, string][], mentions: ConditionSlug[] = []): Entry => ({
  kind, slug, name, subtitle, ruleset: 'srd-2024',
  facts: facts.map(([label, value]) => ({ label, value })),
  sections: sections.map(([title, text]) => ({ title, text })),
  mentions: mentions.map(mention),
})

export const entries: Entry[] = [
  entry('class', 'fighter', 'Fighter', 'Hit Die d10 · Strength or Dexterity', [
    ['Hit Point Die', 'D10 per Fighter level'], ['Saving Throw Proficiencies', 'Strength and Constitution'],
    ['Armor Training', 'Light, Medium, and Heavy armor and Shields'], ['Weapon Proficiencies', 'Simple and Martial weapons'],
  ], [
    ['Second Wind (Level 1)', 'As a Bonus Action, you can regain Hit Points equal to 1d10 plus your Fighter level. You can use this feature twice, regaining one use on a Short Rest and all uses on a Long Rest.'],
    ['Action Surge (Level 2)', 'On your turn, you can take one additional action, except the Magic action. Once you use this feature, you can\'t do so again until you finish a Short or Long Rest.'],
  ]),
  entry('class', 'wizard', 'Wizard', 'Hit Die d6 · Intelligence', [
    ['Hit Point Die', 'D6 per Wizard level'], ['Saving Throw Proficiencies', 'Intelligence and Wisdom'], ['Weapon Proficiencies', 'Simple weapons'],
  ], [
    ['Arcane Recovery (Level 1)', 'When you finish a Short Rest, you can recover expended spell slots with a combined level of no more than half your Wizard level (round up), none of level 6 or higher. Once you do so, you can\'t again until you finish a Long Rest.'],
  ]),
  entry('species', 'dwarf', 'Dwarf', 'Medium Humanoid', [['Size', 'Medium (about 4–5 feet tall)'], ['Speed', '30 feet']], [
    ['Darkvision', 'You have Darkvision with a range of 120 feet.'],
    ['Dwarven Resilience', 'You have Resistance to Poison damage. You also have Advantage on saving throws you make to avoid or end the Poisoned condition.'],
  ], ['poisoned']),
  entry('species', 'halfling', 'Halfling', 'Small Humanoid', [['Size', 'Small (about 2–3 feet tall)'], ['Speed', '30 feet']], [
    ['Brave', 'You have Advantage on saving throws you make to avoid or end the Frightened condition.'],
    ['Luck', 'When you roll a 1 on the d20 of a D20 Test, you can reroll the die, and you must use the new roll.'],
  ], ['frightened']),
  entry('background', 'sage', 'Sage', 'Constitution, Intelligence, Wisdom', [
    ['Feat', 'Magic Initiate (Wizard)'], ['Skill Proficiencies', 'Arcana and History'], ['Tool Proficiency', "Calligrapher's Supplies"],
  ], [['Equipment', "Choose A or B: (A) Quarterstaff, Calligrapher's Supplies, Book (history), Parchment (8 sheets), Robe, 8 GP; or (B) 50 GP"]]),
  entry('background', 'soldier', 'Soldier', 'Strength, Dexterity, Constitution', [
    ['Feat', 'Savage Attacker'], ['Skill Proficiencies', 'Athletics and Intimidation'], ['Tool Proficiency', 'Choose one kind of Gaming Set'],
  ], [['Equipment', 'Choose A or B: (A) Spear, Shortbow, 20 Arrows, Gaming Set, Healer\'s Kit, Quiver, Traveler\'s Clothes, 14 GP; or (B) 50 GP']]),
  entry('feat', 'alert', 'Alert', 'Origin Feat', [], [
    ['Initiative Proficiency', 'When you roll Initiative, you can add your Proficiency Bonus to the roll.'],
    ['Initiative Swap', 'Immediately after you roll Initiative, you can swap your Initiative with the Initiative of one willing ally in the same combat. You can\'t make this swap if you or the ally has the Incapacitated condition.'],
  ], ['incapacitated']),
  entry('feat', 'skilled', 'Skilled', 'Origin Feat', [], [
    ['Benefit', 'You gain proficiency in any combination of three skills or tools of your choice.'],
    ['Repeatable', 'You can take this feat more than once.'],
  ]),
  entry('weapon', 'longsword', 'Longsword', 'Martial Melee Weapon', [['Damage', '1d8 Slashing'], ['Properties', 'Versatile (1d10)'], ['Mastery', 'Sap'], ['Weight', '3 lb.'], ['Cost', '15 GP']], []),
  entry('weapon', 'scimitar', 'Scimitar', 'Martial Melee Weapon', [['Damage', '1d6 Slashing'], ['Properties', 'Finesse, Light'], ['Mastery', 'Nick'], ['Weight', '3 lb.'], ['Cost', '25 GP']], []),
  entry('weapon', 'shortbow', 'Shortbow', 'Simple Ranged Weapon', [['Damage', '1d6 Piercing'], ['Properties', 'Ammunition (Range 80/320; Arrow), Two-Handed'], ['Mastery', 'Vex'], ['Weight', '2 lb.'], ['Cost', '25 GP']], []),
  entry('armor', 'chain-mail', 'Chain Mail', 'Heavy Armor', [['Armor Class', '16'], ['Strength', 'Str 13'], ['Stealth', 'Disadvantage'], ['Weight', '55 lb.'], ['Cost', '75 GP']], []),
  entry('armor', 'leather-armor', 'Leather Armor', 'Light Armor', [['Armor Class', '11 + Dex modifier'], ['Weight', '10 lb.'], ['Cost', '10 GP']], []),
  entry('item', 'torch', 'Torch', 'Adventuring Gear', [['Weight', '1 lb.'], ['Cost', '1 CP']], [
    ['Use', 'A Torch burns for 1 hour, casting Bright Light in a 20-foot radius and Dim Light for an additional 20 feet. When you take the Attack action, you can attack with the Torch, using it as a Simple Melee weapon. On a hit, the target takes 1 Fire damage.'],
  ]),
  entry('item', 'rope', 'Rope', 'Adventuring Gear', [['Weight', '5 lb.'], ['Cost', '1 GP']], [
    ['Use', 'As a Utilize action, you can tie a knot with Rope if you are within 5 feet of it. Rope can be burst with a successful DC 20 Strength (Athletics) check.'],
  ]),
  entry('magic-item', 'potion-of-healing', 'Potion of Healing', 'Potion, Common', [['Rarity', 'Common']], [
    ['Description', 'You regain 2d4 + 2 Hit Points when you drink this potion. The potion\'s red liquid glimmers when agitated.'],
  ]),
  entry('magic-item', 'bag-of-holding', 'Bag of Holding', 'Wondrous Item, Uncommon', [['Rarity', 'Uncommon']], [
    ['Description', 'This bag has an interior space considerably larger than its outside dimensions. The bag can hold up to 500 pounds, not exceeding a volume of 64 cubic feet, and weighs 5 pounds regardless of its contents. Retrieving an item from the bag requires a Utilize action.'],
  ]),
  entry('monster', 'goblin-warrior', 'Goblin Warrior', 'CR 1/4 · Small Fey (Goblinoid)', [
    ['Armor Class', '15'], ['Hit Points', '10 (3d6)'], ['Speed', '30 ft.'], ['Skills', 'Stealth +6'], ['Senses', 'Darkvision 60 ft.; Passive Perception 9'],
    ['Languages', 'Common, Goblin'],
  ], [
    ['Nimble Escape', 'Bonus Action: The goblin takes the Disengage or Hide action.'],
    ['Scimitar', 'Melee Attack Roll: +4, reach 5 ft. Hit: 5 (1d6 + 2) Slashing damage, plus 2 (1d4) Slashing damage if the attack roll had Advantage.'],
    ['Shortbow', 'Ranged Attack Roll: +4, range 80/320 ft. Hit: 5 (1d6 + 2) Piercing damage, plus 2 (1d4) Piercing damage if the attack roll had Advantage.'],
  ]),
  entry('monster', 'skeleton', 'Skeleton', 'CR 1/4 · Medium Undead', [
    ['Armor Class', '14'], ['Hit Points', '13 (2d8 + 4)'], ['Speed', '30 ft.'], ['Vulnerabilities', 'Bludgeoning'], ['Immunities', 'Poison; Exhaustion, Poisoned'],
  ], [
    ['Shortsword', 'Melee Attack Roll: +5, reach 5 ft. Hit: 6 (1d6 + 3) Piercing damage.'],
    ['Shortbow', 'Ranged Attack Roll: +5, range 80/320 ft. Hit: 6 (1d6 + 3) Piercing damage.'],
  ], ['poisoned']),
  entry('monster', 'wolf', 'Wolf', 'CR 1/4 · Medium Beast', [['Armor Class', '12'], ['Hit Points', '11 (2d8 + 2)'], ['Speed', '40 ft.']], [
    ['Pack Tactics', "The wolf has Advantage on an attack roll against a creature if at least one of the wolf's allies is within 5 feet of the creature and the ally doesn't have the Incapacitated condition."],
    ['Bite', 'Melee Attack Roll: +4, reach 5 ft. Hit: 5 (1d6 + 2) Piercing damage. If the target is a Medium or smaller creature, it has the Prone condition.'],
  ], ['incapacitated', 'prone']),
  ...(Object.keys(conditionText) as ConditionSlug[]).map((slug) => {
    const [name, text] = conditionText[slug]
    return entry('condition', slug, name, 'Condition', [], [['Effects', text]])
  }),
]

/** How far the rules engine runs each record, for the automation page: most of the SRD is computed. */
export const automation: Record<string, 'full' | 'partial' | 'manual'> = {
  'detect-magic': 'manual',
  'hold-person': 'partial',
  'bag-of-holding': 'manual',
  wolf: 'partial',
}
