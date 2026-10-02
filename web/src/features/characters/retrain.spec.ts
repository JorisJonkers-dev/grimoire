import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import { sheet } from '@/test/sheet'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const CH = '0190c7a8-0000-7000-8000-000000000009'
const RT = '0190c7a8-0000-7000-8000-0000000000c1'
const base = `/api/v1/campaigns/${ID}/characters/${CH}`
const options = {
  ruleset: 'srd-2024', rulesetYear: 2024, pointBuyBudget: 27,
  classes: [{ slug: 'fighter', name: 'Fighter', hitDie: 10, saves: ['strength', 'constitution'], skillChoices: 2 }],
  species: [{ slug: 'human', name: 'Human', speedFeet: 30 }, { slug: 'dwarf', name: 'Dwarf', speedFeet: 30 }],
  backgrounds: [
    { slug: 'soldier', name: 'Soldier', abilities: ['strength', 'dexterity', 'constitution'], skills: ['athletics', 'intimidation'] },
    { slug: 'sage', name: 'Sage', abilities: ['constitution', 'intelligence', 'wisdom'], skills: ['arcana', 'history'] },
  ],
  armor: [], weapons: [],
  skills: [{ skill: 'perception', ability: 'wisdom' }, { skill: 'survival', ability: 'wisdom' }, { skill: 'stealth', ability: 'dexterity' }],
}
const build = {
  species: 'human', background: 'sage', method: 'standard-array',
  base: { strength: 15, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 8 },
  bonus: { constitution: 2, intelligence: 1 }, increase: {}, skills: ['perception', 'stealth'],
  picks: [{ level: 4, choice: 'feat', value: 'grappler' }],
}
const retrain = (extra = {}) => ({
  id: RT, characterId: CH, status: 'pending', reason: 'Grappling suits her', requestedBy: 'Tamsin', createdAt: '2026-10-02T10:00:00Z', proposed: build, ...extra,
})
const choices = [{
  level: 4, choice: 'feat', name: 'Ability Score Improvement or feat', value: 'ability-score-improvement',
  options: [
    { slug: 'ability-score-improvement', name: 'Ability Score Improvement', unmet: [] },
    { slug: 'grappler', name: 'Grappler', unmet: [] },
    { slug: 'great-weapon-master', name: 'Great Weapon Master', unmet: ['Level 4+'] },
  ],
}]

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('retrain page', () => {
  it('rebuilds the choices and asks the DM', async () => {
    const sent: unknown[] = []
    let requested = false
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/retrain`, {
      [`${base}/retrains/choices`]: () => choices,
      [`${base}/retrains`]: async (_u, req) => {
        if (req.method === 'POST') {
          sent.push(await req.clone().json())
          requested = true
          return retrain()
        }
        return requested ? [retrain()] : [retrain({ status: 'declined', reason: '' })]
      },
      [`${base}/revisions`]: () => [{ no: 1, author: 'Joris', createdAt: '2026-10-01T10:00:00Z', build: { ...build, background: 'soldier' } }],
      [base]: () => sheet({ increase: { strength: 2 } }),
      ['/api/v1/compendium/builder']: () => options,
    })
    expect(wrapper.get('[data-testid="retrain-history"]').text()).toContain('Declined')
    expect(wrapper.get('[data-testid="revisions"]').text()).toContain('Revision 1 · Human Soldier · kept by Joris')
    expect((wrapper.get('[data-testid="increase-strength"]').element as HTMLInputElement).value).toBe('2')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="retrain-species"]').setValue('dwarf')
    await wrapper.get('[data-testid="retrain-species"]').setValue('human')
    await wrapper.get('[data-testid="base-strength"]').setValue(15)
    await wrapper.get('[data-testid="retrain-background"]').setValue('sage')
    await wrapper.get('[data-testid="bonus-strength"]').setValue(0)
    await wrapper.get('[data-testid="bonus-constitution"]').setValue(2)
    await wrapper.get('[data-testid="bonus-intelligence"]').setValue(1)
    await wrapper.get('[data-testid="increase-strength"]').setValue(0)
    await wrapper.get('[data-testid="retrain-skill-survival"]').trigger('change')
    await wrapper.get('[data-testid="retrain-skill-stealth"]').trigger('change')
    await wrapper.get('[data-testid="retrain-pick-4-feat"]').setValue('grappler')
    expect(wrapper.get('[data-testid="retrain-pick-4-feat"] option[value="great-weapon-master"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="retrain-reason"]').setValue('Grappling suits her')
    await wrapper.get('[data-testid="retrain-form"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{
      reason: 'Grappling suits her',
      build: {
        species: 'human', background: 'sage', method: 'standard-array',
        base: { strength: 15, dexterity: 14, constitution: 13, intelligence: 12, wisdom: 10, charisma: 8 },
        bonus: { constitution: 2, intelligence: 1 }, increase: {}, skills: ['perception', 'stealth'],
        picks: [{ level: 4, choice: 'feat', value: 'grappler' }],
      },
    }])
    await vi.waitFor(() => { expect(wrapper.find('[data-testid="retrain-pending"]').exists()).toBe(true) })
  })

  it('reports a refused rebuild and a Character that is not yours', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}/retrain`, {
      [`${base}/retrains/choices`]: () => [],
      [`${base}/retrains`]: (_u, req) =>
        req.method === 'POST' ? jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'choose 2 class skills' }, 422) : [],
      [`${base}/revisions`]: () => [],
      [base]: () => sheet(),
      ['/api/v1/compendium/builder']: () => options,
    })
    expect(wrapper.text()).toContain('No retrains yet.')
    expect(wrapper.text()).toContain('No earlier builds.')
    await wrapper.get('[data-testid="retrain-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="retrain-problem"]').text()).toBe('choose 2 class skills')
    unmountAll()
    const other = await mountApp(`/campaigns/${ID}/characters/${CH}/retrain`, {
      [`${base}/retrains/choices`]: () => jsonResponse({ type: 'about:blank', title: 'Forbidden', status: 403 }, 403),
      [base]: () => sheet({ mine: false }),
    })
    expect(other.wrapper.find('[data-testid="retrain-error"]').exists()).toBe(true)
  })
})

describe('retrain requests on the sheet', () => {
  it('lets the DM approve or decline a waiting retrain', async () => {
    const decided: string[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/characters/${CH}`, {
      [`/api/v1/campaigns/${ID}/retrains/${RT}`]: (u) => {
        decided.push(u.pathname.split('/').at(-1) ?? '')
        return retrain({ status: 'approved', decidedBy: 'Joris', decidedAt: '2026-10-02T11:00:00Z' })
      },
      [`${base}/retrains`]: () => [retrain()],
      [base]: () => sheet({ mine: false }),
    })
    const panel = await vi.waitFor(() => wrapper.get('[data-testid="retrain-waiting"]'))
    expect(panel.text()).toContain('Tamsin asks to retrain Kara: Grappling suits her.')
    expect(panel.text()).toContain('Human Sage, Grappler')
    await expectAccessible(wrapper.element as Element)
    await panel.get('[data-testid="approve-retrain"]').trigger('click')
    await panel.get('[data-testid="decline-retrain"]').trigger('click')
    await flushPromises()
    expect(decided).toEqual(['approve', 'decline'])
  })
})
