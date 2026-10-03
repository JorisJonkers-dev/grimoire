import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { sheet } from '@/test/sheet'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const CH = '0190c7a8-0000-7000-8000-000000000009'
const base = `/api/v1/campaigns/${ID}/characters/${CH}`

const classes = [
  { slug: 'fighter', name: 'Fighter', hitDie: 10, level: 3, unmet: [] },
  { slug: 'wizard', name: 'Wizard', hitDie: 6, level: 0, unmet: [] },
  { slug: 'paladin', name: 'Paladin', hitDie: 10, level: 0, unmet: ['Charisma 13+ (Paladin)'] },
]
const fighterPlan = {
  ready: true, held: false, level: 4, classes, class: 'fighter', classLevel: 4, hitDie: 10, average: 8,
  choices: [
    { slug: 'feat', name: 'Ability Score Improvement or feat', pool: 'feat_category', count: 1, options: [
      { slug: 'ability-score-improvement', name: 'Ability Score Improvement', unmet: [] },
      { slug: 'grappler', name: 'Grappler', unmet: ['Strength 13+ or Dexterity 13+'] },
    ] },
    { slug: 'expertise', name: 'Expertise', pool: 'expertise', count: 2, options: [
      { slug: 'athletics', name: 'Athletics', unmet: [] },
      { slug: 'perception', name: 'Perception', unmet: [] },
      { slug: 'survival', name: 'Survival', unmet: [] },
    ] },
  ],
  cantrips: 0, spells: 0, spellList: [],
}
const wizardPlan = {
  ...fighterPlan, level: 4, class: 'wizard', classLevel: 1, hitDie: 6, average: 6, choices: [], cantrips: 1, spells: 1,
  spellList: [
    { slug: 'light', name: 'Light', level: 0 },
    { slug: 'mage-hand', name: 'Mage Hand', level: 0 },
    { slug: 'magic-missile', name: 'Magic Missile', level: 1 },
    { slug: 'shield', name: 'Shield', level: 1 },
  ],
}

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('level-up wizard', () => {
  it('takes a fighter level with an Ability Score Improvement and Expertise', async () => {
    const sent: unknown[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/characters/${CH}/level-up`, {
      [`${base}/level-up`]: async (_u, req) => {
        if (req.method === 'POST') {
          sent.push(await req.clone().json())
          return sheet({ level: 4 })
        }
        return fighterPlan
      },
      [base]: () => sheet({ level: 4 }),
    })
    expect(wrapper.get('[data-testid="level-class-paladin"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Needs Charisma 13+ (Paladin)')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="next"]').trigger('click')
    expect(wrapper.find('[data-testid="step-choices"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="pick-feat-grappler"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="pick-feat-ability-score-improvement"]').setValue(true)
    for (const skill of ['athletics', 'perception', 'survival', 'perception']) {
      await wrapper.get(`[data-testid="pick-expertise-${skill}"]`).trigger('change')
    }
    await wrapper.get('[data-testid="pick-expertise-survival"]').trigger('change')
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="asi-first"]').setValue('strength')
    await wrapper.get('[data-testid="asi-second"]').setValue('constitution')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="next"]').trigger('click')
    await wrapper.get('[data-testid="hp-roll"]').setValue(true)
    await wrapper.get('[data-testid="next"]').trigger('click')
    const review = wrapper.get('[data-testid="step-review"]')
    expect(review.text()).toContain('Fighter 4')
    expect(review.text()).toContain('Athletics, Survival')
    expect(review.text()).toContain('STR +1, CON +1')
    expect(review.text()).toContain('d10 roll')
    await wrapper.get('[data-testid="back"]').trigger('click')
    await wrapper.get('[data-testid="hp-average"]').setValue(true)
    await wrapper.get('[data-testid="next"]').trigger('click')
    expect(wrapper.get('[data-testid="step-review"]').text()).toContain('+8')
    await wrapper.get('[data-testid="take-level"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('character') })
    expect(sent).toEqual([{
      class: 'fighter', hitPoints: 'average', increase: { strength: 1, constitution: 1 }, spells: [],
      picks: [{ choice: 'feat', values: ['ability-score-improvement'] }, { choice: 'expertise', values: ['athletics', 'survival'] }],
    }])
  })

  it('asks for a homebrew subclass\'s own choices once it is picked, and sends only the choices the plan asks for', async () => {
    const sent: unknown[] = []
    const subclass = { slug: 'subclass', name: 'Subclass', pool: 'subclass', count: 1, options: [
      { slug: 'champion', name: 'Champion', unmet: [] }, { slug: 'hb-0190c7a80000', name: 'Lantern Warden', unmet: [] },
    ] }
    const style = { slug: 'hb-0190c7a80000-lantern-style', name: 'Lantern Style', pool: 'listed', count: 1, options: [
      { slug: 'bog-glass', name: 'Bog Glass', unmet: [] }, { slug: 'ember-wick', name: 'Ember Wick', unmet: [] },
    ] }
    const level3 = { ...fighterPlan, level: 3, classLevel: 3, choices: [subclass] }
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/level-up`, {
      [`${base}/level-up`]: async (url, req) => {
        if (req.method === 'POST') {
          sent.push(await req.clone().json())
          return sheet({ level: 3 })
        }
        return url.searchParams.get('subclass') === 'hb-0190c7a80000' ? { ...level3, choices: [subclass, style] } : level3
      },
      [base]: () => sheet({ level: 3 }),
    })
    await wrapper.get('[data-testid="next"]').trigger('click')
    expect(wrapper.find('[data-testid="choice-hb-0190c7a80000-lantern-style"]').exists()).toBe(false)
    await wrapper.get('[data-testid="pick-subclass-hb-0190c7a80000"]').setValue(true)
    await flushPromises()
    expect(wrapper.get('[data-testid="next"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="pick-hb-0190c7a80000-lantern-style-ember-wick"]').setValue(true)
    await wrapper.get('[data-testid="pick-subclass-champion"]').setValue(true)
    await flushPromises()
    expect(wrapper.find('[data-testid="choice-hb-0190c7a80000-lantern-style"]').exists()).toBe(false)
    await wrapper.get('[data-testid="next"]').trigger('click')
    await wrapper.get('[data-testid="next"]').trigger('click')
    await wrapper.get('[data-testid="take-level"]').trigger('click')
    await flushPromises()
    expect(sent).toEqual([{ class: 'fighter', hitPoints: 'average', increase: {}, spells: [], picks: [{ choice: 'subclass', values: ['champion'] }] }])
  })

  it('multiclasses into wizard and learns a cantrip and a spell', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/level-up`, {
      [`${base}/level-up`]: async (url, req) => {
        if (req.method === 'POST') {
          sent.push(await req.clone().json())
          return jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'learn 1 cantrips and 1 spells' }, 422)
        }
        return url.searchParams.get('class') === 'wizard' ? wizardPlan : fighterPlan
      },
    })
    await wrapper.get('[data-testid="level-class-wizard"]').setValue(true)
    await flushPromises()
    expect(wrapper.text()).toContain('level 1 as Wizard')
    await wrapper.get('[data-testid="next"]').trigger('click')
    const spells = wrapper.get('[data-testid="step-spells"]')
    for (const s of ['light', 'mage-hand', 'magic-missile', 'shield', 'light']) {
      await spells.get(`[data-testid="spell-${s}"]`).trigger('change')
    }
    expect(spells.text()).toContain('Cantrips · 0 of 1')
    await spells.get('[data-testid="spell-mage-hand"]').trigger('change')
    expect(spells.text()).toContain('Spells · 1 of 1')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="next"]').trigger('click')
    await wrapper.get('[data-testid="next"]').trigger('click')
    expect(wrapper.get('[data-testid="step-review"]').text()).toContain('Magic Missile, Mage Hand')
    await wrapper.get('[data-testid="take-level"]').trigger('click')
    await flushPromises()
    expect(sent).toEqual([{ class: 'wizard', hitPoints: 'average', increase: {}, picks: [], spells: ['magic-missile', 'mage-hand'] }])
    expect(wrapper.get('[data-testid="level-up-problem"]').text()).toBe('learn 1 cantrips and 1 spells')
  })

  it('explains a locked or held level and a plan that will not load', async () => {
    const held = await mountApp(`/campaigns/${ID}/characters/${CH}/level-up`, {
      [`${base}/level-up`]: () => ({ ...fighterPlan, ready: false, held: true, choices: [] }),
    })
    expect(held.wrapper.get('[data-testid="level-up-locked"]').text()).toContain('holds level-ups')
    await held.wrapper.get('[data-testid="next"]').trigger('click')
    await held.wrapper.get('[data-testid="next"]').trigger('click')
    expect(held.wrapper.get('[data-testid="take-level"]').attributes('disabled')).toBeDefined()
    unmountAll()
    const locked = await mountApp(`/campaigns/${ID}/characters/${CH}/level-up`, {
      [`${base}/level-up`]: () => ({ ...fighterPlan, ready: false }),
    })
    expect(locked.wrapper.get('[data-testid="level-up-locked"]').text()).toContain('after a long rest')
    unmountAll()
    const broken = await mountApp(`/campaigns/${ID}/characters/${CH}/level-up`, {
      [`${base}/level-up`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'multiclassing into Wizard needs Intelligence 13+ (Wizard)' }, 422),
    })
    expect(broken.wrapper.get('[data-testid="level-up-error"]').text()).toContain('Intelligence 13+')
  })
})

describe('sheet level-up controls', () => {
  it('opens the wizard when the level is unlocked and shows every class', async () => {
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`${base}/level-up`]: () => fighterPlan,
      [base]: () => sheet({
        levelUpReady: true, level: 3,
        classes: [{ slug: 'fighter', name: 'Fighter', level: 2, subclass: 'champion' }, { slug: 'wizard', name: 'Wizard', level: 1 }],
      }),
    })
    expect(wrapper.get('[data-testid="sheet-classes"]').text()).toContain('Fighter 2 (Champion) / Wizard 1')
    await wrapper.get('[data-testid="level-up"]').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('level-up') })
  })

  it('lets a DM grant the next level', async () => {
    const patches: unknown[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [base]: async (_u, req) => {
        if (req.method === 'PATCH') patches.push(await req.clone().json())
        return sheet({ mine: false, levelUpReady: false })
      },
    })
    expect(wrapper.find('[data-testid="level-up"]').exists()).toBe(false)
    await wrapper.get('[data-testid="unlock-level"]').trigger('click')
    await flushPromises()
    expect(patches).toEqual([{ levelUpReady: true }])
  })
})

describe('heroic inspiration on the sheet', () => {
  it('lets the DM grant and take it', async () => {
    const patches: unknown[] = []
    let inspired = false
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [base]: async (_u, req) => {
        if (req.method === 'PATCH') {
          const body = (await req.clone().json()) as { heroicInspiration?: boolean }
          patches.push(body)
          inspired = body.heroicInspiration ?? inspired
        }
        return sheet({ mine: false, heroicInspiration: inspired })
      },
    })
    expect(wrapper.get('[data-testid="inspiration"]').text()).toContain('none')
    await wrapper.get('[data-testid="grant-inspiration"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="inspiration"]').text()).toContain('yours to spend')
    await wrapper.get('[data-testid="take-inspiration"]').trigger('click')
    await flushPromises()
    expect(patches).toEqual([{ heroicInspiration: true }, { heroicInspiration: false }])
  })

  it('lets its holder pass it to an ally without it', async () => {
    const passed: unknown[] = []
    const ally = (id: string, name: string, heroicInspiration: boolean) => ({
      id, name, ownerName: 'Tamsin', mine: false, species: 'human', class: 'fighter', level: 1, hpCurrent: 12, hpMax: 12, heroicInspiration,
    })
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`${base}/inspiration/pass`]: async (_u, req) => {
        passed.push(await req.clone().json())
        return sheet({ heroicInspiration: false })
      },
      [base]: () => sheet({ heroicInspiration: true }),
      [`/api/v1/campaigns/${ID}/characters`]: () => [
        ally(CH, 'Kara', true),
        ally('0190c7a8-0000-7000-8000-0000000000a1', 'Ines', false),
        ally('0190c7a8-0000-7000-8000-0000000000a2', 'Bran', true),
      ],
    })
    const select = await vi.waitFor(() => wrapper.get('[data-testid="pass-to"]'))
    expect(select.findAll('option').map((o) => o.text())).toEqual(['Choose an ally', 'Ines'])
    await select.setValue('0190c7a8-0000-7000-8000-0000000000a1')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="pass-inspiration"]').trigger('click')
    await flushPromises()
    expect(passed).toEqual([{ to: '0190c7a8-0000-7000-8000-0000000000a1' }])
  })
})

describe('sheet actions that fail', () => {
  it('reports when granting, passing, unlocking or deciding is refused', async () => {
    const refused = () => jsonResponse({ type: 'about:blank', title: 'Conflict', status: 409 }, 409)
    const RT = '0190c7a8-0000-7000-8000-0000000000c1'
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/retrains/${RT}`]: refused,
      [`${base}/retrains`]: () => [{
        id: RT, characterId: CH, status: 'pending', reason: '', requestedBy: 'Tamsin', createdAt: '2026-10-02T10:00:00Z',
        proposed: { species: 'human', background: 'sage', method: 'standard-array', base: { strength: 15, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 8 }, bonus: {}, increase: {}, skills: [], picks: [] },
      }],
      [base]: (_u, req) => (req.method === 'GET' ? sheet({ mine: false }) : refused()),
    })
    for (const button of ['grant-inspiration', 'unlock-level']) {
      await wrapper.get(`[data-testid="${button}"]`).trigger('click')
      await flushPromises()
      expect(wrapper.text()).toContain('was not saved')
    }
    const panel = await vi.waitFor(() => wrapper.get('[data-testid="retrain-waiting"]'))
    expect(panel.text()).toContain('no level choices')
    await panel.get('[data-testid="approve-retrain"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('was not saved')
    unmountAll()
    const holder = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`${base}/inspiration/pass`]: refused,
      [base]: () => sheet({ heroicInspiration: true }),
      [`/api/v1/campaigns/${ID}/characters`]: () => [{ id: '0190c7a8-0000-7000-8000-0000000000a1', name: 'Ines', ownerName: 'T', mine: false, species: 'human', class: 'fighter', level: 1, hpCurrent: 1, hpMax: 1 }],
    })
    const select = await vi.waitFor(() => holder.wrapper.get('[data-testid="pass-to"]'))
    await select.setValue('0190c7a8-0000-7000-8000-0000000000a1')
    await holder.wrapper.get('[data-testid="pass-inspiration"]').trigger('click')
    await flushPromises()
    expect(holder.wrapper.text()).toContain('was not saved')
    await holder.wrapper.find('form.controls').trigger('submit')
  })
})
