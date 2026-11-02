import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Retrain } from '@/infrastructure/api/types.gen'
import {
  zBuilderOptions,
  zCharacterDraft,
  zCharacterSheet,
  zInventoryView,
  zLevelUpPlan,
  zOwnedCharacterList,
  zSpellcasting,
  zTracks,
} from '@/infrastructure/api/zod.gen'
import { mountDemo } from '@/test/mountDemo'
import { createDemo, type Demo } from '../demo'
import { ids } from '../ids'
import type { Persona } from '../role'
import { builds, packs, stash } from '../seeds/characters/party'
import { hitPointsAt, type Build } from '../seeds/characters/sheet'
import { inventoryOf, levelUpPlanOf, retrainChoicesOf, unmetFor } from '../seeds/characters/views'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const c = `/api/v1/campaigns/${ids.campaign}`
const [tamsin, oswin, liri, brannoc] = ids.characters
const sheetOf = (id: string) => `${c}/characters/${id}`

type Answer = { status: number; body: unknown }

/** Asks the demo as an SDK call would, and reads the answer. */
async function ask(demo: Demo, method: string, path: string, body?: unknown): Promise<Answer> {
  const init: RequestInit = body === undefined ? { method } : { method, body: JSON.stringify(body), headers: { 'Content-Type': 'application/json' } }
  const res = await demo.fetch(`https://demo.invalid${path}`, init)
  return { status: res.status, body: res.status === 204 ? undefined : await res.json() }
}

const as = (persona: Persona) => createDemo(persona)
const get = async (persona: Persona, path: string) => ask(as(persona), 'GET', path)

describe('the seeded Characters', () => {
  it('are legal SRD 5.2 third-level sheets with the hit points the core world gives them', async () => {
    const demo = as('dm')
    for (const ch of demo.world.characters) {
      const { status, body } = await ask(demo, 'GET', sheetOf(ch.id))
      expect(status).toBe(200)
      const sheet = zCharacterSheet.parse(body)
      expect(sheet.level).toBe(3)
      expect(sheet.hpMax).toBe(ch.hpMax)
      expect(hitPointsAt(builds[ch.id] as Build, 3)).toBe(ch.hpMax)
      expect(sheet.proficiencyBonus).toBe(2)
      expect(sheet.classes?.[0]?.subclass).toBeTruthy()
    }
  })

  it('derive armour, initiative and attacks by the rules', async () => {
    const demo = as('dm')
    const t = zCharacterSheet.parse((await ask(demo, 'GET', sheetOf(tamsin))).body)
    // Leather 11 + Dexterity 3; Alert adds the Proficiency Bonus to Initiative.
    expect([t.armorClass, t.initiative, t.passivePerception]).toEqual([14, 5, 16])
    expect(t.attacks?.map((a) => [a.name, a.toHit, a.damage, a.mastery])).toEqual([
      ['Shortsword', 5, '1d6+3', 'Vex'],
      ['Dagger', 5, '1d4+3', 'Nick'],
      ['Shortbow', 5, '1d6+3', undefined],
    ])
    const o = zCharacterSheet.parse((await ask(demo, 'GET', sheetOf(oswin))).body)
    // Chain Mail 16, a Shield and the Defense Fighting Style.
    expect([o.armorClass, o.tempHp, o.warnings.length]).toEqual([19, 4, 1])
    const l = zCharacterSheet.parse((await ask(demo, 'GET', sheetOf(liri))).body)
    expect([l.armorClass, l.armor, l.attacks?.[0]?.damage]).toEqual([12, undefined, '1d6-1'])
    expect(l.resources.map((r) => `${r.key} ${String(r.current)}/${String(r.max)}`)).toEqual(['hit-dice 3/3', 'spell-slots-1 3/4', 'spell-slots-2 2/2'])
    const b = zCharacterSheet.parse((await ask(demo, 'GET', sheetOf(brannoc))).body)
    // Scale Mail caps Dexterity at +2; the dwarf's Toughness adds a hit point a level.
    expect([b.armorClass, b.speedFeet]).toEqual([16, 30])
  })
})

