import type {
  BackgroundBuild, ClassBuild, ConditionBuild, FeatBuild, ItemBuild, MonsterBuild, PartField, PartHolds, RollTableBuild, SpeciesBuild, SpellBuild,
  SubclassBuild,
} from '@/infrastructure/api/types.gen'
import { castingFor } from '@/features/library/classPresets'
import served from '../../../../../fixtures/part-kinds.json'
import { ids } from '../../ids'

// The DM's homebrew as each builder opens it. The API works a build's lines, card and estimate out of
// the design; the demo holds them as written, since it saves no design and previews none.

/** What each builder answers, keyed by the Library kind it opens, without the entry it reads. */
export type Builds = {
  spell: Omit<SpellBuild, 'entry'>
  item: Omit<ItemBuild, 'entry'>
  subclass: Omit<SubclassBuild, 'entry'>
  class: Omit<ClassBuild, 'entry'>
  species: Omit<SpeciesBuild, 'entry'>
  feat: Omit<FeatBuild, 'entry'>
  background: Omit<BackgroundBuild, 'entry'>
  condition: Omit<ConditionBuild, 'entry'>
  table: Omit<RollTableBuild, 'entry'>
  creature: Omit<MonsterBuild, 'entry'>
}

export type BuilderKind = keyof Builds

const holds = served.holds as { spell: PartHolds; condition: PartHolds }
const activation = served.activation.spell as PartField[]
const slug = (id: string) => `hb-${id.replaceAll('-', '').slice(-12)}`

/** A sphere of a radius in hexes, as the spell's area preview draws it. */
function disc(radius: number) {
  const out: { q: number; r: number }[] = []
  for (let q = -radius; q <= radius; q++) {
    for (let r = Math.max(-radius, -q - radius); r <= Math.min(radius, -q + radius); r++) out.push({ q, r })
  }
  return out
}

/** Where a new entry starts in its builder, before the DM shapes it. */
export const blank: Builds = {
  spell: {
    design: { targeting: { shape: 'sphere', sizeFt: 5, rangeFt: 30 }, concentration: false, duration: { unit: 'instant' }, ritual: false, castingTime: { kind: 'action' }, components: { verbal: true, somatic: true }, parts: [] },
    activation, effect: '', text: ['Casting Time: 1 action.'], hexes: disc(0), holds: holds.spell,
  },
  item: {
    design: { kind: 'trinket', rarity: 'common', enchantment: 0, weightLb: 1, valueGp: 50, properties: [] },
    card: ['Trinket, common'], price: { points: 0, suggested: 'common', priceGp: 50, fits: true, notes: [] },
  },
  subclass: { design: { class: 'fighter', features: [{ level: 3, name: 'First feature', text: '' }], resources: [], choices: [] }, lines: ['Fighter subclass'] },
  class: {
    design: { hitDie: 8, primary: ['strength'], saves: ['strength', 'constitution'], armor: ['light'], weapons: ['simple'], skills: 2, subclassLevel: 3, featLevels: [4, 8, 12, 16, 19], columns: [], features: [{ level: 1, name: 'First feature', text: '' }], casting: { kind: 'none' } },
    lines: ['Hit Die d8', 'No spellcasting'],
  },
  species: { design: { sizes: ['medium'], creatureType: 'humanoid', speedFt: 30, speeds: [], senses: [], resistances: [], traits: [], spells: [], lineages: [] }, lines: ['Speed 30 feet.'] },
  feat: { design: { category: 'general', text: '', prerequisites: [] }, lines: ['General feat'] },
  background: { design: { abilities: ['strength', 'dexterity', 'constitution'], skills: ['athletics', 'survival'], feat: 'alert', featName: 'Alert', gold: 50 }, lines: ['Ability Scores: Strength, Dexterity, Constitution'] },
  condition: { design: { icon: 'spiral', color: '#bfb199', text: '', ends: 'removed', perLevel: { d20: 0, speedFt: 0, deathAt: 0 }, parts: [] }, lines: ['Lasts until removed.'], holds: holds.condition },
  table: { design: { dice: '1d20', results: [] }, lines: ['Roll 1d20.'] },
  creature: {
    design: {
      size: 'medium', creatureType: 'beast', ac: 12, hp: 11, speedFt: 30, challenge: 0.5, abilities: { strength: 10, dexterity: 10, constitution: 10, intelligence: 10, wisdom: 10, charisma: 10 },
      saves: [], senses: [], resistances: [], immunities: [], vulnerabilities: [], threshold: 0, traits: [], multiattack: 0, actions: [], phases: [],
    },
    lines: ['Medium Beast'], estimate: '1/2',
  },
}

