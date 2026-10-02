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
  { slug: 'paladin', name: 'Paladin', hitDie: 10, level: 0, unmet: ['Charisma 13+ (paladin)'] },
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
    expect(wrapper.text()).toContain('Needs Charisma 13+ (paladin)')
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
      [`${base}/level-up`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'multiclassing into Wizard needs Intelligence 13+ (wizard)' }, 422),
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