describe('a Character sheet as each caller sees it', () => {
  it('is the Player\'s own to edit, and only readable when it is another\'s', async () => {
    const own = zCharacterSheet.parse((await get('player', sheetOf(tamsin))).body)
    expect([own.mine, own.editable, own.ownerName]).toEqual([true, true, 'Ash'])
    const other = zCharacterSheet.parse((await get('player', sheetOf(oswin))).body)
    expect([other.mine, other.editable, other.ownerName]).toEqual([false, false, 'Brook'])
    const asDm = zCharacterSheet.parse((await get('dm', sheetOf(tamsin))).body)
    expect([asDm.mine, asDm.editable]).toEqual([false, true])
  })

  it('is not found for a stranger to the Campaign or an unknown Character', async () => {
    expect((await get('admin', sheetOf(tamsin))).status).toBe(404)
    expect((await get('player', sheetOf(ids.npc))).status).toBe(404)
    expect((await get('signed-out', sheetOf(tamsin))).status).toBe(401)
  })

  it('marks the caller\'s own Character in the party list', async () => {
    const { body } = await get('player', `${c}/characters`)
    expect((body as { name: string; mine: boolean }[]).map((x) => [x.name, x.mine])).toEqual([
      ['Tamsin Reed', true],
      ['Oswin Hale', false],
      ['Liri Vael', false],
      ['Brannoc Stone', false],
    ])
    expect((await get('admin', `${c}/characters`)).status).toBe(404)
  })
})

describe('editing a Character', () => {
  it('lets the owner take damage and heal, within the maximum', async () => {
    const demo = as('player')
    let s = zCharacterSheet.parse((await ask(demo, 'PATCH', sheetOf(tamsin), { damage: 5 })).body)
    expect(s.hpCurrent).toBe(16)
    s = zCharacterSheet.parse((await ask(demo, 'PATCH', sheetOf(tamsin), { heal: 30, name: 'Tamsin of the Wharves' })).body)
    expect([s.hpCurrent, s.name]).toEqual([24, 'Tamsin of the Wharves'])
    s = zCharacterSheet.parse((await ask(demo, 'PATCH', sheetOf(tamsin), { hpCurrent: 3, tempHp: 2 })).body)
    expect([s.hpCurrent, s.tempHp]).toEqual([3, 2])
    expect((await ask(demo, 'PATCH', sheetOf(tamsin), { damage: 50 })).body).toMatchObject({ hpCurrent: 0 })
  })

  it('soaks damage with temporary hit points first', async () => {
    const s = zCharacterSheet.parse((await ask(as('dm'), 'PATCH', sheetOf(oswin), { damage: 6 })).body)
    expect([s.hpCurrent, s.tempHp]).toEqual([29, undefined])
  })

  it('leaves unlocking levels and Heroic Inspiration to the DM', async () => {
    expect((await ask(as('player'), 'PATCH', sheetOf(tamsin), { heroicInspiration: true })).status).toBe(403)
    expect((await ask(as('player'), 'PATCH', sheetOf(tamsin), { levelUpReady: false })).status).toBe(403)
    expect((await ask(as('player'), 'PATCH', sheetOf(oswin), { heal: 1 })).status).toBe(403)
    const demo = as('dm')
    const s = zCharacterSheet.parse((await ask(demo, 'PATCH', sheetOf(liri), { levelUpReady: true, heroicInspiration: true })).body)
    expect([s.levelUpReady, s.heroicInspiration]).toEqual([true, true])
  })
})

