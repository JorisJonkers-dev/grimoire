import type {
  Ability,
  AbilityBase,
  AbilityBonus,
  AbilityLine,
  ActiveEffect,
  AttackLine,
  CharacterSheet,
  PickLine,
  ResourcePool,
  SkillLine,
  TraitLine,
} from '@/infrastructure/api/types.gen'
import { abilities, armors, type ClassRules, backgrounds, classes, classRules, fullCasterSlots, skills, species, speciesTraits, weaponLine, weapons } from './srd'

/** A spell a Character learned through a class: a cantrip, one it prepared, or one in its spellbook. */
export type Learned = { slug: string; class: string; prepared: boolean; spellbook: boolean }

/** Everything a seeded Character's sheet is derived from, beyond what the core world keeps of it. */
export type Build = {
  species: string
  class: string
  background: string
  method: CharacterSheet['method']
  base: AbilityBase
  bonus: AbilityBonus
  classSkills: string[]
  armor: string
  shield: boolean
  weapons: string[]
  /** The choices made on reaching a level: Expertise, a subclass, a Fighting Style and the like. */
  picks: PickLine[]
  spells: Learned[]
  xp: number
  levelUpReady: boolean
  heroicInspiration: boolean
  tempHp: number
  hitDiceSpent: number
  slotsSpent: number[]
  effects: ActiveEffect[]
  appearance: string
  backstory: string
}

/** What the core world keeps of a Character: its name, level and hit points. */
export type Core = { id: string; name: string; level: number; hpCurrent: number }

const modifier = (score: number) => Math.floor((score - 10) / 2)
const proficiencyAt = (level: number) => 2 + Math.floor((level - 1) / 4)
/** The value picked for a choice, such as the subclass. */
export const picked = (b: Pick<Build, 'picks'>, choice: string) => b.picks.filter((p) => p.choice === choice).map((p) => p.value)

const named = <T extends { slug: string; name: string }>(list: T[], slug: string): T => list.find((x) => x.slug === slug) as T

function scoresOf(b: Pick<Build, 'base' | 'bonus'>): Record<Ability, number> {
  return Object.fromEntries(abilities.map((a) => [a, b.base[a] + (b.bonus[a] ?? 0)])) as Record<Ability, number>
}

/** Hit points at a level by the average of each Hit Die after the first, as the API takes them. */
export function hitPointsAt(b: Pick<Build, 'base' | 'bonus' | 'class' | 'species'>, level: number): number {
  const die = named(classes, b.class).hitDie
  const con = modifier(scoresOf(b).constitution)
  const toughness = b.species === 'dwarf' ? level : 0
  return die + con + (level - 1) * (die / 2 + 1 + con) + toughness
}

function armorClass(b: Build, dex: number): number {
  const worn = armors.find((a) => a.slug === b.armor)
  const shield = b.shield ? 2 : 0
  if (!worn) return 10 + dex + shield
  const fromDex = worn.addDex ? Math.min(dex, worn.dexCap ?? dex) : 0
  const defense = b.class === 'fighter' ? 1 : 0
  return worn.acBase + fromDex + shield + defense
}

function attack(slug: string, mods: Record<Ability, number>, pb: number, mastered: boolean): AttackLine {
  const w = weapons.find((x) => x.slug === slug) as (typeof weapons)[number]
  const ranged = w.rangeFeet > 0 && !w.thrown
  const mod = w.finesse ? Math.max(mods.strength, mods.dexterity) : ranged ? mods.dexterity : mods.strength
  const sign = mod < 0 ? '-' : '+'
  return {
    name: w.name, toHit: mod + pb, damage: `${w.damageDice}${sign}${String(Math.abs(mod))}`, damageType: w.damageType,
    reachFeet: ranged ? 0 : 5, rangeFeet: w.rangeFeet, longRangeFeet: w.longRangeFeet, ...(mastered ? { mastery: w.mastery } : {}),
  }
}

// Every seeded Character is third level, where each class has chosen its subclass.
const subclassOf = (b: Build) => picked(b, 'subclass')[0] as string

/** Whether a class casts spells, which the demo's tables tell by the cantrips it knows. */
export const casts = (klass: string) => (classRules[klass] as ClassRules).cantrips.length > 0

