import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { ClassBuild, ClassDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const CLASS = '0190c7a8-0000-7000-8000-0000000000c4'
const at = '2026-10-03T20:00:00Z'
const start: ClassDesign = {
  hitDie: 8, primary: ['strength'], saves: ['strength', 'constitution'], armor: ['light'], weapons: ['simple'], skills: 2, subclassLevel: 3,
  featLevels: [4, 8, 12, 16, 19], columns: [], features: [{ level: 1, name: 'First feature', text: '' }], casting: { kind: 'none' },
}
const entry = { id: CLASS, kind: 'class' as const, name: 'Lamplighter', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: ClassDesign, extra: Partial<ClassBuild> = {}): ClassBuild => ({
  entry, design, slug: 'hb-0190c7a80000', lines: ['Hit Die d8', 'No spellcasting'], ...extra,
})

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('class builder', () => {
  it('builds the Lamplighter with its own slot table, a column and features, previews and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${CLASS}/class`, {
      '/api/v1/builders/classes/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: ClassDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { lines: ['Hit Die d10', 'Spellcasting: Charisma, from the bard list, its own slot table'] })
      },
      [`/api/v1/builders/classes/${CLASS}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as ClassDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    expect(wrapper.find('[data-testid="class-table"] thead').text()).toBe('Level')
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    const click = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('click')
    const change = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('change')
    await set('class-hit-die', 10)
    await set('class-save-0', 'dexterity')
    await set('class-save-1', 'charisma')
    await set('class-skills', 3)
    await set('class-subclass-level', 2)
    await set('class-feat-levels', '4, x, 10')
    await change('class-primary-strength')
    await change('class-primary-charisma')
    await set('class-any-primary', true)
    await change('class-armor-shields')
    await change('class-armor-light')
    await change('class-weapons-martial')
    await set('class-feature-0-name', 'Wickcraft')
    await set('class-feature-0-text', 'You tend lanterns.')
    await click('class-add-feature')
    await set('class-feature-1-level', 5)
    await click('class-feature-1-remove')
    await set('class-casting-kind', 'slots')
    await set('class-casting-ability', 'charisma')
    await set('class-spell-list', 'bard')
    await set('class-after-rest', true)
    await set('class-cantrips-1', 2)
    await set('class-prepared-1', 2)
    await set('class-casting-kind', 'points')
    await set('class-cost-1', 3)
    await set('class-points-1', 5)
    await set('class-max-spell-1', 1)
    await set('class-spellbook', true)
    await set('class-casting-kind', 'full')
    await set('class-casting-kind', 'slots')
    await set('class-slots-1-1', 1)
    await click('class-add-column')
    await set('class-column-0-name', 'Wick Marks')
    await set('class-column-0-1', '+1')
    await click('class-add-column')
    await click('class-column-1-remove')
    expect(wrapper.get('[data-testid="class-table"] thead').text()).toContain('Wick Marks')
    await click('class-preview')
    await flushPromises()
    expect(wrapper.get('[data-testid="class-lines"]').text()).toContain('its own slot table')
    expect((sent[0]?.body as { name: string }).name).toBe('Lamplighter')
    await wrapper.get('[data-testid="class-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="class-status"]').text()).toBe('Saved as Revision 2.')
    const saved = sent.find((x) => x.method === 'PUT')?.body as ClassDesign
    expect({ ...saved, casting: undefined, columns: undefined }).toEqual({
      hitDie: 10, primary: ['charisma'], anyPrimary: true, saves: ['dexterity', 'charisma'], armor: ['shields'], weapons: ['simple', 'martial'], skills: 3,
      subclassLevel: 2, featLevels: [4, 10], features: [{ level: 1, name: 'Wickcraft', text: 'You tend lanterns.' }],
    })
    expect(saved.columns[0]?.values[0]).toBe('+1')
    expect(saved.casting).toMatchObject({ kind: 'slots', ability: 'charisma', spellList: 'bard', afterRest: true, spellbook: true })
    expect(saved.casting.cantrips?.[0]).toBe(2)
    expect(saved.casting.slots?.[0]?.[0]).toBe(1)
    expect(saved.casting.slots?.[2]).toEqual([4, 2, 0, 0, 0, 0, 0, 0, 0])
    expect(saved.casting.points).toBeUndefined()
  })

  it('starts spell points from a sensible table, and turns spellcasting off again', async () => {
    const sent: ClassDesign[] = []
    const { wrapper } = await mountApp(`/library/${CLASS}/class`, {
      [`/api/v1/builders/classes/${CLASS}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          sent.push((await req.clone().json()) as ClassDesign)
          return build(sent[sent.length - 1] ?? start, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    await wrapper.get('[data-testid="class-casting-kind"]').setValue('half')
    expect((wrapper.get('[data-testid="class-prepared-2"]').element as HTMLInputElement).value).toBe('3')
    await wrapper.get('[data-testid="class-casting-kind"]').setValue('none')
    await wrapper.get('[data-testid="class-casting-kind"]').setValue('points')
    expect((wrapper.get('[data-testid="class-prepared-2"]').element as HTMLInputElement).value).toBe('5')
    expect((wrapper.get('[data-testid="class-points-20"]').element as HTMLInputElement).value).toBe('80')
    expect((wrapper.get('[data-testid="class-max-spell-20"]').element as HTMLInputElement).value).toBe('9')
    await wrapper.get('[data-testid="class-casting-kind"]').setValue('none')
    await wrapper.get('[data-testid="class-form"]').trigger('submit')
    await flushPromises()
    expect(sent[0]?.casting).toEqual({ kind: 'none' })
  })

  it('shows a Shared Library copy without saving, reports a design that will not build, and refuses what is not a class', async () => {
    const { wrapper } = await mountApp(`/library/${CLASS}/class`, {
      '/api/v1/builders/classes/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'give it a Hit Die of d6, d8, d10 or d12' }, 422),
      [`/api/v1/builders/classes/${CLASS}`]: () => build(start, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="class-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="class-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="class-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="class-problem"]').text()).toBe('give it a Hit Die of d6, d8, d10 or d12')
    const refused = await mountApp(`/library/${CLASS}/class`, {
      [`/api/v1/builders/classes/${CLASS}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="class-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${CLASS}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/classes/${CLASS}`]: () => build(start),
      [`/api/v1/library/${CLASS}`]: () => ({ entry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-class-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Class builder')
  })
})
