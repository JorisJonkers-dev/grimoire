import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { SpeciesBuild, SpeciesDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const SPECIES = '0190c7a8-0000-7000-8000-0000000000d5'
const at = '2026-10-03T20:00:00Z'
const start: SpeciesDesign = {
  sizes: ['medium'], creatureType: 'humanoid', speedFt: 30, speeds: [], senses: [], resistances: [], traits: [], spells: [], lineages: [],
}
const entry = { id: SPECIES, kind: 'species' as const, name: 'Marshkin', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: SpeciesDesign, extra: Partial<SpeciesBuild> = {}): SpeciesBuild => ({
  entry, design, slug: 'hb-0190c7a80000', lines: ['Marshkin: 30 feet.'], ...extra,
})

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('species builder', () => {
  it('builds the Marshkin with speeds, senses, traits, spells and lineages, previews and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${SPECIES}/species`, {
      '/api/v1/builders/species/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: SpeciesDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { lines: ['Marshkin, Bog lineage: 30 feet; Swim 30 feet.'] })
      },
      [`/api/v1/builders/species/${SPECIES}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as SpeciesDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    const click = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('click')
    const change = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('change')
    await change('species-size-small')
    await change('species-size-large')
    await change('species-size-large')
    await set('species-type', 'fey')
    await set('species-speed', 35)
    await click('species-add-speed')
    await set('species-speed-0-kind', 'climb')
    await set('species-speed-0-feet', 20)
    await click('species-add-speed')
    await click('species-speed-1-remove')
    await click('species-add-sense')
    await set('species-sense-0-kind', 'blindsight')
    await set('species-sense-0-feet', 10)
    await click('species-add-sense')
    await click('species-sense-1-remove')
    await change('species-resist-poison')
    await change('species-resist-fire')
    await change('species-resist-fire')
    await click('species-add-trait')
    await set('species-trait-0-name', 'Reed Breath')
    await set('species-trait-0-text', 'You can hold your breath for an hour.')
    await click('species-add-trait')
    await click('species-trait-1-remove')
    await click('species-add-spell')
    await set('species-spell-0-level', 3)
    await set('species-spell-0-name', 'Fog Cloud')
    await set('species-spell-0-slug', 'fog-cloud')
    await set('species-spell-0-uses', 'long_rest')
    await click('species-add-spell')
    await click('species-spell-1-remove')
    await click('species-add-lineage')
    await set('species-lineage-0-name', 'Bog')
    await set('species-lineage-0-text', 'You know the Druidcraft cantrip.')
    await click('species-lineage-0-add-spell')
    await set('species-lineage-0-spell-0-level', 1)
    await set('species-lineage-0-spell-0-name', 'Druidcraft')
    await set('species-lineage-0-spell-0-slug', 'druidcraft')
    await set('species-lineage-0-spell-0-uses', 'at_will')
    await click('species-add-lineage')
    await click('species-lineage-1-remove')
    await click('species-preview')
    await flushPromises()
    expect(wrapper.get('[data-testid="species-lines"]').text()).toContain('Bog lineage')
    expect((sent[0]?.body as { name: string }).name).toBe('Marshkin')
    await wrapper.get('[data-testid="species-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="species-status"]').text()).toBe('Saved as Revision 2.')
    expect(sent.find((x) => x.method === 'PUT')?.body).toEqual({
      sizes: ['medium', 'small'], creatureType: 'fey', speedFt: 35, speeds: [{ kind: 'climb', feet: 20 }], senses: [{ kind: 'blindsight', feet: 10 }],
      resistances: ['poison'], traits: [{ name: 'Reed Breath', text: 'You can hold your breath for an hour.' }],
      spells: [{ level: 3, spell: 'fog-cloud', name: 'Fog Cloud', uses: 'long_rest' }],
      lineages: [{ name: 'Bog', text: 'You know the Druidcraft cantrip.', spells: [{ level: 1, spell: 'druidcraft', name: 'Druidcraft', uses: 'at_will' }] }],
    })
  })

  it('shows a Shared Library copy without saving, reports a design that will not build, and refuses what is not a species', async () => {
    const { wrapper } = await mountApp(`/library/${SPECIES}/species`, {
      '/api/v1/builders/species/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'choose one or two sizes' }, 422),
      [`/api/v1/builders/species/${SPECIES}`]: () => build(start, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="species-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="species-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="species-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="species-problem"]').text()).toBe('choose one or two sizes')
    const refused = await mountApp(`/library/${SPECIES}/species`, {
      [`/api/v1/builders/species/${SPECIES}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="species-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${SPECIES}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/species/${SPECIES}`]: () => build(start),
      [`/api/v1/library/${SPECIES}`]: () => ({ entry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-species-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Species builder')
  })
})