describe('the Character draft', () => {
  it('is the first Player\'s own, and the DM has none', async () => {
    const demo = as('player')
    const draft = zCharacterDraft.parse((await ask(demo, 'GET', `${c}/character-draft`)).body)
    expect(draft.build.name).toBe('Wren Ashdown')
    expect((await get('dm', `${c}/character-draft`)).status).toBe(404)
  })

  it('keeps what the wizard saves but never the rolled scores, and starts over', async () => {
    const demo = as('player')
    const mine = demo.world.characterData.drafts[ids.players[0]]
    if (mine) mine.rolled = [15, 14, 13, 12, 10, 8]
    const saved = zCharacterDraft.parse((await ask(demo, 'PUT', `${c}/character-draft`, { step: 5, build: { name: 'Wren' } })).body)
    expect([saved.step, saved.build.name, saved.rolled?.[0]]).toEqual([5, 'Wren', 15])
    expect((await ask(demo, 'DELETE', `${c}/character-draft`)).status).toBe(204)
    expect((await ask(demo, 'GET', `${c}/character-draft`)).status).toBe(404)
    demo.world.persona = 'dm'
    const fresh = zCharacterDraft.parse((await ask(demo, 'PUT', `${c}/character-draft`, { step: 1, build: {} })).body)
    expect(fresh.rolled).toBeUndefined()
  })

  it('offers the SRD to anyone, and a Campaign\'s options only to its Members', async () => {
    const open = zBuilderOptions.parse((await get('signed-out', '/api/v1/compendium/builder?ruleset=srd-2024')).body)
    expect(open.classes).toHaveLength(12)
    expect(open.backgrounds.map((b) => b.slug)).toEqual(['acolyte', 'criminal', 'sage', 'soldier'])
    expect((await get('player', `/api/v1/compendium/builder?ruleset=srd-2024&campaignId=${ids.campaign}`)).status).toBe(200)
    expect((await get('admin', `/api/v1/compendium/builder?ruleset=srd-2024&campaignId=${ids.campaign}`)).status).toBe(404)
  })
})

describe('levelling up', () => {
  const plan = (persona: Persona, id: string, query = '') => get(persona, `${sheetOf(id)}/level-up${query}`)

  it('shows the owner what fourth level in their class offers', async () => {
    const p = zLevelUpPlan.parse((await plan('player', tamsin)).body)
    expect([p.ready, p.level, p.class, p.classLevel, p.average, p.cantrips, p.spells]).toEqual([true, 4, 'rogue', 4, 7, 0, 0])
    expect(p.choices.map((ch) => ch.slug)).toEqual(['ability-score-improvement'])
    expect(p.classes.find((x) => x.slug === 'wizard')?.unmet).toEqual(['Intelligence 13'])
  })

  it('opens a new class at its first level when the multiclass rules allow it', async () => {
    const fighter = zLevelUpPlan.parse((await plan('player', tamsin, '?class=fighter')).body)
    expect([fighter.classLevel, fighter.hitDie, fighter.choices]).toEqual([1, 10, []])
    const monk = zLevelUpPlan.parse((await plan('player', tamsin, '?class=monk')).body)
    expect([monk.cantrips, monk.spells, monk.spellList]).toEqual([0, 0, []])
  })

  it('refuses a class the scores do not allow, or one the ruleset lacks', async () => {
    expect((await plan('player', tamsin, '?class=wizard')).status).toBe(422)
    expect((await plan('player', tamsin, '?class=artificer')).status).toBe(422)
  })

  it('is the owner\'s and the DM\'s alone', async () => {
    expect((await plan('player', liri)).status).toBe(403)
    const wizard = zLevelUpPlan.parse((await plan('dm', liri)).body)
    // A fourth-level wizard learns a cantrip and writes two spells into the book.
    expect([wizard.ready, wizard.cantrips, wizard.spells]).toEqual([false, 1, 2])
    expect(wizard.spellList.map((s) => s.slug)).toContain('web')
    const cleric = zLevelUpPlan.parse((await plan('dm', brannoc)).body)
    expect([cleric.cantrips, cleric.spells]).toEqual([1, 1])
  })
})

