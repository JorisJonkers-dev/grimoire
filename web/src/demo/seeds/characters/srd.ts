import type {
  Ability,
  ArmorOptionItem,
  BackgroundChoice,
  ClassChoice,
  SkillChoice,
  SpeciesChoice,
  SpellPick,
  TraitLine,
  WeaponLine,
} from '@/infrastructure/api/types.gen'

// The SRD 5.2 rules the demo's Characters are built from (System Reference Document 5.2, CC BY 4.0).
// Only what the four seeded Characters and the builder's first page need: the API reads these from the
// compendium, the demo keeps a small table of them.

export const abilities: Ability[] = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma']

export const skills: SkillChoice[] = [
  { skill: 'acrobatics', ability: 'dexterity' },
  { skill: 'animal-handling', ability: 'wisdom' },
  { skill: 'arcana', ability: 'intelligence' },
  { skill: 'athletics', ability: 'strength' },
  { skill: 'deception', ability: 'charisma' },
  { skill: 'history', ability: 'intelligence' },
  { skill: 'insight', ability: 'wisdom' },
  { skill: 'intimidation', ability: 'charisma' },
  { skill: 'investigation', ability: 'intelligence' },
  { skill: 'medicine', ability: 'wisdom' },
  { skill: 'nature', ability: 'intelligence' },
  { skill: 'perception', ability: 'wisdom' },
  { skill: 'performance', ability: 'charisma' },
  { skill: 'persuasion', ability: 'charisma' },
  { skill: 'religion', ability: 'intelligence' },
  { skill: 'sleight-of-hand', ability: 'dexterity' },
  { skill: 'stealth', ability: 'dexterity' },
  { skill: 'survival', ability: 'wisdom' },
]

export const classes: ClassChoice[] = [
  { slug: 'barbarian', name: 'Barbarian', hitDie: 12, saves: ['strength', 'constitution'], skillChoices: 2, primaryAbilities: ['strength'], caster: 'none' },
  { slug: 'bard', name: 'Bard', hitDie: 8, saves: ['dexterity', 'charisma'], skillChoices: 3, primaryAbilities: ['charisma'], caster: 'full' },
  { slug: 'cleric', name: 'Cleric', hitDie: 8, saves: ['wisdom', 'charisma'], skillChoices: 2, primaryAbilities: ['wisdom'], caster: 'full' },
  { slug: 'druid', name: 'Druid', hitDie: 8, saves: ['intelligence', 'wisdom'], skillChoices: 2, primaryAbilities: ['wisdom'], caster: 'full' },
  { slug: 'fighter', name: 'Fighter', hitDie: 10, saves: ['strength', 'constitution'], skillChoices: 2, primaryAbilities: ['strength', 'dexterity'], caster: 'none' },
  { slug: 'monk', name: 'Monk', hitDie: 8, saves: ['strength', 'dexterity'], skillChoices: 2, primaryAbilities: ['dexterity', 'wisdom'], caster: 'none' },
  { slug: 'paladin', name: 'Paladin', hitDie: 10, saves: ['wisdom', 'charisma'], skillChoices: 2, primaryAbilities: ['strength', 'charisma'], caster: 'half' },
  { slug: 'ranger', name: 'Ranger', hitDie: 10, saves: ['strength', 'dexterity'], skillChoices: 3, primaryAbilities: ['dexterity', 'wisdom'], caster: 'half' },
  { slug: 'rogue', name: 'Rogue', hitDie: 8, saves: ['dexterity', 'intelligence'], skillChoices: 4, primaryAbilities: ['dexterity'], caster: 'none' },
  { slug: 'sorcerer', name: 'Sorcerer', hitDie: 6, saves: ['constitution', 'charisma'], skillChoices: 2, primaryAbilities: ['charisma'], caster: 'full' },
  { slug: 'warlock', name: 'Warlock', hitDie: 8, saves: ['wisdom', 'charisma'], skillChoices: 2, primaryAbilities: ['charisma'], caster: 'pact' },
  { slug: 'wizard', name: 'Wizard', hitDie: 6, saves: ['intelligence', 'wisdom'], skillChoices: 2, primaryAbilities: ['intelligence'], caster: 'full' },
]

export const species: SpeciesChoice[] = [
  { slug: 'dragonborn', name: 'Dragonborn', speedFeet: 30 },
  { slug: 'dwarf', name: 'Dwarf', speedFeet: 30 },
  { slug: 'elf', name: 'Elf', speedFeet: 30 },
  { slug: 'gnome', name: 'Gnome', speedFeet: 30 },
  { slug: 'goliath', name: 'Goliath', speedFeet: 35 },
  { slug: 'halfling', name: 'Halfling', speedFeet: 30 },
  { slug: 'human', name: 'Human', speedFeet: 30 },
  { slug: 'orc', name: 'Orc', speedFeet: 30 },
  { slug: 'tiefling', name: 'Tiefling', speedFeet: 30 },
]

