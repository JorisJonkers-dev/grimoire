import type { CharacterSheet, OwnedCharacter } from '@/infrastructure/api/types.gen'
import { DemoProblem, forbidden, notFound, type Handlers } from '../operations'
import type { Viewer } from '../role'
import type { Build } from '../seeds/characters/sheet'
import { deriveSheet } from '../seeds/characters/sheet'
import { armors, backgrounds, classes, skills, species, weaponLine, weapons } from '../seeds/characters/srd'
import type { Pack } from '../seeds/characters/party'
import { inventoryOf, levelUpPlanOf, retrainChoicesOf, spellcastingOf, unmetFor } from '../seeds/characters/views'
import type { DemoCharacter, World } from '../world'
import { member, signedIn } from './access'

// The Campaign's Characters as each caller may see them. Every Member reads every sheet, as the API
// lets them; only the owner and the DM may change one, and some pages (retraining, level-up, spells,
// the Inventory) are for those two alone.

function characterOf(world: World, characterId: string): DemoCharacter {
  const found = world.characters.find((c) => c.id === characterId)
  if (!found) throw notFound('Character')
  return found
}

const buildOf = (world: World, characterId: string) => world.characterData.builds[characterId] as Build
const packOf = (world: World, characterId: string) => world.characterData.packs[characterId] as Pack
// Every seeded Character belongs to a Member of the Campaign.
const ownerName = (world: World, c: DemoCharacter) => (world.members.find((m) => m.accountId === c.ownerId) as World['members'][number]).displayName

/** The sheet a Member reads: theirs when they own it, editable by its owner and the DM. */
function sheetFor(who: Viewer, world: World, campaignId: string, characterId: string): CharacterSheet {
  const role = member(who, campaignId, world)
  const c = characterOf(world, characterId)
  const mine = c.ownerId === signedIn(who).id
  return { ...deriveSheet(c, buildOf(world, c.id)), ownerName: ownerName(world, c), mine, editable: mine || role === 'dm' }
}

/** The sheet, for a page only its owner and the DM may open. */
function editable(who: Viewer, world: World, campaignId: string, characterId: string): CharacterSheet {
  const sheet = sheetFor(who, world, campaignId, characterId)
  if (!sheet.editable) throw forbidden()
  return sheet
}

/** One of the caller's own Characters with its progress in each Campaign; anyone else's is not found. */
function ownedFor(who: Viewer, world: World, characterId: string): OwnedCharacter {
  const me = signedIn(who)
  const o = world.characterData.owned.find((x) => x.id === characterId && x.ownerId === me.id)
  if (!o) throw notFound('Character')
  const playing = world.characters.filter((c) => c.id === o.id)
  return {
    ...o, ruleset: 'srd-2024', hasPortrait: false,
    campaigns: playing.map((c) => ({ campaignId: world.campaign.id, campaignName: world.campaign.name, characterId: c.id, level: c.level, hpCurrent: c.hpCurrent, hpMax: c.hpMax })),
  }
}