describe('spells', () => {
  const spells = (persona: Persona, id: string) => get(persona, `${sheetOf(id)}/spells`)

  it('lists a wizard\'s cantrips, prepared spells and spellbook', async () => {
    const s = zSpellcasting.parse((await spells('dm', liri)).body)
    const [wizard] = s.classes
    expect([wizard?.cantrips.length, wizard?.prepared.length, wizard?.spellbook.length, wizard?.limit, wizard?.allotment]).toEqual([3, 6, 10, 6, 10])
    expect(wizard?.copyable.map((x) => x.slug)).toContain('web')
    expect(wizard?.options.map((x) => x.slug)).not.toContain('web')
  })

  it('keeps a Life cleric\'s domain spells always prepared and out of the choice', async () => {
    const [cleric] = zSpellcasting.parse((await spells('dm', brannoc)).body).classes
    expect(cleric?.always.map((x) => x.slug)).toEqual(['bless', 'cure-wounds', 'aid', 'lesser-restoration'])
    expect(cleric?.options.map((x) => x.slug)).not.toContain('bless')
    expect([cleric?.keepsSpellbook, cleric?.copyable]).toEqual([false, []])
  })

  it('shows a rogue casting nothing, and nobody else\'s spells to a Player', async () => {
    const s = zSpellcasting.parse((await spells('player', tamsin)).body)
    expect([s.classes, s.purse.map((p) => p.coin)]).toEqual([[], ['gp', 'sp', 'cp']])
    expect((await spells('player', liri)).status).toBe(403)
  })

  it('prepares spells within the limit from what the class may prepare', async () => {
    const demo = as('dm')
    const path = `${sheetOf(brannoc)}/spells/prepared`
    const done = zSpellcasting.parse((await ask(demo, 'PUT', path, { class: 'cleric', spells: ['sanctuary', 'healing-word'] })).body)
    expect(done.classes[0]?.prepared.map((x) => x.slug)).toEqual(['healing-word', 'sanctuary'])
    expect(done.classes[0]?.cantrips).toHaveLength(3)
    expect((await ask(demo, 'PUT', path, { class: 'cleric', spells: ['bless'] })).status).toBe(422)
    expect((await ask(demo, 'PUT', path, { class: 'wizard', spells: [] })).status).toBe(422)
    const book = `${sheetOf(liri)}/spells/prepared`
    const wizard = zSpellcasting.parse((await ask(demo, 'PUT', book, { class: 'wizard', spells: ['identify', 'shield'] })).body)
    expect(wizard.classes[0]?.prepared.map((x) => x.slug)).toEqual(['identify', 'shield'])
    expect(wizard.classes[0]?.spellbook).toHaveLength(10)
    const many = ['burning-hands', 'comprehend-languages', 'detect-magic', 'feather-fall', 'identify', 'mage-armor', 'magic-missile']
    expect((await ask(demo, 'PUT', book, { class: 'wizard', spells: many })).status).toBe(422)
  })
})

describe('retraining', () => {
  it('lists the owner\'s picks with what each could be instead, for the owner alone', async () => {
    const { status, body } = await get('player', `${sheetOf(tamsin)}/retrains/choices`)
    expect(status).toBe(200)
    const choices = body as { choice: string; value: string; options: { slug: string }[] }[]
    expect(choices.map((x) => `${x.choice}:${x.value}`)).toEqual(['expertise:stealth', 'expertise:perception', 'subclass:thief'])
    expect(choices[0]?.options.map((o) => o.slug)).toEqual(['acrobatics', 'deception', 'insight', 'perception', 'sleight-of-hand', 'stealth'])
    expect((await get('dm', `${sheetOf(tamsin)}/retrains/choices`)).status).toBe(403)
  })

  it('shows the retrains and Revisions to the owner and the DM, not to another Player', async () => {
    const retrains = (await get('dm', `${sheetOf(tamsin)}/retrains`)).body as Retrain[]
    expect(retrains.map((r) => r.status)).toEqual(['approved'])
    const revisions = (await get('player', `${sheetOf(tamsin)}/revisions`)).body as { build: { picks: { value: string }[] } }[]
    expect(revisions[0]?.build.picks.map((p) => p.value)).toContain('acrobatics')
    expect((await get('dm', `${sheetOf(oswin)}/revisions`)).body).toEqual([])
    expect((await get('player', `${sheetOf(oswin)}/retrains`)).status).toBe(403)
  })

  it('names every kind of pick a class makes', () => {
    const names = (id: string) => retrainChoicesOf(builds[id] as Build).map((x) => `${x.name}:${String(x.options.length)}`)
    expect(names(oswin)).toEqual(['Fighting Style:4', 'Subclass:1'])
    expect(names(liri)).toEqual(['Scholar:6', 'Subclass:1'])
    expect(names(brannoc)).toEqual(['Divine Order:2', 'Subclass:1'])
    const odd = { ...(builds[oswin] as Build), picks: [{ level: 2, choice: 'unknown', value: 'x' }] }
    expect(retrainChoicesOf(odd)[0]?.options).toEqual([])
  })
})

