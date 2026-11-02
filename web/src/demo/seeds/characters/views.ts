import type {
  Ability,
  ClassSpells,
  GameClock,
  InventoryView,
  ItemCard,
  LevelUpChoice,
  LevelUpOption,
  LevelUpPlan,
  LiveCoins,
  PartyBearer,
  RetrainChoice,
  SpellPick,
  Spellcasting,
} from '@/infrastructure/api/types.gen'
import { slotOrder, type Pack } from './party'
import { casts, picked, type Build } from './sheet'
import { abilities, alwaysPrepared, backgrounds, classes, classRules, skills, spellLists, type ClassRules } from './srd'

// What the Character pages read beyond the sheet, derived from a build the way the API derives them.

const title = (slug: string) => slug.split('-').map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' ')
const option = (slug: string, unmet: string[] = []): LevelUpOption => ({ slug, name: title(slug), unmet })
const scoresOf = (b: Build) => Object.fromEntries(abilities.map((a) => [a, b.base[a] + (b.bonus[a] ?? 0)])) as Record<Ability, number>
// A full caster reaches 2nd-level spells at its third level.
const maxSpellLevel = (classLevel: number) => (classLevel >= 3 ? 2 : 1)
const rulesOf = (klass: string): ClassRules | undefined => classRules[klass]

/** A Character's spells through its class: cantrips, prepared, always prepared, a wizard's spellbook. */
export function spellcastingOf(b: Build, level: number, purse: LiveCoins[], clock: GameClock): Spellcasting {
  const out: ClassSpells[] = []
  if (casts(b.class)) {
    const rules = classRules[b.class] as ClassRules
    const maxLevel = maxSpellLevel(level)
    const list = (spellLists[b.class] as SpellPick[]).filter((s) => s.level <= maxLevel)
    const always = alwaysPrepared[picked(b, 'subclass')[0] as string] ?? []
    const mine = (slug: string) => b.spells.find((s) => s.slug === slug)
    const cs: ClassSpells = {
      class: b.class, name: title(b.class), level, limit: rules.prepared[level - 1] as number, maxLevel, keepsSpellbook: rules.spellbook,
      allotment: rules.spellbook ? 6 + 2 * (level - 1) : 0,
      cantrips: list.filter((s) => s.level === 0 && mine(s.slug)),
      prepared: list.filter((s) => s.level > 0 && mine(s.slug)?.prepared),
      always: list.filter((s) => always.includes(s.slug)),
      spellbook: list.filter((s) => mine(s.slug)?.spellbook),
      // A wizard prepares from its spellbook, any other caster from its whole list.
      options: list.filter((s) => s.level > 0 && !always.includes(s.slug) && (!rules.spellbook || mine(s.slug)?.spellbook)),
      copyable: rules.spellbook ? list.filter((s) => s.level > 0 && !mine(s.slug)?.spellbook) : [],
    }
    out.push(cs)
  }
  return { canPrepare: true, classes: out, purse, clock }
}

/** What multiclassing into a class needs that a Character's scores do not have. */
export function unmetFor(b: Build, klass: string): string[] {
  const scores = scoresOf(b)
  const needs = (classes.find((c) => c.slug === klass) as (typeof classes)[number]).primaryAbilities as Ability[]
  if (klass === 'fighter') return needs.some((a) => scores[a] >= 13) ? [] : ['Strength or Dexterity 13']
  return needs.filter((a) => scores[a] < 13).map((a) => `${title(a)} 13`)
}

