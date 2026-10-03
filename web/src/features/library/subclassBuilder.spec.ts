import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { SubclassBuild, SubclassDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const SUB = '0190c7a8-0000-7000-8000-0000000000b3'
const at = '2026-10-03T20:00:00Z'
const start: SubclassDesign = { class: 'fighter', features: [{ level: 3, name: 'First feature', text: '' }], resources: [], choices: [] }
const entry = { id: SUB, kind: 'subclass' as const, name: 'Lantern Warden', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: SubclassDesign, extra: Partial<SubclassBuild> = {}): SubclassBuild => ({
  entry, design, slug: 'hb-0190c7a80000', lines: ['Fighter subclass', 'Level 3: First feature. '], ...extra,
})

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('subclass builder', () => {
  it('builds the Lantern Warden from features, Resources and a choice, previews and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${SUB}/subclass`, {
      '/api/v1/builders/subclasses/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: SubclassDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { lines: ['Fighter subclass', 'Level 3 choice: Lantern Style, pick 1 of Bog Glass, Ember Wick.'] })
      },
      [`/api/v1/builders/subclasses/${SUB}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as SubclassDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    expect(wrapper.get('[data-testid="subclass-lines"]').text()).toContain('Fighter subclass')
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    await set('subclass-class', 'wizard')
    await set('subclass-class', 'fighter')
    await set('feature-0-name', 'Lantern Oath')
    await set('feature-0-text', 'You carry a lantern that never gutters.')
    await wrapper.get('[data-testid="add-resource"]').trigger('click')
    await set('resource-0-name', 'Lantern Light')
    await wrapper.get('[data-testid="add-feature"]').trigger('click')
    await set('feature-1-name', 'Kindle')
    await set('feature-1-uses', 'lantern-light')
    await set('feature-1-spell', 'light')
    await set('feature-1-spell-name', 'Light')
    await set('resource-0-name', 'Lamp Light')
    expect((wrapper.get('[data-testid="feature-1-uses"]').element as HTMLSelectElement).value).toBe('lamp-light')
    await wrapper.get('[data-testid="add-resource"]').trigger('click')
    await set('resource-1-name', 'Embers')
    await set('resource-1-basis', 'fixed')
    await set('resource-1-amount', 2)
    await set('resource-1-from', 7)
    await set('resource-1-die', 'd6')
    await set('resource-1-recharge', 'short_rest')
    await set('resource-1-basis', 'ability')
    await set('resource-1-ability', 'charisma')
    await set('resource-1-basis', 'class_level')
    expect((wrapper.get('[data-testid="resource-1-amount"]').element as HTMLInputElement).value).toBe('1')
    await set('resource-1-amount', 2)
    await wrapper.get('[data-testid="add-feature"]').trigger('click')
    await set('feature-2-level', 7)
    await wrapper.get('[data-testid="feature-2-remove"]').trigger('click')
    await wrapper.get('[data-testid="add-choice"]').trigger('click')
    await set('choice-0-name', 'Lantern Style')
    await set('choice-0-level', 3)
    await set('choice-0-count', 1)
    await set('choice-0-options', 'Bog Glass, , Ember Wick')
    await wrapper.get('[data-testid="add-choice"]').trigger('click')
    await wrapper.get('[data-testid="choice-1-remove"]').trigger('click')
    await wrapper.get('[data-testid="add-resource"]').trigger('click')
    await wrapper.get('[data-testid="resource-2-remove"]').trigger('click')
    await wrapper.get('[data-testid="subclass-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="subclass-lines"]').text()).toContain('Lantern Style, pick 1 of Bog Glass, Ember Wick.')
    expect((sent[0]?.body as { name: string }).name).toBe('Lantern Warden')
    await wrapper.get('[data-testid="subclass-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="subclass-status"]').text()).toBe('Saved as Revision 2.')
    expect(sent.find((x) => x.method === 'PUT')?.body).toEqual({
      class: 'fighter',
      features: [
        { level: 3, name: 'Lantern Oath', text: 'You carry a lantern that never gutters.' },
        { level: 3, name: 'Kindle', text: '', uses: 'lamp-light', spell: 'light', spellName: 'Light' },
      ],
      resources: [
        { key: 'lamp-light', name: 'Lamp Light', basis: 'proficiency', fromLevel: 3, recharge: 'long_rest' },
        { key: 'embers', name: 'Embers', basis: 'class_level', amount: 2, fromLevel: 7, die: 'd6', recharge: 'short_rest' },
      ],
      choices: [{ level: 3, name: 'Lantern Style', count: 1, options: ['Bog Glass', 'Ember Wick'] }],
    })
  })

  it('shows a Shared Library copy without saving, reports a design that will not build, and refuses what is not a subclass', async () => {
    const { wrapper } = await mountApp(`/library/${SUB}/subclass`, {
      '/api/v1/builders/subclasses/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'give it 1 to 40 features' }, 422),
      [`/api/v1/builders/subclasses/${SUB}`]: () => build(start, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="subclass-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="subclass-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="feature-0-remove"]').trigger('click')
    await wrapper.get('[data-testid="subclass-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="subclass-problem"]').text()).toBe('give it 1 to 40 features')
    const refused = await mountApp(`/library/${SUB}/subclass`, {
      [`/api/v1/builders/subclasses/${SUB}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="subclass-error"]').text()).toContain('cannot be opened')
  })

  it('opens a subclass in its builder from the Library entry', async () => {
    const { wrapper } = await mountApp(`/library/${SUB}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/subclasses/${SUB}`]: () => build(start),
      [`/api/v1/library/${SUB}`]: () => ({ entry, revisions: [], uses: [] }),
    })
    await wrapper.get('[data-testid="open-subclass-builder"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('h1').text()).toContain('Subclass builder')
  })
})