describe('the Inventory', () => {
  it('shows the owner every slot, the bag, the weight and the Party Stash', async () => {
    const inv = zInventoryView.parse((await get('player', `${sheetOf(tamsin)}/inventory`)).body)
    expect(inv.slots).toHaveLength(14)
    expect(inv.slots.find((s) => s.slot === 'armor')?.item?.name).toBe('Leather Armor')
    expect([inv.capacityLb, inv.load, inv.party.map((p) => p.name)]).toEqual([120, 'none', ['Oswin Hale', 'Liri Vael', 'Brannoc Stone']])
    expect(inv.stash.map((i) => i.fits.length > 0)).toEqual([true, true, true])
  })

  it('is the DM\'s to open too, but not another Player\'s', async () => {
    expect(zInventoryView.parse((await get('dm', `${sheetOf(oswin)}/inventory`)).body).slots.find((s) => s.slot === 'off_hand')?.item?.slug).toBe('shield')
    expect((await get('player', `${sheetOf(oswin)}/inventory`)).status).toBe(403)
  })

  it('slows a Character carrying past its capacity and stops one past twice it', () => {
    const b = builds[liri] as Build
    const anvil = (lb: number) => ({ ...(packs[liri] as (typeof packs)[string]), bag: [{ slug: 'anvil', name: 'Anvil', category: 'gear', quantity: 1, weightLb: lb, identified: true, attuned: false, fits: [] }] })
    expect(inventoryOf(liri, 'Liri', b, anvil(150), stash, []).load).toBe('encumbered')
    expect(inventoryOf(liri, 'Liri', b, anvil(300), stash, []).load).toBe('immobile')
  })
})

describe('multiclassing and feats', () => {
  it('needs only one of Strength or Dexterity for a fighter', () => {
    const weak: Build = { ...(builds[liri] as Build), base: { strength: 8, dexterity: 8, constitution: 13, intelligence: 15, wisdom: 12, charisma: 10 } }
    expect(unmetFor(weak, 'fighter')).toEqual(['Strength or Dexterity 13'])
    expect(unmetFor(builds[tamsin] as Build, 'fighter')).toEqual([])
    const fourth = levelUpPlanOf({ ...weak, class: 'cleric' }, 3, 'cleric', false)
    expect(fourth.choices[0]?.options.map((o) => o.unmet)).toEqual([[], ['Strength or Dexterity 13']])
  })

  it('gives a new wizard its first six spells to write into the book', () => {
    const p = levelUpPlanOf(builds[brannoc] as Build, 3, 'wizard', true)
    expect([p.held, p.classLevel, p.cantrips, p.spells, p.spellList.every((s) => s.level <= 1)]).toEqual([true, 1, 3, 6, true])
  })
})

describe('the Characters an Account owns', () => {
  it('lists the Player\'s Characters with their progress in each Campaign', async () => {
    const { items } = zOwnedCharacterList.parse((await get('player', '/api/v1/characters')).body)
    expect(items.map((o) => [o.name, o.campaigns.map((e) => e.campaignName)])).toEqual([
      ['Tamsin Reed', ['The Sunken Bell']],
      ['Wren Ashdown', []],
    ])
    expect(zOwnedCharacterList.parse((await get('dm', '/api/v1/characters')).body).items).toEqual([])
  })

  it('reads only the caller\'s own', async () => {
    expect((await get('player', `/api/v1/characters/${tamsin}`)).status).toBe(200)
    expect((await get('dm', `/api/v1/characters/${tamsin}`)).status).toBe(404)
  })

  it('renames a Character in every Campaign it plays in', async () => {
    const demo = as('player')
    const { status } = await ask(demo, 'PUT', `/api/v1/characters/${tamsin}`, { name: 'Tamsin Saltmere', backstory: 'Still picking locks.' })
    expect(status).toBe(200)
    expect(demo.world.characters[0]?.name).toBe('Tamsin Saltmere')
    expect((await ask(demo, 'GET', sheetOf(tamsin))).body).toMatchObject({ name: 'Tamsin Saltmere' })
    expect((await ask(demo, 'PUT', `/api/v1/characters/${oswin}`, { name: 'Nope', backstory: '' })).status).toBe(404)
  })
})

