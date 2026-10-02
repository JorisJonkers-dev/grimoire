/** A Character sheet as the API answers it, for tests. */
const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'].map((ability, i) => ({
  ability, score: 10 + i, modifier: Math.floor(i / 2), save: Math.floor(i / 2) + (i === 0 ? 2 : 0), saveProficient: i === 0,
}))
export const sheet = (extra = {}) => ({
  id: '0190c7a8-0000-7000-8000-000000000009', name: 'Kara', ruleset: 'srd-2024', level: 1, ownerName: 'Joris', mine: true, editable: true,
  species: { slug: 'human', name: 'Human' }, class: { slug: 'fighter', name: 'Fighter' }, background: { slug: 'soldier', name: 'Soldier' },
  method: 'standard-array',
  base: { strength: 15, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 8 },
  bonus: { strength: 2, constitution: 1 }, abilities,
  skills: [{ skill: 'perception', ability: 'wisdom', bonus: 2, proficient: true }, { skill: 'stealth', ability: 'dexterity', bonus: 2, proficient: false }],
  classSkills: ['perception', 'survival'], backgroundSkills: ['athletics', 'intimidation'],
  hpCurrent: 12, hpMax: 12, armorClass: 18, initiative: 2, speedFeet: 30, proficiencyBonus: 2, passivePerception: 12,
  armor: { slug: 'chain-mail', name: 'Chain Mail' }, shield: true,
  weapons: [{ slug: 'longbow', name: 'Longbow', damageDice: '1d8', damageType: 'piercing', rangeFeet: 150, longRangeFeet: 600 }],
  resources: [{ key: 'hit-dice', label: 'Hit Dice (d10)', current: 1, max: 1 }],
  effects: [], warnings: ['Your armour gives Disadvantage on Stealth checks.'], ...extra,
})
