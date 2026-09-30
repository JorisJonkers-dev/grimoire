import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const CH = '0190c7a8-0000-7000-8000-000000000009'
const member = { id: '0190c7a8-0000-7000-8000-000000000004', displayName: 'Joris', role: 'dm', joinedAt: '2026-09-30T20:00:00Z', isMe: true }
const campaign = {
  id: ID, name: 'Strahd', ruleset: 'srd-2024', myRole: 'dm', memberCount: 1, createdAt: '2026-09-30T20:00:00Z', me: member, members: [member],
}
const options = {
  ruleset: 'srd-2024', rulesetYear: 2024, pointBuyBudget: 27,
  classes: [{ slug: 'fighter', name: 'Fighter', hitDie: 10, saves: ['strength', 'constitution'], skillChoices: 2 }],
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
  skills: [{ skill: 'perception', ability: 'wisdom', bonus: 2, proficient: true }, { skill: 'stealth', ability: 'dexterity', bonus: 2, proficient: false }],
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
  it('walks every step, previews and creates', async () => {
    const bodies: unknown[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/characters/new`, {
      [`/api/v1/campaigns/${ID}/characters/preview`]: async (_u, req) => {
        bodies.push(await req.clone().json())
        return sheet({ id: undefined })
      },
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () => sheet(),
      [`/api/v1/campaigns/${ID}/characters`]: () => sheet(),
      [`/api/v1/campaigns/${ID}`]: () => campaign,
      '/api/v1/compendium/builder': () => options,
    })
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="character-name"]').setValue('Kara')
    await wrapper.get('input[value="human"]').setValue(true)
    await wrapper.get('input[value="soldier"]').setValue(true)
    await expectAccessible(wrapper.element as Element)
    await next(wrapper)
    await wrapper.get('input[value="fighter"]').setValue(true)
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
    expect(wrapper.get('[data-testid="step-review"]').text()).toContain('18')
    expect(bodies[0]).toMatchObject({ name: 'Kara', class: 'fighter', bonus: { strength: 2, constitution: 1 }, armor: 'chain-mail', shield: true })
    await wrapper.get('[data-testid="create-character"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('character') }, { timeout: 5000 })
  })

  it('supports point buy, rolled scores, going back and showing rule errors', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/new`, {
      [`/api/v1/campaigns/${ID}/characters/preview`]: problem(422, 'choose 2 class skills'),
      [`/api/v1/campaigns/${ID}`]: () => campaign,
      '/api/v1/compendium/builder': () => options,
    })
    await wrapper.get('[data-testid="character-name"]').setValue('Kara')
    await wrapper.get('input[value="human"]').setValue(true)
    await wrapper.get('input[value="soldier"]').setValue(true)
    await next(wrapper)
    await wrapper.get('input[value="fighter"]').setValue(true)
    await next(wrapper)
    await wrapper.get('[data-testid="ability-method"]').setValue('point-buy')
    expect(wrapper.get('[data-testid="points-left"]').text()).toContain('27 of 27')
    await wrapper.get('[aria-label="Raise strength"]').trigger('click')
    await wrapper.get('[aria-label="Lower dexterity"]').trigger('click')
    expect(wrapper.get('[data-testid="score-strength"]').text()).toBe('9')
    expect(wrapper.get('[data-testid="score-dexterity"]').text()).toBe('8')
    await wrapper.get('input[value="one-one-one"]').setValue(true)
    for (const [i, a] of ['strength', 'dexterity', 'constitution'].entries()) await wrapper.get(`[data-testid="bonus-${String(i)}"]`).setValue(a)
    await wrapper.get('[data-testid="ability-method"]').setValue('rolled')
    await wrapper.get('input[type="number"]').setValue(17)
    await next(wrapper)
    await wrapper.findAll('[data-testid="step-skills"] input').at(0)?.setValue(true)
    await wrapper.findAll('[data-testid="step-skills"] input').at(1)?.setValue(true)
    await next(wrapper)
    await next(wrapper)
    expect(wrapper.get('[data-testid="builder-error"]').text()).toBe('choose 2 class skills')
    expect(wrapper.get('[data-testid="create-character"]').attributes('disabled')).toBeDefined()
    const back = wrapper.findAll('.nav button').at(0)
    await back?.trigger('click')
    expect(wrapper.find('[data-testid="step-equipment"]').exists()).toBe(true)
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
        return sheet()
      },
    })
    const s = wrapper.get('[data-testid="character-sheet"]')
    expect(s.get('[data-testid="ac"]').text()).toBe('18')
    expect(s.get('[data-testid="hp"]').text()).toBe('12 / 12')
    expect(s.text()).toContain('Chain Mail and a shield')
    expect(s.text()).toContain('150/600 ft')
    expect(s.text()).toContain('Disadvantage on Stealth')
    expect(s.get('[data-testid="resources"]').text()).toContain('Hit Dice (d10): 1 / 1')
    await expectAccessible(wrapper.element as Element)
    await s.get('[data-testid="hp-down"]').trigger('click')
    await s.get('[data-testid="hp-up"]').trigger('click')
    await flushPromises()
    expect(patches).toEqual([{ hpCurrent: 11 }, { hpCurrent: 12 }])
  })

  it('reads effects, locks when not editable and reports failures', async () => {
    const locked = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: () =>
        sheet({ editable: false, armor: undefined, shield: false, effects: [{ name: 'Blessed', detail: '+1d4' }], warnings: [] }),
    })
    expect(locked.wrapper.find('[data-testid="sheet-locked"]').exists()).toBe(true)
    expect(locked.wrapper.find('[data-testid="hp-down"]').exists()).toBe(false)
    expect(locked.wrapper.text()).toContain('No armour')
    expect(locked.wrapper.text()).toContain('Blessed: +1d4')
    document.body.innerHTML = ''
    const failing = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/characters/${CH}`]: (_u, req) => (req.method === 'GET' ? sheet() : problem(409)()),
    })
    await failing.wrapper.get('[data-testid="hp-down"]').trigger('click')
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
