import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const CH = '0190c7a8-0000-7000-8000-000000000009'
const member = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = {
  id: ID, name: 'Morvain', ruleset: 'srd-2024', myRole: 'dm', memberCount: 1, createdAt: '2026-09-30T20:00:00Z', me: member, members: [member],
}
const options = {
  ruleset: 'srd-2024', rulesetYear: 2024, pointBuyBudget: 27,
  classes: [{ slug: 'fighter', name: 'Fighter', hitDie: 10, saves: ['strength', 'constitution'], skillChoices: 2, primaryAbilities: ['strength', 'dexterity'], caster: 'none' }],
  species: [{ slug: 'human', name: 'Human', speedFeet: 30 }],
  backgrounds: [{ slug: 'soldier', name: 'Soldier', abilities: ['strength', 'dexterity', 'constitution'], skills: ['athletics', 'intimidation'] }],
  armor: [
    { slug: 'chain-mail', name: 'Chain Mail', category: 'heavy', shield: false, acBase: 16, addDex: false, strengthRequired: 13, stealthDisadvantage: true },
    { slug: 'shield', name: 'Shield', category: 'shield', shield: true, acBase: 2, addDex: false, strengthRequired: 0, stealthDisadvantage: false },
  ],
  weapons: [{ slug: 'longsword', name: 'Longsword', damageDice: '1d8', damageType: 'slashing', rangeFeet: 0, longRangeFeet: 0 }],
  skills: [
    { skill: 'athletics', ability: 'strength' },
    { skill: 'perception', ability: 'wisdom' },
    { skill: 'survival', ability: 'wisdom' },
    { skill: 'stealth', ability: 'dexterity' },
  ],
}
const abilities = ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'].map((ability, i) => ({
  ability, score: 10 + i, modifier: Math.floor(i / 2), save: Math.floor(i / 2) + (i === 0 ? 2 : 0), saveProficient: i === 0,
}))
const sheet = (extra = {}) => ({
  id: CH, name: 'Kara', ruleset: 'srd-2024', level: 1, ownerName: 'Joris', mine: true, editable: true,
  species: { slug: 'human', name: 'Human' }, class: { slug: 'fighter', name: 'Fighter' }, background: { slug: 'soldier', name: 'Soldier' },
  method: 'standard-array',
  base: { strength: 15, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 8 },
  bonus: { strength: 2, constitution: 1 }, abilities,
  skills: [{ skill: 'perception', ability: 'wisdom', bonus: 2, proficient: true, expertise: false }, { skill: 'stealth', ability: 'dexterity', bonus: 2, proficient: false, expertise: false }],
  classSkills: ['perception', 'survival'], backgroundSkills: ['athletics', 'intimidation'],
  hpCurrent: 12, hpMax: 12, armorClass: 18, initiative: 2, speedFeet: 30, proficiencyBonus: 2, passivePerception: 12,
  armor: { slug: 'chain-mail', name: 'Chain Mail' }, shield: true,
  weapons: [{ slug: 'longbow', name: 'Longbow', damageDice: '1d8', damageType: 'piercing', rangeFeet: 150, longRangeFeet: 600 }],
  resources: [{ key: 'hit-dice', label: 'Hit Dice (d10)', current: 1, max: 1 }],
  effects: [], warnings: ['Your armour gives Disadvantage on Stealth checks.'], ...extra,
})
const problem = (status: number, detail?: string) => () => jsonResponse({ type: 'about:blank', title: 'x', status, detail }, status)

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

async function next(w: VueWrapper) {
  await w.get('[data-testid="next"]').trigger('click')
  await flushPromises()
}