/** Each SRD background, the abilities its +2/+1 may go to, its skills and the origin feat it grants. */
export const backgrounds: (BackgroundChoice & { feat: string })[] = [
  { slug: 'acolyte', name: 'Acolyte', abilities: ['intelligence', 'wisdom', 'charisma'], skills: ['insight', 'religion'], feat: 'Magic Initiate (Cleric)' },
  { slug: 'criminal', name: 'Criminal', abilities: ['dexterity', 'constitution', 'intelligence'], skills: ['sleight-of-hand', 'stealth'], feat: 'Alert' },
  { slug: 'sage', name: 'Sage', abilities: ['constitution', 'intelligence', 'wisdom'], skills: ['arcana', 'history'], feat: 'Magic Initiate (Wizard)' },
  { slug: 'soldier', name: 'Soldier', abilities: ['strength', 'dexterity', 'constitution'], skills: ['athletics', 'intimidation'], feat: 'Savage Attacker' },
]

const armor = (slug: string, name: string, category: string, acBase: number, dexCap: number | null, strengthRequired = 0, stealthDisadvantage = false): ArmorOptionItem => ({
  slug, name, category, shield: false, acBase, addDex: dexCap !== 0, ...(dexCap ? { dexCap } : {}), strengthRequired, stealthDisadvantage,
})

export const armors: ArmorOptionItem[] = [
  armor('padded-armor', 'Padded Armor', 'light', 11, null, 0, true),
  armor('leather-armor', 'Leather Armor', 'light', 11, null),
  armor('studded-leather-armor', 'Studded Leather Armor', 'light', 12, null),
  armor('hide-armor', 'Hide Armor', 'medium', 12, 2),
  armor('chain-shirt', 'Chain Shirt', 'medium', 13, 2),
  armor('scale-mail', 'Scale Mail', 'medium', 14, 2, 0, true),
  armor('breastplate', 'Breastplate', 'medium', 14, 2),
  armor('half-plate-armor', 'Half Plate Armor', 'medium', 15, 2, 0, true),
  armor('ring-mail', 'Ring Mail', 'heavy', 14, 0, 0, true),
  armor('chain-mail', 'Chain Mail', 'heavy', 16, 0, 13, true),
  armor('splint-armor', 'Splint Armor', 'heavy', 17, 0, 15, true),
  armor('plate-armor', 'Plate Armor', 'heavy', 18, 0, 15, true),
  { slug: 'shield', name: 'Shield', category: 'shield', shield: true, acBase: 2, addDex: false, strengthRequired: 0, stealthDisadvantage: false },
]

/** A weapon with what an attack with it needs: its properties and its Weapon Mastery. */
export type Weapon = WeaponLine & { finesse: boolean; thrown: boolean; mastery: string; weightLb: number }

const weapon = (slug: string, name: string, damageDice: string, damageType: string, mastery: string, weightLb: number, extra: Partial<Weapon> = {}): Weapon => ({
  slug, name, damageDice, damageType, rangeFeet: 0, longRangeFeet: 0, finesse: false, thrown: false, mastery, weightLb, ...extra,
})

export const weapons: Weapon[] = [
  weapon('dagger', 'Dagger', '1d4', 'piercing', 'Nick', 1, { finesse: true, thrown: true, rangeFeet: 20, longRangeFeet: 60 }),
  weapon('handaxe', 'Handaxe', '1d6', 'slashing', 'Vex', 2, { thrown: true, rangeFeet: 20, longRangeFeet: 60 }),
  weapon('javelin', 'Javelin', '1d6', 'piercing', 'Slow', 2, { thrown: true, rangeFeet: 30, longRangeFeet: 120 }),
  weapon('mace', 'Mace', '1d6', 'bludgeoning', 'Sap', 4),
  weapon('quarterstaff', 'Quarterstaff', '1d6', 'bludgeoning', 'Topple', 4),
  weapon('light-crossbow', 'Light Crossbow', '1d8', 'piercing', 'Slow', 5, { rangeFeet: 80, longRangeFeet: 320 }),
  weapon('shortbow', 'Shortbow', '1d6', 'piercing', 'Vex', 2, { rangeFeet: 80, longRangeFeet: 320 }),
  weapon('shortsword', 'Shortsword', '1d6', 'piercing', 'Vex', 2, { finesse: true }),
  weapon('longsword', 'Longsword', '1d8', 'slashing', 'Sap', 3),
  weapon('rapier', 'Rapier', '1d8', 'piercing', 'Vex', 2, { finesse: true }),
  weapon('warhammer', 'Warhammer', '1d8', 'bludgeoning', 'Push', 5),
  weapon('longbow', 'Longbow', '1d8', 'piercing', 'Slow', 2, { rangeFeet: 150, longRangeFeet: 600 }),
]

