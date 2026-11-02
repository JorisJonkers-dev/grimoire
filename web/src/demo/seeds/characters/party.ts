import type { BuildSnapshot, EquipmentSlot, ItemCard, LiveCoins, PickLine } from '@/infrastructure/api/types.gen'
import { ids } from '../../ids'
import type { Build, Learned } from './sheet'

// The four Characters of The Sunken Bell at third level, each a legal SRD 5.2 build: the standard array,
// the background's +2/+1, the class's skills and the picks each level asked for.

const [tamsin, oswin, liri, brannoc] = ids.characters

const pick = (level: number, choice: string, value: string): PickLine => ({ level, choice, value })
const learned = (klass: string, spellbook: boolean) => (slug: string, prepared = false): Learned => ({ slug, class: klass, prepared, spellbook })
const plain = { xp: 900, levelUpReady: false, heroicInspiration: false, tempHp: 0, hitDiceSpent: 0, slotsSpent: [], effects: [], spells: [] }

export const builds: Record<string, Build> = {
  [tamsin]: {
    ...plain,
    species: 'halfling', class: 'rogue', background: 'criminal', method: 'standard-array',
    base: { strength: 8, dexterity: 15, constitution: 13, intelligence: 12, wisdom: 14, charisma: 10 },
    bonus: { dexterity: 2, constitution: 1 },
    classSkills: ['acrobatics', 'deception', 'insight', 'perception'],
    armor: 'leather-armor', shield: false, weapons: ['shortsword', 'dagger', 'shortbow'],
    picks: [pick(1, 'expertise', 'stealth'), pick(1, 'expertise', 'perception'), pick(3, 'subclass', 'thief')],
    // She has the XP for fourth level and the DM has let her take it.
    xp: 2700, levelUpReady: true, hitDiceSpent: 1,
    appearance: 'Small and quick, with ink-stained fingers and a coat of too many pockets.',
    backstory: 'Raised on the wharves of Saltmere, Tamsin picked locks for the tide-gangs until the Bell rang under the water.',
  },
  [oswin]: {
    ...plain,
    species: 'human', class: 'fighter', background: 'soldier', method: 'standard-array',
    base: { strength: 15, dexterity: 13, constitution: 14, intelligence: 8, wisdom: 12, charisma: 10 },
    bonus: { strength: 1, constitution: 2 },
    classSkills: ['perception', 'history'],
    armor: 'chain-mail', shield: true, weapons: ['longsword', 'javelin', 'handaxe'],
    picks: [pick(1, 'fighting-style', 'defense'), pick(3, 'subclass', 'champion')],
    heroicInspiration: true, tempHp: 4,
    appearance: 'Broad and weathered, his shield painted with a faded harbour gull.',
    backstory: 'Oswin held the sea wall during the drowning storm and has not slept well since.',
  },
  [liri]: {
    ...plain,
    species: 'elf', class: 'wizard', background: 'sage', method: 'standard-array',
    base: { strength: 8, dexterity: 14, constitution: 13, intelligence: 15, wisdom: 12, charisma: 10 },
    bonus: { intelligence: 2, wisdom: 1 },
    classSkills: ['investigation', 'insight'],
    armor: '', shield: false, weapons: ['quarterstaff', 'dagger'],
    picks: [pick(2, 'scholar', 'arcana'), pick(3, 'subclass', 'evoker')],
    slotsSpent: [1],
    spells: [
      ...['fire-bolt', 'mage-hand', 'light'].map((s) => learned('wizard', false)(s)),
      ...['magic-missile', 'shield', 'mage-armor', 'detect-magic', 'misty-step', 'sleep'].map((s) => learned('wizard', true)(s, true)),
      ...['burning-hands', 'feather-fall', 'identify', 'comprehend-languages'].map((s) => learned('wizard', true)(s)),
    ],
    appearance: 'Tall and pale, with sea-glass beads braided into silver hair.',
    backstory: 'Liri came to Saltmere to read the drowned chapel\'s records and found the pages still wet.',
  },
  [brannoc]: {
    ...plain,
    species: 'dwarf', class: 'cleric', background: 'acolyte', method: 'standard-array',
    base: { strength: 13, dexterity: 10, constitution: 14, intelligence: 8, wisdom: 15, charisma: 12 },
    bonus: { wisdom: 2, charisma: 1 },
    classSkills: ['medicine', 'persuasion'],
    armor: 'scale-mail', shield: true, weapons: ['mace'],
    picks: [pick(1, 'divine-order', 'protector'), pick(3, 'subclass', 'life')],
    spells: [
      ...['sacred-flame', 'guidance', 'thaumaturgy'].map((s) => learned('cleric', false)(s)),
      ...['healing-word', 'guiding-bolt', 'shield-of-faith', 'command', 'spiritual-weapon', 'prayer-of-healing'].map((s) => learned('cleric', false)(s, true)),
    ],
    appearance: 'Stout, grey-bearded, a brass bell hung from his belt that never rings.',
    backstory: 'Brannoc kept the lighthouse shrine until its keeper vanished into the bay.',
  },
}