const lib = ids.library

/** The DM's own builds for The Sunken Bell, one per builder. */
export function seedBuilds(): { [K in BuilderKind]: Record<string, Builds[K]> } {
  return {
    spell: {
      [lib.spell]: {
        design: {
          targeting: { shape: 'sphere', sizeFt: 15, rangeFt: 60 }, save: 'constitution', concentration: false, duration: { unit: 'instant' }, ritual: false,
          castingTime: { kind: 'action' }, components: { verbal: true, somatic: true, material: { text: 'a cracked bronze bell' } },
          parts: [
            { kind: 'save_damage', values: { dice: { text: '3d8' }, damage_type: { text: 'thunder' }, half: { flag: true } }, groups: [] },
            { kind: 'save_condition', values: { condition: { text: 'deafened' } }, groups: [] },
          ],
        },
        activation, effect: slug(lib.spell), hexes: disc(3), holds: holds.spell,
        text: [
          'Casting Time: 1 action.',
          'Area: a 15-foot-radius sphere within 60 feet.',
          'Each creature in the area makes a Constitution saving throw, taking 3d8 thunder damage on a failure or half as much on a success.',
          'A creature that fails is deafened.',
        ],
      },
    },
    item: {
      [lib.item]: {
        design: {
          kind: 'amulet', rarity: 'uncommon', enchantment: 0, weightLb: 1, valueGp: 400, attunement: {},
          properties: [{ type: 'speed', speed: 'swim', feet: 30 }, { type: 'sense', sense: 'darkvision', feet: 60 }],
        },
        slug: slug(lib.item),
        card: ['Drowned Bell Clapper', 'Amulet, uncommon (requires attunement)', 'Swim speed 30 feet.', 'Darkvision 60 feet.'],
        price: { points: 2, suggested: 'uncommon', priceGp: 400, fits: true, notes: [] },
      },
    },
    subclass: {
      [lib.subclass]: {
        design: {
          class: 'paladin',
          features: [
            { level: 3, name: 'Toll of Warding', text: 'When an ally within 30 feet of you is hit, you can take a Reaction to give it a +2 bonus to AC against that attack.' },
            { level: 7, name: 'Undertow Aura', text: 'Hostile creatures that start their turn within 10 feet of you have their Speed halved until the turn ends.' },
          ],
          resources: [], choices: [],
        },
        slug: slug(lib.subclass),
        lines: ['Paladin subclass', 'Level 3: Toll of Warding.', 'Level 7: Undertow Aura.'],
      },
    },
    class: {
      [lib.class]: {
        design: {
          hitDie: 10, primary: ['wisdom'], saves: ['constitution', 'wisdom'], armor: ['light', 'medium', 'shield'], weapons: ['simple'], skills: 2, subclassLevel: 3,
          featLevels: [4, 8, 12, 16, 19], columns: [],
          features: [
            { level: 1, name: 'Tidesense', text: 'You always know the direction of the nearest body of water within 1 mile.' },
            { level: 2, name: 'Bell Ward', text: 'As a Bonus Action, you ring a ward around you; until the start of your next turn, you and allies within 10 feet have Advantage on saves against the Frightened condition.' },
          ],
          casting: castingFor('half', { kind: 'half', ability: 'wisdom', spellList: 'cleric' }),
        },
        slug: slug(lib.class),
        lines: ['Hit Die d10', 'Spellcasting: Wisdom, from the cleric list, half caster'],
      },
    },
    species: {
      [lib.species]: {
        design: {
          sizes: ['medium'], creatureType: 'humanoid', speedFt: 30, speeds: [{ kind: 'swim', feet: 30 }], senses: [{ kind: 'darkvision', feet: 60 }], resistances: ['cold'],
          traits: [{ name: 'Deep Lungs', text: 'You can hold your breath for up to 1 hour.' }], spells: [], lineages: [],
        },
        slug: slug(lib.species),
        lines: ['Tideborn: 30 feet; Swim 30 feet.', 'Darkvision 60 feet.', 'Resistance to cold damage.'],
      },
    },
    feat: {
      [lib.feat]: {
        design: { category: 'general', text: 'Increase your Wisdom by 1. When you take the Dodge action, the next attack against an ally within 5 feet of you has Disadvantage.', prerequisites: [{ kind: 'level', minimum: 4, group: 0 }] },
        slug: slug(lib.feat),
        lines: ['General feat', 'Prerequisites: Level 4+'],
      },
    },
    background: {
      [lib.background]: {
        design: {
          abilities: ['strength', 'dexterity', 'constitution'], skills: ['athletics', 'survival'], feat: 'skilled', featName: 'Skilled', tool: "Navigator's Tools",
          equipment: 'Rope, Crowbar, Lantern, Traveler\'s Clothes', gold: 12, text: 'You hauled cargo up from wrecks in the bay before the Bell rang.',
        },
        slug: slug(lib.background),
        lines: ['Ability Scores: Strength, Dexterity, Constitution', 'Skill Proficiencies: Athletics, Survival', 'Feat: Skilled'],
      },
    },
    condition: {
      [lib.condition]: {
        design: {
          icon: 'drop', color: '#4f7fa8', text: 'Soaked through and weighed down.', ends: 'rest', perLevel: { d20: 0, speedFt: 0, deathAt: 0 },
          parts: [{ kind: 'speed_penalty', values: { ft: { number: 10 } }, groups: [] }],
        },
        slug: slug(lib.condition), holds: holds.condition,
        lines: ['Ends on a rest.', 'Speed is reduced by 10 feet.'],
      },
    },
    table: {
      [lib.table]: {
        design: {
          dice: '1d6',
          results: [
            { from: 1, to: 2, text: 'A barnacled strongbox, rusted shut.' },
            { from: 3, to: 4, text: 'Fifty feet of waterlogged rope.', item: 'rope', quantity: 1 },
            { from: 5, to: 5, text: 'A torch wrapped in oilcloth, still dry.', item: 'torch', quantity: 1 },
            { from: 6, to: 6, text: 'A bronze bell shard that hums when struck.' },
          ],
        },
        lines: ['Roll 1d6.', '1–2: A barnacled strongbox, rusted shut.', '3–4: Fifty feet of waterlogged rope.', '5: A torch wrapped in oilcloth, still dry.', '6: A bronze bell shard that hums when struck.'],
      },
    },
    creature: {
      [lib.monster]: {
        design: {
          size: 'medium', creatureType: 'undead', ac: 13, hp: 45, speedFt: 30, challenge: 2,
          abilities: { strength: 14, dexterity: 12, constitution: 14, intelligence: 6, wisdom: 10, charisma: 8 },
          saves: [], senses: [{ kind: 'darkvision', feet: 60 }], resistances: ['cold'], immunities: ['poison'], vulnerabilities: [], threshold: 0,
          traits: [{ name: 'Waterlogged', text: 'The ringer has Advantage on saving throws against being knocked Prone.' }],
          multiattack: 2,
          actions: [
            { name: 'Bell Rope', kind: 'melee', toHit: 4, reachFt: 10, damage: '1d8', damageBonus: 2, damageType: 'bludgeoning' },
            { name: 'Drowning Toll', kind: 'save', saveAbility: 'constitution', dc: 12, damage: '2d6', damageType: 'thunder', recharge: 5, text: 'Each creature within 15 feet makes the save.' },
          ],
          phases: [],
        },
        slug: slug(lib.monster),
        lines: ['Medium Undead', 'Multiattack: 2 attacks.', 'Drowning Toll (Recharge 5–6).'],
        estimate: '2',
      },
    },
  }
}
