import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { SpellBuild, SpellDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const SPELL = '0190c7a8-0000-7000-8000-0000000000e9'
const at = '2026-10-01T20:00:00Z'
const start: SpellDesign = {
  targeting: { shape: 'sphere', sizeFt: 20, rangeFt: 60 }, concentration: false, duration: { unit: 'instant' }, ritual: false,
  castingTime: { kind: 'action' }, components: { verbal: true, somatic: true }, parts: [],
}
const entry = { id: SPELL, kind: 'spell' as const, name: 'Marsh Lantern', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: SpellDesign, extra: Partial<SpellBuild> = {}): SpellBuild => ({
  entry,
  design, effect: 'hb-0190c7a80000', text: ['Casting Time: 1 action.'], hexes: [{ q: 0, r: 0 }, { q: 1, r: 0 }], ...extra,
})

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('effect builder', () => {
  it('builds the Marsh Lantern from rows, previews it and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${SPELL}/spell`, {
      '/api/v1/builders/spells/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { design: SpellDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { text: ['Area: a 10-foot emanation from the caster.', 'A creature that starts its turn in the area takes 1d6 radiant damage.'], hexes: Array.from({ length: 18 }, (_, i) => ({ q: i, r: 0 })) })
      },
      [`/api/v1/builders/spells/${SPELL}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as SpellDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(start))
      },
    })
    expect(wrapper.findAll('[data-testid="area-hex"]')).toHaveLength(2)
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    await set('shape', 'emanation')
    await set('size', 10)
    await set('range', 0)
    await set('save', 'wisdom')
    await set('casting', 'reaction')
    await set('trigger', 'when_hit')
    await set('casting', 'minutes')
    await set('casting-minutes', 10)
    await set('casting', 'action')
    await set('duration', 'minutes')
    await set('duration-amount', 1)
    await set('concentration', true)
    await set('ritual', true)
    await set('somatic', false)
    await set('material', true)
    await set('material-text', 'a lantern of bog glass')
    await set('material-cost', 25)
    await set('material-consumed', true)
    await set('material-item', 'lantern')
    await set('material', false)
    await set('material', true)
    await set('material-text', 'a lantern of bog glass')
    for (const type of ['damage', 'condition', 'light', 'reveal', 'surface', 'manual']) {
      await set('add-type', type)
      await wrapper.get('[data-testid="add-part"]').trigger('click')
    }
    await set('when-0', 'start_of_turn')
    await set('dice-0', '1d6')
    await set('damage-type-0', 'radiant')
    await set('condition-1', 'charmed')
    await wrapper.get('[data-testid="only-1-undead"]').trigger('change')
    await wrapper.get('[data-testid="only-1-fey"]').trigger('change')
    await wrapper.get('[data-testid="only-1-fey"]').trigger('change')
    await wrapper.get('[data-testid="only-1-fey"]').trigger('change')
    await set('bright-2', 20)
    await set('dim-2', 20)
    await wrapper.get('[data-testid="quality-3-hidden"]').trigger('change')
    await wrapper.get('[data-testid="quality-3-hidden"]').trigger('change')
    await set('surface-4', 'fog')
    await set('rounds-4', 3)
    await set('text-5', 'Ring the bell.')
    await wrapper.get('[data-testid="remove-5"]').trigger('click')
    await wrapper.get('[data-testid="remove-4"]').trigger('click')
    await wrapper.get('[data-testid="down-2"]').trigger('click')
    await wrapper.get('[data-testid="up-3"]').trigger('click')
    await wrapper.get('[data-testid="up-0"]').trigger('click')
    await wrapper.get('[data-testid="part-0"]').trigger('dragstart')
    await wrapper.get('[data-testid="part-3"]').trigger('drop')
    await wrapper.get('[data-testid="part-3"]').trigger('dragstart')
    await wrapper.get('[data-testid="part-0"]').trigger('drop')
    expect(wrapper.findAll('[data-testid="parts"] .kind').map((k) => k.text())).toEqual(['damage', 'condition', 'light', 'reveal'])
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="area-hex"]')).toHaveLength(18)
    expect(wrapper.get('[data-testid="rules-text"]').text()).toContain('takes 1d6 radiant damage')
    await wrapper.get('[data-testid="builder-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="builder-status"]').text()).toBe('Saved as Revision 2.')
    const saved = sent.find((x) => x.method === 'PUT')?.body as SpellDesign
    expect(saved).toEqual({
      targeting: { shape: 'emanation', sizeFt: 10, rangeFt: 0 }, save: 'wisdom', concentration: true, duration: { unit: 'minutes', amount: 1 }, ritual: true,
      castingTime: { kind: 'action', minutes: 10, trigger: 'when_hit' }, components: { verbal: true, somatic: false, material: { text: 'a lantern of bog glass' } },
      parts: [
        { type: 'damage', when: 'start_of_turn', dice: '1d6', damageType: 'radiant', half: true },
        { type: 'condition', condition: 'charmed', onlyTypes: ['undead', 'fey'] },
        { type: 'light', brightFt: 20, dimFt: 20 },
        { type: 'reveal', qualities: ['invisible'] },
      ],
    })
  })

  it('shows a Shared Library copy without saving, reports a design that will not build, and refuses what is not a spell', async () => {
    const { wrapper } = await mountApp(`/library/${SPELL}/spell`, {
      '/api/v1/builders/spells/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'a condition needs the save the targets make' }, 422),
      [`/api/v1/builders/spells/${SPELL}`]: () => build(start, { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="builder-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="save-spell"]').exists()).toBe(false)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="builder-problem"]').text()).toBe('a condition needs the save the targets make')
    const refused = await mountApp(`/library/${SPELL}/spell`, {
      [`/api/v1/builders/spells/${SPELL}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="builder-error"]').text()).toContain('cannot be opened')
  })
})