function resources(b: Build, level: number, die: number): ResourcePool[] {
  const out: ResourcePool[] = [{ key: 'hit-dice', label: `Hit Dice (d${String(die)})`, current: level - b.hitDiceSpent, max: level }]
  const slots: readonly number[] = casts(b.class) ? (fullCasterSlots[level - 1] as readonly number[]) : []
  slots.forEach((n, i) => {
    const at = String(i + 1)
    out.push({ key: `spell-slots-${at}`, label: `Level ${at} spell slots`, current: n - (b.slotsSpent[i] ?? 0), max: n })
  })
  return out
}

/**
 * A Character's sheet with every value derived by the SRD rules, as any Member of its Campaign reads it.
 * What only the caller decides (whether it is theirs, whether they may edit it) is added by the handler.
 */
export function deriveSheet(core: Core, b: Build): Omit<CharacterSheet, 'mine' | 'editable' | 'ownerName'> {
  const klass = named(classes, b.class)
  const rules = classRules[b.class] as (typeof classRules)[string]
  const background = named(backgrounds, b.background)
  const scores = scoresOf(b)
  const mods = Object.fromEntries(abilities.map((a) => [a, modifier(scores[a])])) as Record<Ability, number>
  const pb = proficiencyAt(core.level)
  const lines: AbilityLine[] = abilities.map((a) => {
    const saveProficient = klass.saves.includes(a)
    return { ability: a, score: scores[a], modifier: mods[a], save: mods[a] + (saveProficient ? pb : 0), saveProficient }
  })
  // Expertise is picked at first level by a rogue, and on a wizard's Scholar feature at second.
  const expert = [...picked(b, 'expertise'), ...picked(b, 'scholar')]
  const trained = new Set([...b.classSkills, ...background.skills])
  const skillLines: SkillLine[] = skills.map(({ skill, ability }) => {
    const proficient = trained.has(skill)
    const expertise = expert.includes(skill)
    return { skill, ability, bonus: mods[ability] + (proficient ? pb : 0) + (expertise ? pb : 0), proficient, expertise }
  })
  const perception = skillLines.find((s) => s.skill === 'perception') as SkillLine
  const alert = background.feat === 'Alert'
  const worn = armors.find((a) => a.slug === b.armor)
  return {
    id: core.id,
    characterId: core.id,
    name: core.name,
    ruleset: 'srd-2024',
    level: core.level,
    species: { slug: b.species, name: named(species, b.species).name },
    class: { slug: klass.slug, name: klass.name },
    background: { slug: background.slug, name: background.name },
    method: b.method,
    base: b.base,
    bonus: b.bonus,
    abilities: lines,
    skills: skillLines,
    classSkills: b.classSkills,
    backgroundSkills: background.skills,
    hpCurrent: core.hpCurrent,
    hpMax: hitPointsAt(b, core.level),
    ...(b.tempHp > 0 ? { tempHp: b.tempHp } : {}),
    armorClass: armorClass(b, mods.dexterity),
    initiative: mods.dexterity + (alert ? pb : 0),
    speedFeet: named(species, b.species).speedFeet,
    proficiencyBonus: pb,
    passivePerception: 10 + perception.bonus,
    ...(worn ? { armor: { slug: worn.slug, name: worn.name } } : {}),
    shield: b.shield,
    weapons: b.weapons.map((slug) => weaponLine(weapons.find((w) => w.slug === slug) as (typeof weapons)[number])),
    resources: resources(b, core.level, klass.hitDie),
    effects: b.effects,
    warnings: worn?.stealthDisadvantage ? ['Your armour gives Disadvantage on Stealth checks.'] : [],
    attacks: b.weapons.map((slug, i) => attack(slug, mods, pb, i < rules.masteries)),
    traits: [
      ...(speciesTraits[b.species] as TraitLine[]),
      ...rules.features.filter((f) => f.level <= core.level),
      { name: background.feat, source: 'feat', level: 1, description: `The Origin feat of the ${background.name} background.` },
    ],
    proficiencies: { armor: rules.armor, weapons: rules.weapons },
    levelUpReady: b.levelUpReady,
    heroicInspiration: b.heroicInspiration,
    xp: b.xp,
    increase: {},
    classes: [{ slug: klass.slug, name: klass.name, subclass: subclassOf(b), level: core.level }],
    spells: b.spells.map(({ slug, class: c }) => ({ slug, class: c })),
  }
}