export const charactersHandlers = {
  listCharacters: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    const me = signedIn(who).id
    return world.characters.map((c) => {
      const b = buildOf(world, c.id)
      return {
        id: c.id, name: c.name, ownerName: ownerName(world, c), mine: c.ownerId === me,
        species: c.species, class: c.class, level: c.level, hpCurrent: c.hpCurrent, hpMax: c.hpMax, heroicInspiration: b.heroicInspiration,
      }
    })
  },
  getCharacter: ({ who, world, path }) => sheetFor(who, world, path.campaignId, path.characterId),
  updateCharacter: ({ who, world, path, body }) => {
    const role = member(who, path.campaignId, world)
    const sheet = editable(who, world, path.campaignId, path.characterId)
    // Unlocking a level and granting Heroic Inspiration are the DM's to give.
    if ((body.levelUpReady !== undefined || body.heroicInspiration !== undefined) && role !== 'dm') throw forbidden()
    const c = characterOf(world, path.characterId)
    const b = buildOf(world, c.id)
    let hp = body.hpCurrent ?? c.hpCurrent
    let temp = body.tempHp === undefined ? b.tempHp : Math.max(b.tempHp, body.tempHp)
    if (body.damage) {
      const soaked = Math.min(temp, body.damage)
      temp -= soaked
      hp -= body.damage - soaked
    }
    hp += body.heal ?? 0
    c.hpCurrent = Math.max(0, Math.min(sheet.hpMax, hp))
    b.tempHp = temp
    if (body.name) c.name = body.name
    if (body.levelUpReady !== undefined) b.levelUpReady = body.levelUpReady
    if (body.heroicInspiration !== undefined) b.heroicInspiration = body.heroicInspiration
    return sheetFor(who, world, path.campaignId, path.characterId)
  },

  getCharacterDraft: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    const draft = world.characterData.drafts[signedIn(who).id]
    if (!draft) throw new DemoProblem(404, 'No draft')
    return draft
  },
  saveCharacterDraft: ({ who, world, path, body }) => {
    member(who, path.campaignId, world)
    const me = signedIn(who).id
    const rolled = world.characterData.drafts[me]?.rolled
    world.characterData.drafts[me] = { ...body, ...(rolled ? { rolled } : {}) }
    return world.characterData.drafts[me]
  },
  discardCharacterDraft: ({ who, world, path }) => {
    member(who, path.campaignId, world)
    Reflect.deleteProperty(world.characterData.drafts, signedIn(who).id)
    return undefined
  },
  getBuilderOptions: ({ who, world, query }) => {
    // Asked for a Campaign's options, which add its homebrew, the caller must belong to it.
    if (query.campaignId) member(who, query.campaignId, world)
    return { ruleset: query.ruleset, rulesetYear: 2024, pointBuyBudget: 27, classes, species, backgrounds: backgrounds.map(({ slug, name, abilities, skills: trained }) => ({ slug, name, abilities, skills: trained })), armor: armors, weapons: weapons.map(weaponLine), skills }
  },

  planLevelUp: ({ who, world, path, query }) => {
    const sheet = editable(who, world, path.campaignId, path.characterId)
    const b = buildOf(world, path.characterId)
    // A page that names no class sends no query string at all, so the query may be missing.
    const klass = query?.class ?? sheet.class.slug
    const offer = classes.find((c) => c.slug === klass)
    if (!offer) throw new DemoProblem(422, 'Choose a class from this ruleset')
    const unmet = klass === b.class ? [] : unmetFor(b, klass)
    if (unmet.length > 0) throw new DemoProblem(422, `Multiclassing into ${offer.name} needs ${unmet.join(', ')}`)
    return levelUpPlanOf(b, sheet.level, klass, false)
  },
  getSpellcasting: ({ who, world, path }) => {
    const sheet = editable(who, world, path.campaignId, path.characterId)
    return spellcastingOf(buildOf(world, path.characterId), sheet.level, packOf(world, path.characterId).coins, world.characterData.clock)
  },
  prepareSpells: ({ who, world, path, body }) => {
    const sheet = editable(who, world, path.campaignId, path.characterId)
    const b = buildOf(world, path.characterId)
    const now = spellcastingOf(b, sheet.level, [], world.characterData.clock).classes.find((c) => c.class === body.class)
    if (!now) throw new DemoProblem(422, 'This Character prepares no spells through that class')
    const allowed = new Set(now.options.map((s) => s.slug))
    if (body.spells.length > now.limit || body.spells.some((s) => !allowed.has(s))) throw new DemoProblem(422, `Prepare up to ${String(now.limit)} spells this class may prepare`)
    const kept = b.spells.filter((s) => s.class !== body.class || s.spellbook || !s.prepared)
    for (const s of kept) if (s.class === body.class && s.spellbook) s.prepared = body.spells.includes(s.slug)
    for (const slug of body.spells) {
      if (!kept.some((s) => s.slug === slug && s.class === body.class)) kept.push({ slug, class: body.class, prepared: true, spellbook: false })
    }
    b.spells = kept
    return spellcastingOf(b, sheet.level, packOf(world, path.characterId).coins, world.characterData.clock)
  },

  listRetrainChoices: ({ who, world, path }) => {
    const sheet = sheetFor(who, world, path.campaignId, path.characterId)
    if (!sheet.mine) throw forbidden()
    return retrainChoicesOf(buildOf(world, path.characterId))
  },
  listRetrains: ({ who, world, path }) => {
    editable(who, world, path.campaignId, path.characterId)
    return world.characterData.retrains.filter((r) => r.characterId === path.characterId)
  },
  listCharacterRevisions: ({ who, world, path }) => {
    editable(who, world, path.campaignId, path.characterId)
    return world.characterData.revisions[path.characterId] ?? []
  },

  getInventory: ({ who, world, path }) => {
    const sheet = editable(who, world, path.campaignId, path.characterId)
    const party = world.characters.filter((c) => c.id !== path.characterId).map((c) => ({ characterId: c.id, name: c.name }))
    return inventoryOf(path.characterId, sheet.name, buildOf(world, path.characterId), packOf(world, path.characterId), world.characterData.stash, party)
  },

  listMyCharacters: ({ who, world }) => {
    const me = signedIn(who).id
    return { items: world.characterData.owned.filter((o) => o.ownerId === me).map((o) => ownedFor(who, world, o.id)) }
  },
  getMyCharacter: ({ who, world, path }) => ownedFor(who, world, path.characterId),
  updateMyCharacter: ({ who, world, path, body }) => {
    ownedFor(who, world, path.characterId)
    const o = world.characterData.owned.find((x) => x.id === path.characterId) as World['characterData']['owned'][number]
    o.name = body.name
    o.backstory = body.backstory
    // The name flows to every Campaign the Character plays in.
    for (const c of world.characters.filter((x) => x.id === o.id)) c.name = body.name
    return ownedFor(who, world, path.characterId)
  },
} satisfies Handlers