/** What the next level offers in a class: the Character's own class, or a new one it multiclasses into. */
export function levelUpPlanOf(b: Build, level: number, klass: string, held: boolean): LevelUpPlan {
  const target = classes.find((c) => c.slug === klass) as (typeof classes)[number]
  const has = klass === b.class ? level : 0
  const classLevel = has + 1
  const rules = rulesOf(klass)
  const con = Math.floor((scoresOf(b).constitution - 10) / 2)
  // A new class grants its whole first level; another level adds the difference of its tables.
  const gain = (table: number[] | undefined) => (table?.[classLevel - 1] ?? 0) - (table?.[has - 1] ?? 0)
  const cantrips = gain(rules?.cantrips)
  const spells = rules?.spellbook ? (has === 0 ? 6 : 2) : gain(rules?.prepared)
  const known = new Set(b.spells.filter((s) => s.class === klass).map((s) => s.slug))
  const spellList = (spellLists[klass] ?? []).filter(
    (s) => s.level <= maxSpellLevel(classLevel) && !known.has(s.slug) && (s.level === 0 ? cantrips > 0 : spells > 0),
  )
  const scores = scoresOf(b)
  const feats: LevelUpChoice = {
    slug: 'ability-score-improvement', name: 'Ability Score Improvement', pool: 'feat_category', count: 1,
    options: [option('ability-score-improvement'), option('grappler', scores.strength >= 13 || scores.dexterity >= 13 ? [] : ['Strength or Dexterity 13'])],
  }
  return {
    ready: b.levelUpReady, held, level: level + 1, class: klass, classLevel, hitDie: target.hitDie,
    average: target.hitDie / 2 + 1 + con,
    classes: classes.map((c) => ({ slug: c.slug, name: c.name, hitDie: c.hitDie, level: c.slug === b.class ? level : 0, unmet: c.slug === b.class ? [] : unmetFor(b, c.slug) })),
    choices: classLevel === 4 ? [feats] : [],
    cantrips, spells, spellList,
  }
}

/** The options each kind of pick could have taken instead. */
function optionsFor(b: Build, choice: string): LevelUpOption[] {
  switch (choice) {
    case 'expertise': {
      const trained = [...b.classSkills, ...(backgrounds.find((x) => x.slug === b.background) as (typeof backgrounds)[number]).skills]
      return skills.filter((s) => trained.includes(s.skill)).map((s) => option(s.skill))
    }
    case 'scholar':
      return ['arcana', 'history', 'investigation', 'medicine', 'nature', 'religion'].map((s) => option(s))
    case 'fighting-style':
      return ['archery', 'defense', 'great-weapon-fighting', 'two-weapon-fighting'].map((s) => option(s))
    case 'divine-order':
      return ['protector', 'thaumaturge'].map((s) => option(s))
    case 'subclass':
      // The SRD gives each class one subclass, so the only option is the one taken.
      return picked(b, 'subclass').map((s) => option(s))
  }
  return []
}

const choiceNames: Record<string, string> = { expertise: 'Expertise', scholar: 'Scholar', 'fighting-style': 'Fighting Style', 'divine-order': 'Divine Order', subclass: 'Subclass' }

/** Each pick a Character made on a level, with what it could take instead. */
export function retrainChoicesOf(b: Build): RetrainChoice[] {
  return b.picks.map((p) => ({ level: p.level, choice: p.choice, name: choiceNames[p.choice] as string, value: p.value, options: optionsFor(b, p.choice) }))
}

const weightOf = (items: ItemCard[], coins: LiveCoins[]) =>
  items.reduce((sum, i) => sum + i.weightLb * i.quantity, 0) + coins.reduce((sum, c) => sum + c.count, 0) / 50

/** A Character's Inventory: its slots, bag, weight against capacity and the Party Stash. */
export function inventoryOf(characterId: string, name: string, b: Build, pack: Pack, stash: Pick<Pack, 'bag' | 'coins'>, party: PartyBearer[]): InventoryView {
  const weightLb = Math.round(weightOf([...pack.worn, ...pack.bag], pack.coins) * 100) / 100
  const capacityLb = scoresOf(b).strength * 15
  const load = weightLb > 2 * capacityLb ? 'immobile' : weightLb > capacityLb ? 'encumbered' : 'none'
  return {
    characterId, name,
    slots: slotOrder.map((slot) => {
      const item = pack.worn.find((i) => i.slot === slot)
      return item ? { slot, item } : { slot }
    }),
    bag: pack.bag, coins: pack.coins, weightLb, capacityLb, load,
    stash: stash.bag, stashCoins: stash.coins, party, weaponSet: 'melee',
  }
}