describe('the Campaign overview', () => {
  it('is listed for its Members only', async () => {
    expect((await get('player', '/api/v1/campaigns')).body).toMatchObject({ items: [{ id: ids.campaign, myRole: 'player' }] })
    expect((await get('admin', '/api/v1/campaigns')).body).toEqual({ items: [] })
  })

  it('lists the open invites for the DM and refuses them to a Player', async () => {
    expect((await get('dm', `${c}/invites`)).body).toMatchObject([{ id: ids.invite, createdBy: 'Morgan' }])
    expect((await get('player', `${c}/invites`)).status).toBe(403)
  })

  it('shows the DM the invites and never asks for them as a Player', async () => {
    const dm = await mountDemo(`/campaigns/${ids.campaign}`, 'dm')
    expect(dm.demo.answers.some((a) => a.what.startsWith('listInvites '))).toBe(true)
    expect(dm.wrapper.text()).toContain('Tamsin Reed')
    const player = await mountDemo(`/campaigns/${ids.campaign}`, 'player')
    expect(player.demo.answers.some((a) => a.what.startsWith('listInvites '))).toBe(false)
  })

  it('opens and revokes an invite link, which then leads nowhere', async () => {
    const demo = as('dm')
    const made = (await ask(demo, 'POST', `${c}/invites`)).body as { id: string; token: string }
    expect(made.token).toMatch(/^[A-Za-z0-9_-]{43}$/)
    demo.world.persona = 'admin'
    expect((await ask(demo, 'POST', '/api/v1/invites/preview', { token: made.token })).body).toEqual({ campaignName: 'The Sunken Bell', invitedBy: 'Morgan' })
    demo.world.persona = 'dm'
    expect((await ask(demo, 'DELETE', `${c}/invites/${made.id}`)).status).toBe(204)
    expect((await ask(demo, 'DELETE', `${c}/invites/${made.id}`)).status).toBe(404)
    expect((await ask(demo, 'POST', '/api/v1/invites/preview', { token: made.token })).status).toBe(404)
  })

  it('shows the seeded invite on the join page to a caller who is not yet a Member', async () => {
    const token = createDemo().world.characterData.inviteTokens[ids.invite] as string
    const { wrapper, demo } = await mountDemo(`/join#${token}`, 'admin')
    expect(demo.answers.filter((a) => a.status >= 400)).toEqual([])
    expect(wrapper.find('[data-testid="join-form"]').text()).toContain('The Sunken Bell')
  })

  it('lists the Sessions newest first, the live one among them', async () => {
    const sessions = (await get('player', `${c}/sessions`)).body as { id: string; number: number; status: string }[]
    expect(sessions.map((s) => [s.number, s.status])).toEqual([[4, 'live'], [3, 'ended'], [2, 'ended'], [1, 'ended']])
    expect(sessions[0]?.id).toBe(ids.session)
    expect((await get('admin', `${c}/sessions`)).status).toBe(404)
  })

  it('keeps the DM\'s notes on Companions from the Players', async () => {
    const dm = (await get('dm', `${c}/companions`)).body as { notes: string }[]
    const player = (await get('player', `${c}/companions`)).body as { notes: string }[]
    expect(dm.every((x) => x.notes !== '')).toBe(true)
    expect(player.map((x) => x.notes)).toEqual(['', ''])
  })
})

describe('Tracks', () => {
  it('give the DM every score and the thresholds', async () => {
    const t = zTracks.parse((await get('dm', `${c}/tracks`)).body)
    expect(t.dm).toBe(true)
    expect(t.tracks.map((x) => [x.name, x.standings.length, x.thresholds?.length])).toEqual([
      ['Renown with the Bellwardens', 1, 2],
      ['Dread', 4, 2],
    ])
  })

  it('give a Player the party\'s score and their own Character\'s, and no threshold', async () => {
    const t = zTracks.parse((await get('player', `${c}/tracks`)).body)
    expect(t.dm).toBe(false)
    expect(t.tracks.map((x) => [x.standings.map((s) => [s.name, s.value]), x.thresholds])).toEqual([
      [[['', 2]], undefined],
      [[['Tamsin Reed', 1]], undefined],
    ])
  })

  it('stand at their start until moved', async () => {
    const demo = as('dm')
    for (const track of demo.world.characterData.tracks) {
      track.values = {}
      Reflect.deleteProperty(track, 'thresholds')
    }
    const t = zTracks.parse((await ask(demo, 'GET', `${c}/tracks`)).body)
    expect(t.tracks.map((x) => [x.thresholds, x.standings.every((s) => s.value === x.start)])).toEqual([[undefined, true], [undefined, true]])
  })
})