/** The weapon a slug names, as the builder offers it. */
export const weaponLine = ({ slug, name, damageDice, damageType, rangeFeet, longRangeFeet }: Weapon): WeaponLine => ({ slug, name, damageDice, damageType, rangeFeet, longRangeFeet })

/** How a class trains its Characters, how many weapons it masters, and how it casts. */
export type ClassRules = {
  armor: string[]
  weapons: string[]
  masteries: number
  /** Cantrips known and spells prepared at levels 1 to 4; empty for a class that casts none. */
  cantrips: number[]
  prepared: number[]
  spellbook: boolean
  features: TraitLine[]
}

const feature = (level: number, name: string, description: string): TraitLine => ({ name, source: 'class', level, description })

export const classRules: Record<string, ClassRules> = {
  rogue: {
    armor: ['Light armor'], weapons: ['Simple weapons', 'Martial weapons with the Finesse or Light property'], masteries: 2, cantrips: [], prepared: [], spellbook: false,
    features: [
      feature(1, 'Expertise', 'Twice the Proficiency Bonus on two chosen skills.'),
      feature(1, 'Sneak Attack', 'Once per turn, 2d6 extra damage with a Finesse or Ranged weapon when the attack has Advantage or an ally is next to the target.'),
      feature(1, "Thieves' Cant", 'A secret mix of dialect, jargon and code, and one more language.'),
      feature(1, 'Weapon Mastery', 'Uses the mastery properties of two kinds of weapons.'),
      feature(2, 'Cunning Action', 'Dash, Disengage or Hide as a Bonus Action.'),
      feature(3, 'Steady Aim', 'Advantage on the next attack this turn as a Bonus Action, if it has not moved.'),
      feature(3, 'Fast Hands', 'Sleight of Hand, a lock, a trap or an object as a Bonus Action.'),
      feature(3, 'Second-Story Work', 'Climbs at full Speed and jumps farther.'),
    ],
  },
  fighter: {
    armor: ['Light armor', 'Medium armor', 'Heavy armor', 'Shields'], weapons: ['Simple weapons', 'Martial weapons'], masteries: 3, cantrips: [], prepared: [], spellbook: false,
    features: [
      feature(1, 'Fighting Style: Defense', '+1 to Armor Class while wearing armor.'),
      feature(1, 'Second Wind', 'Regains 1d10 + Fighter level Hit Points as a Bonus Action, twice per Long Rest.'),
      feature(1, 'Weapon Mastery', 'Uses the mastery properties of three kinds of weapons.'),
      feature(2, 'Action Surge', 'One more action on its turn, once per Short or Long Rest.'),
      feature(2, 'Tactical Mind', 'Spends a use of Second Wind to add 1d10 to a failed ability check.'),
      feature(3, 'Improved Critical', 'Weapon attacks score a Critical Hit on a roll of 19 or 20.'),
      feature(3, 'Remarkable Athlete', 'Advantage on Initiative and on Strength (Athletics) checks.'),
    ],
  },
  wizard: {
    armor: [], weapons: ['Simple weapons'], masteries: 0, cantrips: [3, 3, 3, 4], prepared: [4, 5, 6, 7], spellbook: true,
    features: [
      feature(1, 'Spellcasting', 'Casts wizard spells from a spellbook, with Intelligence.'),
      feature(1, 'Ritual Adept', 'Casts any Ritual spell in the spellbook without preparing it.'),
      feature(1, 'Arcane Recovery', 'Recovers spell slots on a Short Rest, once per day.'),
      feature(2, 'Scholar', 'Expertise in Arcana.'),
      feature(3, 'Evocation Savant', 'Two Evocation spells are copied into the spellbook for free.'),
      feature(3, 'Potent Cantrip', 'A damaging cantrip that misses or is saved against still deals half damage.'),
    ],
  },
  cleric: {
    armor: ['Light armor', 'Medium armor', 'Heavy armor', 'Shields'], weapons: ['Simple weapons', 'Martial weapons'], masteries: 0, cantrips: [3, 3, 3, 4], prepared: [4, 5, 6, 7], spellbook: false,
    features: [
      feature(1, 'Spellcasting', 'Prepares cleric spells each day, with Wisdom.'),
      feature(1, 'Divine Order: Protector', 'Trained with Martial weapons and Heavy armor.'),
      feature(2, 'Channel Divinity', 'Divine Spark or Turn Undead, twice per Long Rest.'),
      feature(3, 'Disciple of Life', 'Healing spells restore extra Hit Points equal to 2 + the spell slot level.'),
      feature(3, 'Preserve Life', 'Channel Divinity to share out five times the Cleric level in Hit Points.'),
      feature(3, 'Life Domain Spells', 'Aid, Bless, Cure Wounds and Lesser Restoration are always prepared.'),
    ],
  },
}

