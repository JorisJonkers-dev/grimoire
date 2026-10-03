import type { ItemProperty } from '@/infrastructure/api/types.gen'

/** One field of an Item Property row: what it edits and how. */
export type RowField = { key: keyof ItemProperty; label: string; kind: 'text' | 'number' | 'check' | 'select'; options?: string[] }

const skills = [
  'acrobatics', 'animal-handling', 'arcana', 'athletics', 'deception', 'history', 'insight', 'intimidation', 'investigation',
  'medicine', 'nature', 'perception', 'performance', 'persuasion', 'religion', 'sleight-of-hand', 'stealth', 'survival',
]
const damage = ['acid', 'bludgeoning', 'cold', 'fire', 'force', 'lightning', 'necrotic', 'piercing', 'poison', 'psychic', 'radiant', 'slashing', 'thunder']
const kinds = [
  'weapon', 'armor', 'shield', 'helmet', 'cloak', 'gloves', 'boots', 'amulet', 'ring', 'clothing', 'instrument', 'ammunition',
  'potion', 'scroll', 'throwable', 'coating', 'wand', 'container', 'trinket',
]
const text = (label = 'What it does'): RowField => ({ key: 'text', label, kind: 'text' })

/** The fields each kind of Item Property shows, in order. */
export const rowFields: Record<string, RowField[]> = {
  skill_boost: [
    { key: 'skill', label: 'Skill', kind: 'select', options: skills },
    { key: 'mode', label: 'Boost', kind: 'select', options: ['advantage', 'd4', 'flat', 'proficiency', 'expertise'] },
    { key: 'value', label: 'Flat bonus', kind: 'number' },
  ],
  bonus: [
    { key: 'target', label: 'Bonus to', kind: 'select', options: ['attack', 'damage', 'ac', 'saves', 'spell_attack', 'spell_dc', 'initiative'] },
    { key: 'value', label: 'Bonus', kind: 'number' },
  ],
  resistance: [{ key: 'damage', label: 'Damage type', kind: 'select', options: damage }],
  extra_damage: [{ key: 'dice', label: 'Dice', kind: 'text' }, { key: 'damage', label: 'Damage type', kind: 'select', options: damage }],
  sense: [{ key: 'sense', label: 'Sense', kind: 'select', options: ['darkvision', 'blindsight', 'tremorsense', 'truesight'] }, { key: 'feet', label: 'Feet', kind: 'number' }],
  speed: [{ key: 'speed', label: 'Speed', kind: 'select', options: ['walk', 'fly', 'swim', 'climb', 'burrow'] }, { key: 'feet', label: 'Feet', kind: 'number' }],
  cantrip: [{ key: 'name', label: 'Cantrip', kind: 'text' }, { key: 'spell', label: 'Spell slug', kind: 'text' }],
  spell: [
    { key: 'name', label: 'Spell', kind: 'text' }, { key: 'spell', label: 'Spell slug', kind: 'text' },
    { key: 'level', label: 'Level', kind: 'number' }, { key: 'cost', label: 'Charges (0 at will)', kind: 'number' },
  ],
  light: [{ key: 'brightFt', label: 'Bright (ft)', kind: 'number' }, { key: 'dimFt', label: 'Dim (ft)', kind: 'number' }],
  consumable: [{ key: 'uses', label: 'Uses', kind: 'select', options: ['single', 'long_rest', 'coating'] }, { key: 'hits', label: 'Hits (coating)', kind: 'number' }],
  curse: [text('The curse'), { key: 'cannotDrop', label: "Can't be removed", kind: 'check' }],
  sentient: [text('Personality and goals')],
  growth: [{ key: 'atLevel', label: 'At level', kind: 'number' }, text('What it gains')],
  set_bonus: [{ key: 'set', label: 'Set', kind: 'text' }, { key: 'pieces', label: 'Pieces', kind: 'number' }, text('Bonus')],
  container: [
    { key: 'capacityLb', label: 'Holds (lb)', kind: 'number' }, { key: 'weightless', label: 'Weightless', kind: 'check' },
    { key: 'onlyKind', label: 'Only holds', kind: 'select', options: ['', ...kinds] },
  ],
  firearm: [{ key: 'misfire', label: 'Misfire on', kind: 'number' }, { key: 'reload', label: 'Reload after', kind: 'number' }, { key: 'burst', label: 'Burst', kind: 'number' }],
  trigger: [text('When it triggers, and what happens')],
  manual: [text('What the DM does')],
}

/** Where each new row starts. */
export const freshRow: Record<string, ItemProperty> = {
  skill_boost: { type: 'skill_boost', skill: 'perception', mode: 'advantage' },
  bonus: { type: 'bonus', target: 'ac', value: 1 },
  resistance: { type: 'resistance', damage: 'fire' },
  extra_damage: { type: 'extra_damage', dice: '1d6', damage: 'fire' },
  sense: { type: 'sense', sense: 'darkvision', feet: 60 },
  speed: { type: 'speed', speed: 'fly', feet: 30 },
  cantrip: { type: 'cantrip', name: 'Light', spell: 'light' },
  spell: { type: 'spell', name: 'Shield', spell: 'shield', level: 1, cost: 1 },
  light: { type: 'light', brightFt: 20, dimFt: 20 },
  consumable: { type: 'consumable', uses: 'single' },
  curse: { type: 'curse', text: '' },
  sentient: { type: 'sentient', text: '' },
  growth: { type: 'growth', atLevel: 5, text: '' },
  set_bonus: { type: 'set_bonus', set: '', pieces: 2, text: '' },
  container: { type: 'container', capacityLb: 500, weightless: true },
  firearm: { type: 'firearm', misfire: 1, reload: 6, burst: 0 },
  trigger: { type: 'trigger', text: '' },
  manual: { type: 'manual', text: '' },
}

export const itemKinds = kinds
export const rarities = ['common', 'uncommon', 'rare', 'very_rare', 'legendary'] as const
export const weaponProperties = ['ammunition', 'finesse', 'heavy', 'light', 'loading', 'reach', 'thrown', 'two-handed', 'versatile']
export const masteries = ['', 'cleave', 'graze', 'nick', 'push', 'sap', 'slow', 'topple', 'vex', 'custom']