describe('character builder', () => {
  it('walks all nine steps, keeps a draft, previews and creates', async () => {
    const bodies: unknown[] = []
    const drafts: unknown[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/characters/new`, {
      [`/api/v1/campaigns/${ID}/characters/preview`]: async (_u, req) => {
        bodies.push(await req.clone().json())
        return sheet({ id: undefined, level: 3 })
      },
      [`/api/v1/campaigns/${ID}/character-draft`]: async (_u, req) => {
        if (req.method === 'PUT') {
          drafts.push(await req.clone().json())
          return { step: 1, build: {} }
        }
        return problem(404)()
      },
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () => sheet(),
      [`/api/v1/campaigns/${ID}/characters`]: () => sheet(),
      [`/api/v1/campaigns/${ID}`]: () => ({ ...campaign, startingLevel: 3 }),
      '/api/v1/compendium/builder': () => options,
    })
    expect(wrapper.get('[data-testid="starting-level"]').text()).toContain('level 3')
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('input[value="human"]').setValue(true)
    await expectAccessible(wrapper.element as Element)
    await next(wrapper)
    expect(wrapper.get('[data-testid="primary-fighter"]').text()).toBe('Main: STR or DEX')
    await wrapper.get('input[value="fighter"]').setValue(true)
    await next(wrapper)
    await wrapper.get('input[value="soldier"]').setValue(true)
    await next(wrapper)
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="bonus-0"]').setValue('strength')
    await wrapper.get('[data-testid="bonus-1"]').setValue('constitution')
    await next(wrapper)
    expect(wrapper.find('input[value="athletics"]').exists()).toBe(false)
    await wrapper.get('input[value="perception"]').setValue(true)
    await wrapper.get('input[value="survival"]').setValue(true)
    expect(wrapper.get('input[value="stealth"]').attributes('disabled')).toBeDefined()
    await next(wrapper)
    await wrapper.get('[data-testid="armor"]').setValue('chain-mail')
    await wrapper.get('[data-testid="shield"]').setValue(true)
    await wrapper.get('input[value="longsword"]').setValue(true)
    await next(wrapper)
    await wrapper.get('[data-testid="appearance"]').setValue('Tall, scarred')
    await next(wrapper)
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="character-name"]').setValue('Kara')
    await wrapper.get('[data-testid="backstory"]').setValue('Raised in the barracks')
    await next(wrapper)
    expect(wrapper.get('[data-testid="step-review"]').text()).toContain('Level 3')
    expect(bodies[0]).toMatchObject({ name: 'Kara', class: 'fighter', bonus: { strength: 2, constitution: 1 }, armor: 'chain-mail', shield: true, appearance: 'Tall, scarred', backstory: 'Raised in the barracks' })
    expect(drafts).toHaveLength(8)
    expect(drafts[7]).toMatchObject({ step: 8, build: { name: 'Kara', species: 'human', class: 'fighter' } })
    await wrapper.get('[data-testid="create-character"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('character') }, { timeout: 5000 })
  })

  it('offers only the Campaign\'s methods, rolls on the server and places each score once', async () => {
    let rolledOnce = false
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/new`, {
      [`/api/v1/campaigns/${ID}/characters/preview`]: problem(422, 'place the six scores you rolled'),
      [`/api/v1/campaigns/${ID}/character-draft/roll`]: () => {
        rolledOnce = true
        return { step: 3, build: {}, rolled: [16, 15, 12, 11, 9, 8] }
      },
      [`/api/v1/campaigns/${ID}/character-draft`]: (_u, req) => (req.method === 'PUT' ? { step: 1, build: {} } : problem(404)()),
      [`/api/v1/campaigns/${ID}`]: () => ({ ...campaign, creationMethods: ['point-buy', 'rolled'] }),
      '/api/v1/compendium/builder': () => options,
    })
    await wrapper.get('input[value="human"]').setValue(true)
    await next(wrapper)
    await wrapper.get('input[value="fighter"]').setValue(true)
    await next(wrapper)
    await wrapper.get('input[value="soldier"]').setValue(true)
    await next(wrapper)
    const methods = wrapper.get('[data-testid="ability-method"]').findAll('option').map((x) => x.text())
    expect(methods).toEqual(['Point buy', 'Rolled (4d6, drop lowest)'])
    expect(wrapper.get('[data-testid="points-left"]').text()).toContain('27 of 27')
    await wrapper.get('[aria-label="Raise strength"]').trigger('click')
    await wrapper.get('[aria-label="Lower dexterity"]').trigger('click')
    expect(wrapper.get('[data-testid="score-strength"]').text()).toBe('9')
    expect(wrapper.get('[data-testid="score-dexterity"]').text()).toBe('8')
    await wrapper.get('input[value="one-one-one"]').setValue(true)
    for (const [i, a] of ['strength', 'dexterity', 'constitution'].entries()) await wrapper.get(`[data-testid="bonus-${String(i)}"]`).setValue(a)
    await wrapper.get('[data-testid="ability-method"]').setValue('rolled')
    expect(wrapper.get('[data-testid="place-strength"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="roll-scores"]').trigger('click')
    await flushPromises()
    expect(rolledOnce).toBe(true)
    expect(wrapper.get('[data-testid="rolled"]').text()).toContain('16, 15, 12, 11, 9, 8')
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    for (const [i, a] of ['strength', 'dexterity', 'constitution', 'intelligence', 'wisdom', 'charisma'].entries()) {
      await wrapper.get(`[data-testid="place-${a}"]`).setValue(String(i))
    }
    expect(wrapper.get('[data-testid="place-dexterity"] option[value="0"]').attributes('disabled')).toBeDefined()
    await next(wrapper)
    await wrapper.findAll('[data-testid="step-skills"] input').at(0)?.setValue(true)
    await wrapper.findAll('[data-testid="step-skills"] input').at(1)?.setValue(true)
    await next(wrapper)
    await next(wrapper)
    await next(wrapper)
    await wrapper.get('[data-testid="character-name"]').setValue('Ros')
    await next(wrapper)
    expect(wrapper.get('[data-testid="builder-error"]').text()).toBe('place the six scores you rolled')
    expect(wrapper.get('[data-testid="create-character"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="back"]').trigger('click')
    expect(wrapper.find('[data-testid="step-story"]').exists()).toBe(true)
  })

  it('picks up a saved draft and starts over', async () => {
    let discarded = false
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/new`, {
      [`/api/v1/campaigns/${ID}/character-draft`]: (_u, req) => {
        if (req.method === 'DELETE') {
          discarded = true
          return new Response(null, { status: 204 })
        }
        return {
          step: 4,
          build: { name: 'Kara', species: 'human', class: 'fighter', background: 'soldier', method: 'standard-array', base: { strength: 15, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 8 }, bonus: { strength: 2, constitution: 1 }, skills: ['perception'], shield: true, weapons: [], appearance: 'Tall', backstory: 'Old' },
          rolled: [16, 15, 12, 11, 9, 8],
        }
      },
      [`/api/v1/campaigns/${ID}`]: () => campaign,
      '/api/v1/compendium/builder': () => options,
    })
    await flushPromises()
    expect(wrapper.find('[data-testid="step-skills"]').exists()).toBe(true)
    expect((wrapper.get('input[value="perception"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-testid="start-over"]').trigger('click')
    await flushPromises()
    expect(discarded).toBe(true)
    expect(wrapper.find('[data-testid="step-species"]').exists()).toBe(true)
    expect((wrapper.get('input[value="human"]').element as HTMLInputElement).checked).toBe(false)
  })

  it('reports a builder that cannot load', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/new`, { [`/api/v1/campaigns/${ID}`]: problem(404) })
    expect(wrapper.get('[role="alert"]').text()).toContain('could not be loaded')
  })
})

describe('character sheet', () => {
  it('shows the sheet and changes hit points', async () => {
    const patches: unknown[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: async (_u, req) => {
        if (req.method === 'PATCH') patches.push(await req.clone().json())
        return sheet({
          tempHp: 3,
          attacks: [
            { name: 'Longsword', toHit: 5, damage: '1d8+3', damageType: 'slashing', reachFeet: 5, rangeFeet: 0, longRangeFeet: 0, mastery: 'sap' },
            { name: 'Longbow', toHit: 4, damage: '1d8+2', damageType: 'piercing', reachFeet: 0, rangeFeet: 150, longRangeFeet: 600 },
          ],
          traits: [
            { name: 'Second Wind', source: 'class', level: 1, description: 'Regain hit points.' },
            { name: 'Resourceful', source: 'species', level: 0, description: 'Heroic Inspiration.' },
          ],
          proficiencies: { armor: ['Light', 'Heavy'], weapons: [] },
          skills: [{ skill: 'stealth', ability: 'dexterity', bonus: 6, proficient: true, expertise: true }],
        })
      },
    })
    const s = wrapper.get('[data-testid="character-sheet"]')
    expect(s.get('[data-testid="ac"]').text()).toBe('18')
    expect(s.get('[data-testid="hp"]').text()).toBe('12 / 12')
    expect(s.get('[data-testid="temp-hp"]').text()).toBe('+3 temporary')
    expect(s.text()).toContain('Chain Mail and a shield')
    expect(s.get('[data-testid="attacks"]').text()).toContain('Sap mastery')
    expect(s.get('[data-testid="attacks"]').text()).toContain('150/600 ft')
    expect(s.get('[data-testid="attacks"]').text()).toContain('5 ft reach')
    expect(s.findAll('[data-testid="trait"]').map((t) => t.text())).toEqual([
      expect.stringContaining('Fighter 1'),
      expect.stringContaining('Human'),
    ])
    expect(s.get('[data-testid="proficiencies"]').text()).toContain('Light, Heavy')
    expect(s.get('[data-testid="proficiencies"]').text()).toContain('None')
    expect(s.find('[aria-label="Expertise"]').exists()).toBe(true)
    expect(s.text()).toContain('Disadvantage on Stealth')
    expect(s.get('[data-testid="resources"]').text()).toContain('Hit Dice (d10): 1 / 1')
    await expectAccessible(wrapper.element as Element)
    await s.get('[data-testid="part-gear"]').trigger('click')
    expect(s.get('[data-testid="part-gear"]').attributes('aria-pressed')).toBe('true')
    await s.get('[data-testid="hp-amount"]').setValue('4')
    await s.get('[data-testid="hp-damage"]').trigger('click')
    await s.get('[data-testid="hp-heal"]').trigger('click')
    await s.get('[data-testid="hp-temp"]').trigger('click')
    await s.get('[data-testid="hp-amount"]').setValue('0')
    expect(s.get('[data-testid="hp-damage"]').attributes('disabled')).toBeDefined()
    await flushPromises()
    expect(patches).toEqual([{ damage: 4 }, { heal: 4 }, { tempHp: 4 }])
  })

  it('shows where the Character stands on the Campaign\'s Tracks, and the party with it', async () => {
    const OTHER = '0190c7a8-0000-7000-8000-0000000000aa'
    const tracks = {
      dm: false,
      tracks: [
        { id: '0190c7a8-0000-7000-8000-0000000000a1', name: 'Stress', scope: 'character', min: 0, max: 10, start: 1, standings: [{ characterId: OTHER, name: 'Brom', value: 9 }, { characterId: CH, name: 'Kara', value: 5 }] },
        { id: '0190c7a8-0000-7000-8000-0000000000a2', name: 'Renown', scope: 'party', min: -5, max: 5, start: 0, standings: [{ name: '', value: -2 }] },
        { id: '0190c7a8-0000-7000-8000-0000000000a3', name: 'Piety', scope: 'character', min: 0, max: 20, start: 0, standings: [{ characterId: OTHER, name: 'Brom', value: 3 }] },
      ],
    }
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () => sheet(),
      [`/api/v1/campaigns/${ID}/tracks`]: () => tracks,
    })
    await flushPromises()
    // Her own score on a Track kept for each Character, the party's on the party's, and nothing of a Track she has no score on.
    expect(wrapper.findAll('[data-testid="sheet-tracks"] li').map((li) => li.text())).toEqual(['Stress: 5 (0 to 10)', 'Renown, the party\'s: -2 (-5 to 5)'])
    await expectAccessible(wrapper.element as Element)
    // A Campaign with no Tracks shows no such part of the sheet.
    const none = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () => sheet(),
      [`/api/v1/campaigns/${ID}/tracks`]: () => ({ dm: false, tracks: [] }),
    })
    await flushPromises()
    expect(none.wrapper.find('[data-testid="sheet-tracks"]').exists()).toBe(false)
  })

  it('switches between the Campaigns a Character plays in', async () => {
    const OTHER = '0190c7a8-0000-7000-8000-00000000000b'
    const OWNED = '0190c7a8-0000-7000-8000-00000000000c'
    const entry = (campaignId: string, campaignName: string, characterId: string) => ({ campaignId, campaignName, characterId, level: 1, hpCurrent: 12, hpMax: 12 })
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${OTHER}/characters/${CH}`]: () => sheet({ name: 'Kara in Saltmarsh', characterId: OWNED }),
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () => sheet({ characterId: OWNED }),
      [`/api/v1/characters/${OWNED}`]: () => ({
        id: OWNED, name: 'Kara', ruleset: 'srd-2024', species: 'human', class: 'fighter', background: 'soldier', backstory: '',
        hasPortrait: false,
        campaigns: [entry(ID, 'Morvain', CH), entry(OTHER, 'Saltmarsh', CH)],
      }),
    })
    const select = await vi.waitFor(() => wrapper.get('[data-testid="campaign-switch"]'))
    await select.setValue(OTHER)
    await vi.waitFor(() => { expect(router.currentRoute.value.params.id).toBe(OTHER) })
    await flushPromises()
    expect(wrapper.text()).toContain('Kara in Saltmarsh')
  })

  it('reads effects, locks when not editable and reports failures', async () => {
    const locked = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () =>
        sheet({ editable: false, armor: undefined, shield: false, effects: [{ name: 'Blessed', detail: '+1d4' }], warnings: [] }),
    })
    expect(locked.wrapper.find('[data-testid="sheet-locked"]').exists()).toBe(true)
    expect(locked.wrapper.find('[data-testid="hp-damage"]').exists()).toBe(false)
    expect(locked.wrapper.text()).toContain('No weapons carried')
    expect(locked.wrapper.text()).toContain('Nothing yet')
    expect(locked.wrapper.text()).toContain('No armour')
    expect(locked.wrapper.text()).toContain('Blessed: +1d4')
    document.body.innerHTML = ''
    const failing = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: (_u, req) => (req.method === 'GET' ? sheet() : problem(409)()),
    })
    await failing.wrapper.get('[data-testid="hp-heal"]').trigger('click')
    await flushPromises()
    expect(failing.wrapper.text()).toContain('was not saved')
    await failing.wrapper.get('[data-testid="delete-character"]').trigger('click')
    await flushPromises()
    expect(failing.wrapper.text()).toContain('was not saved')
    document.body.innerHTML = ''
    const missing = await mountApp(`/campaigns/${ID}/characters/${CH}`, {})
    expect(missing.wrapper.find('[data-testid="sheet-missing"]').exists()).toBe(true)
  })

  it('deletes and returns to the campaign', async () => {
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: (_u, req) => (req.method === 'DELETE' ? new Response(null, { status: 204 }) : sheet()),
      [`/api/v1/campaigns/${ID}/characters`]: () => [],
      [`/api/v1/campaigns/${ID}/invites`]: () => [],
      [`/api/v1/campaigns/${ID}`]: () => campaign,
    })
    await wrapper.get('[data-testid="delete-character"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('campaign') }, { timeout: 5000 })
    await flushPromises()
    expect(wrapper.get('[data-testid="party"]').text()).toContain('No characters yet')
  })
})

describe('pictures on the sheet', () => {
  it('uploads a portrait and saves each token mode', async () => {
    const writes: string[] = []
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:preview')
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D)
    vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation((cb) => {
      cb(new Blob(['x'], { type: 'image/png' }))
    })
    const base = `/api/v1/campaigns/${ID}/characters/${CH}`
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`${base}/portrait`]: (_u, req) => {
        writes.push(`${req.method} portrait`)
        return new Response(null, { status: 204 })
      },
      [`${base}/token`]: (_u, req) => {
        writes.push(`${req.method} token`)
        return new Response(null, { status: 204 })
      },
      [base]: () => sheet({ portraitUrl: `${base}/portrait?v=abc`, tokenUrl: `${base}/token?v=def` }),
    })
    expect(wrapper.get('[data-testid="portrait"]').attributes('src')).toContain('/portrait?v=abc')
    const file = (type: string, size = 3) => new File([new Uint8Array(size)], 'pic', { type })
    const choose = async (testid: string, f: File) => {
      const input = wrapper.get(`[data-testid="${testid}"]`)
      Object.defineProperty(input.element, 'files', { value: [f], configurable: true })
      await input.trigger('change')
      await flushPromises()
    }
    await choose('portrait-file', file('image/gif'))
    expect(wrapper.get('[data-testid="portrait-problem"]').text()).toContain('PNG, JPEG or WebP')
    await choose('portrait-file', file('image/jpeg'))
    expect(writes).toContain('PUT portrait')
    expect(wrapper.find('[data-testid="portrait-problem"]').exists()).toBe(false)

    const editor = wrapper.get('[data-testid="token-editor"]')
    expect(editor.findAll('[data-testid="token-previews"] figure')).toHaveLength(3)
    await editor.get('input[value="crop"]').setValue(true)
    await editor.get('img.source').trigger('load')
    await flushPromises()
    await editor.get('[data-testid="crop-zoom"]').setValue(2)
    const sliders = editor.findAll('input[type="range"]')
    await sliders[1]?.setValue(0.5)
    await sliders[2]?.setValue(-0.5)
    await flushPromises()
    await editor.get('[data-testid="save-token"]').trigger('click')
    await flushPromises()
    await editor.get('input[value="icon"]').setValue(true)
    await choose('icon-file', file('image/webp'))
    await editor.get('[data-testid="save-token"]').trigger('click')
    await flushPromises()
    await editor.get('input[value="initials"]').setValue(true)
    await editor.get('[data-testid="save-token"]').trigger('click')
    await flushPromises()
    expect(writes.filter((w) => w.endsWith('token'))).toEqual(['PUT token', 'PUT token', 'DELETE token'])
    await expectAccessible(wrapper.element as Element)
  })

  it('reports failed uploads and bad icons', async () => {
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:preview')
    const base = `/api/v1/campaigns/${ID}/characters/${CH}`
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`${base}/portrait`]: problem(503),
      [`${base}/token`]: problem(503),
      [base]: () => sheet(),
    })
    const choose = async (testid: string, f: File | undefined) => {
      const input = wrapper.get(`[data-testid="${testid}"]`)
      Object.defineProperty(input.element, 'files', { value: f ? [f] : [], configurable: true })
      await input.trigger('change')
      await flushPromises()
    }
    await choose('portrait-file', undefined)
    await choose('portrait-file', new File(['x'], 'p', { type: 'image/png' }))
    expect(wrapper.get('[data-testid="portrait-problem"]').text()).toContain('could not be saved')
    const editor = wrapper.get('[data-testid="token-editor"]')
    expect(editor.get('input[value="crop"]').attributes('disabled')).toBeDefined()
    await editor.get('input[value="icon"]').setValue(true)
    await choose('icon-file', undefined)
    await choose('icon-file', new File(['x'], 'i', { type: 'image/gif' }))
    expect(editor.get('[data-testid="token-problem"]').text()).toContain('PNG, JPEG or WebP')
    expect(editor.get('[data-testid="save-token"]').attributes('disabled')).toBeDefined()
    await choose('icon-file', new File(['x'], 'i', { type: 'image/png' }))
    await editor.get('[data-testid="save-token"]').trigger('click')
    await flushPromises()
    expect(editor.get('[data-testid="token-problem"]').text()).toContain('could not be saved')
    wrapper.unmount()
  })
})