const trait = (name: string, description: string): TraitLine => ({ name, source: 'species', level: 1, description })

export const speciesTraits: Record<string, TraitLine[]> = {
  halfling: [
    trait('Brave', 'Advantage on saving throws against the Frightened condition.'),
    trait('Halfling Nimbleness', 'Moves through the space of any creature a size larger.'),
    trait('Luck', 'Rerolls a 1 on a d20 Test.'),
    trait('Naturally Stealthy', 'Hides even when only a larger creature obscures it.'),
  ],
  human: [
    trait('Resourceful', 'Gains Heroic Inspiration after each Long Rest.'),
    trait('Skillful', 'Proficiency in one more skill.'),
    trait('Versatile', 'One more Origin feat.'),
  ],
  elf: [
    trait('Darkvision', 'Sees in darkness within 60 feet as if it were dim light.'),
    trait('Fey Ancestry', 'Advantage on saving throws against the Charmed condition.'),
    trait('Keen Senses', 'Proficiency in Insight, Perception or Survival.'),
    trait('Trance', 'A Long Rest takes 4 hours of meditation.'),
  ],
  dwarf: [
    trait('Darkvision', 'Sees in darkness within 120 feet as if it were dim light.'),
    trait('Dwarven Resilience', 'Resistance to Poison damage, and Advantage on saves against the Poisoned condition.'),
    trait('Dwarven Toughness', 'One more Hit Point per level.'),
    trait('Stonecunning', 'Tremorsense within 60 feet while on stone, as a Bonus Action.'),
  ],
}

const spell = (slug: string, name: string, level: number, ritual = false): SpellPick => ({ slug, name, level, ...(ritual ? { ritual } : {}) })

/** Each caster class's list, up to 2nd-level spells. */
export const spellLists: Record<string, SpellPick[]> = {
  wizard: [
    spell('fire-bolt', 'Fire Bolt', 0),
    spell('light', 'Light', 0),
    spell('mage-hand', 'Mage Hand', 0),
    spell('minor-illusion', 'Minor Illusion', 0),
    spell('prestidigitation', 'Prestidigitation', 0),
    spell('ray-of-frost', 'Ray of Frost', 0),
    spell('burning-hands', 'Burning Hands', 1),
    spell('comprehend-languages', 'Comprehend Languages', 1, true),
    spell('detect-magic', 'Detect Magic', 1, true),
    spell('feather-fall', 'Feather Fall', 1),
    spell('identify', 'Identify', 1, true),
    spell('mage-armor', 'Mage Armor', 1),
    spell('magic-missile', 'Magic Missile', 1),
    spell('shield', 'Shield', 1),
    spell('sleep', 'Sleep', 1),
    spell('thunderwave', 'Thunderwave', 1),
    spell('invisibility', 'Invisibility', 2),
    spell('misty-step', 'Misty Step', 2),
    spell('scorching-ray', 'Scorching Ray', 2),
    spell('web', 'Web', 2),
  ],
  cleric: [
    spell('guidance', 'Guidance', 0),
    spell('light', 'Light', 0),
    spell('sacred-flame', 'Sacred Flame', 0),
    spell('spare-the-dying', 'Spare the Dying', 0),
    spell('thaumaturgy', 'Thaumaturgy', 0),
    spell('bless', 'Bless', 1),
    spell('command', 'Command', 1),
    spell('cure-wounds', 'Cure Wounds', 1),
    spell('detect-magic', 'Detect Magic', 1, true),
    spell('guiding-bolt', 'Guiding Bolt', 1),
    spell('healing-word', 'Healing Word', 1),
    spell('sanctuary', 'Sanctuary', 1),
    spell('shield-of-faith', 'Shield of Faith', 1),
    spell('aid', 'Aid', 2),
    spell('hold-person', 'Hold Person', 2),
    spell('lesser-restoration', 'Lesser Restoration', 2),
    spell('prayer-of-healing', 'Prayer of Healing', 2),
    spell('spiritual-weapon', 'Spiritual Weapon', 2),
  ],
}

/** Spells a subclass keeps prepared without counting them, from level 3. */
export const alwaysPrepared: Record<string, string[]> = { life: ['aid', 'bless', 'cure-wounds', 'lesser-restoration'] }

/** Spell slots a full caster has at levels 1 to 4, by spell level. */
export const fullCasterSlots = [[2], [3], [4, 2], [4, 3]] as const
