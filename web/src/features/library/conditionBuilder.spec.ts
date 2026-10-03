import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { ConditionBuild, ConditionDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const COND = '0190c7a8-0000-7000-8000-0000000000f8'
const at = '2026-10-03T20:00:00Z'
const start: ConditionDesign = { icon: 'spiral', color: '#bfb199', text: '', ends: 'removed', perLevel: { d20: 0, speedFt: 0, deathAt: 0 }, parts: [] }
const entry = { id: COND, kind: 'condition' as const, name: 'Frostbite', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: ConditionDesign, extra: Partial<ConditionBuild> = {}): ConditionBuild => ({ entry, design, slug: 'hb-c', lines: ['Lasts until removed.'], ...extra })

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('condition builder', () => {
  it('builds Frostbite with an icon, stacking levels and parts, previews and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${COND}/condition`, {
      '/api/v1/builders/conditions/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: ConditionDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { lines: ['Ends on a rest.', 'Each level reduces Speed by 5 feet; it rises to level 3 at most.'] })
      },
      [`/api/v1/builders/conditions/${COND}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as ConditionDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    await set('condition-icon', 'snow')
    await set('condition-color', '#7fa8dd')
    expect(wrapper.get('[data-testid="condition-sample"] svg').attributes('stroke')).toBe('#7fa8dd')
    await set('condition-text', 'Cold seeps into the bones.')
    await set('condition-ends', 'save')
    await set('condition-ability', 'constitution')
    await set('condition-ends', 'rest')
    await set('condition-stacks', true)
    await set('condition-max-level', 3)
    await set('condition-d20', 1)
    await set('condition-speed', 5)
    await set('condition-death', 0)
    for (let i = 0; i < 4; i++) await wrapper.get('[data-testid="condition-add-part"]').trigger('click')
    await set('condition-part-1-type', 'save_disadvantage')
    await set('condition-part-1-ability', 'dexterity')
    await set('condition-part-2-type', 'speed_penalty')
    await set('condition-part-2-feet', 10)
    await set('condition-part-3-type', 'manual')
    await set('condition-part-3-text', 'The DM decides what the cold does to gear.')
    await wrapper.get('[data-testid="condition-add-part"]').trigger('click')
    await wrapper.get('[data-testid="condition-part-4-remove"]').trigger('click')
    await wrapper.get('[data-testid="condition-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="condition-lines"]').text()).toContain('it rises to level 3 at most')
    await wrapper.get('[data-testid="condition-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="condition-status"]').text()).toBe('Saved as Revision 2.')
    expect(sent.find((x) => x.method === 'PUT')?.body).toEqual({
      icon: 'snow', color: '#7fa8dd', text: 'Cold seeps into the bones.', ends: 'rest', ability: 'constitution', stacks: true, maxLevel: 3,
      perLevel: { d20: 1, speedFt: 5, deathAt: 0 },
      parts: [
        { type: 'attacked_advantage' }, { type: 'save_disadvantage', ability: 'dexterity' }, { type: 'speed_penalty', feet: 10 },
        { type: 'manual', text: 'The DM decides what the cold does to gear.' },
      ],
    })
  })

  it('builds a lingering injury: a condition that lasts until its cure, which it names', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp(`/library/${COND}/condition`, {
      [`/api/v1/builders/conditions/${COND}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          sent.push(await req.clone().json())
          return build(start, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    const set = (id: string, value: string) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    expect(wrapper.find('[data-testid="condition-cure"]').exists()).toBe(false)
    await set('condition-ends', 'cure')
    await set('condition-cure', 'Regenerate, or a month of rest')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="condition-form"]').trigger('submit')
    await flushPromises()
    expect(sent.at(-1)).toMatchObject({ ends: 'cure', cure: 'Regenerate, or a month of rest' })
    // Made to last some other way, it names no cure.
    await set('condition-ends', 'removed')
    expect(wrapper.find('[data-testid="condition-cure"]').exists()).toBe(false)
    await wrapper.get('[data-testid="condition-form"]').trigger('submit')
    await flushPromises()
    expect(sent.at(-1)).toMatchObject({ ends: 'removed' })
    expect(sent.at(-1)).not.toHaveProperty('cure')
  })

  it('shows a Shared Library copy, reports a design that will not build, and opens from the entry', async () => {
    const { wrapper } = await mountApp(`/library/${COND}/condition`, {
      '/api/v1/builders/conditions/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'choose an icon' }, 422),
      [`/api/v1/builders/conditions/${COND}`]: () => build(start, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="condition-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="condition-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="condition-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="condition-problem"]').text()).toBe('choose an icon')
    const refused = await mountApp(`/library/${COND}/condition`, {
      [`/api/v1/builders/conditions/${COND}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="condition-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${COND}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/conditions/${COND}`]: () => build(start),
      [`/api/v1/library/${COND}`]: () => ({ entry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-condition-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Condition builder')
  })
})