/** The build a Character had before an approved retrain changed it: Tamsin first took Acrobatics. */
export function snapshotOf(b: Build): BuildSnapshot {
  return { species: b.species, background: b.background, method: b.method, base: b.base, bonus: b.bonus, increase: {}, skills: b.classSkills, picks: b.picks }
}

const fitsSlot: Record<EquipmentSlot, (category: string, slug: string) => boolean> = {
  head: (c) => c === 'wondrous-item',
  cloak: (c) => c === 'wondrous-item',
  neck: (c) => c === 'wondrous-item',
  hands: (c) => c === 'wondrous-item',
  feet: (c) => c === 'wondrous-item',
  armor: (c, s) => c === 'armor' && s !== 'shield',
  ring_1: (c) => c === 'ring',
  ring_2: (c) => c === 'ring',
  main_hand: (c) => c === 'weapon',
  ranged_main: (c) => c === 'weapon',
  off_hand: (c, s) => c === 'weapon' || s === 'shield',
  ranged_off: (c, s) => c === 'weapon' || s === 'shield',
  ammunition: (c) => c === 'ammunition',
  instrument: (_c, s) => s.startsWith('musical-instrument'),
}

/** Every slot around a Character's figure, in the order the Inventory shows them. */
export const slotOrder = Object.keys(fitsSlot) as EquipmentSlot[]

function item(slug: string, name: string, category: string, weightLb: number, quantity = 1, slot?: EquipmentSlot): ItemCard {
  const fits = slotOrder.filter((s) => fitsSlot[s](category, slug))
  return { slug, name, category, quantity, weightLb, identified: true, attuned: false, fits, ...(slot ? { slot } : {}) }
}

const coins = (gp: number, sp: number, cp: number): LiveCoins[] => [
  { coin: 'gp', count: gp },
  { coin: 'sp', count: sp },
  { coin: 'cp', count: cp },
]

/** What a Character carries: the items worn or held in a slot, the bag and the purse. */
export type Pack = { worn: ItemCard[]; bag: ItemCard[]; coins: LiveCoins[] }

const potion = (n: number) => item('potion-of-healing', 'Potion of Healing', 'potion', 0.5, n)
const rations = (n: number) => item('rations', 'Rations', 'gear', 2, n)

export const packs: Record<string, Pack> = {
  [tamsin]: {
    worn: [
      item('leather-armor', 'Leather Armor', 'armor', 10, 1, 'armor'),
      item('shortsword', 'Shortsword', 'weapon', 2, 1, 'main_hand'),
      item('dagger', 'Dagger', 'weapon', 1, 1, 'off_hand'),
      item('shortbow', 'Shortbow', 'weapon', 2, 1, 'ranged_main'),
      item('arrows', 'Arrows', 'ammunition', 0.05, 20, 'ammunition'),
    ],
    bag: [item('thieves-tools', "Thieves' Tools", 'tool', 1), item('crowbar', 'Crowbar', 'gear', 5), item('rope', 'Rope', 'gear', 5), rations(4), potion(2)],
    coins: coins(14, 22, 31),
  },
  [oswin]: {
    worn: [
      item('chain-mail', 'Chain Mail', 'armor', 55, 1, 'armor'),
      item('longsword', 'Longsword', 'weapon', 3, 1, 'main_hand'),
      item('shield', 'Shield', 'armor', 6, 1, 'off_hand'),
    ],
    bag: [item('javelin', 'Javelin', 'weapon', 2, 4), item('handaxe', 'Handaxe', 'weapon', 2, 2), item('bedroll', 'Bedroll', 'gear', 7), rations(5), potion(1)],
    coins: coins(9, 5, 12),
  },
  [liri]: {
    worn: [item('quarterstaff', 'Quarterstaff', 'weapon', 4, 1, 'main_hand')],
    bag: [item('dagger', 'Dagger', 'weapon', 1), item('spellbook', 'Spellbook', 'gear', 3), item('component-pouch', 'Component Pouch', 'gear', 2), item('ink', 'Ink', 'gear', 0), rations(3)],
    coins: coins(21, 8, 3),
  },
  [brannoc]: {
    worn: [
      item('scale-mail', 'Scale Mail', 'armor', 45, 1, 'armor'),
      item('mace', 'Mace', 'weapon', 4, 1, 'main_hand'),
      item('shield', 'Shield', 'armor', 6, 1, 'off_hand'),
    ],
    bag: [item('holy-symbol', 'Holy Symbol', 'gear', 1), item('healers-kit', "Healer's Kit", 'gear', 3), rations(5), potion(2)],
    coins: coins(11, 12, 7),
  },
}

/** The Party Stash: what the party keeps in common. */
export const stash: Pick<Pack, 'bag' | 'coins'> = {
  bag: [
    item('bag-of-holding', 'Bag of Holding', 'wondrous-item', 15),
    item('musical-instrument-drum', 'Drum', 'tool', 3),
    item('ring-of-protection', 'Ring of Protection', 'ring', 0),
  ],
  coins: [{ coin: 'gp', count: 120 }, { coin: 'pp', count: 2 }],
}
