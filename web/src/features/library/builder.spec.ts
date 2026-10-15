import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { Part, PartField, SpellBuild, SpellDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'
import served from '../../../../fixtures/part-kinds.json'

const SPELL = '0190c7a8-0000-7000-8000-0000000000e9'
// charmedIf charms a creature of one type that fails the spell's save.
const charmedIf = (creature: string): Part => ({
  kind: 'branch', values: { when: { text: 'creature_is' }, creature_type: { text: creature } },
  groups: [{ name: 'then', parts: [{
    kind: 'branch', values: { when: { text: 'fails_by' }, n: { number: 1 } },
    groups: [{ name: 'then', parts: [{ kind: 'save_condition', values: { condition: { text: 'charmed' } }, groups: [] }] }],
  }] }],
})
const at = '2026-10-01T20:00:00Z'
const start: SpellDesign = {
  targeting: { shape: 'sphere', sizeFt: 20, rangeFt: 60 }, concentration: false, duration: { unit: 'instant' }, ritual: false,
  castingTime: { kind: 'action' }, components: { verbal: true, somatic: true }, parts: [],
}
const entry = { id: SPELL, kind: 'spell' as const, name: 'Marsh Lantern', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: SpellDesign, extra: Partial<SpellBuild> = {}): SpellBuild => ({
  entry,
  design, effect: 'hb-0190c7a80000', text: ['Casting Time: 1 action.'], hexes: [{ q: 0, r: 0 }, { q: 1, r: 0 }], holds: served.holds.spell,
  activation: served.activation.spell as PartField[], ...extra,
})
const kinds = { '/api/v1/builders/part-kinds': () => served.kinds }

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('effect builder', () => {
  it('builds the Marsh Lantern from the served kinds, previews it and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${SPELL}/spell`, {
      ...kinds,
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
    const add = async (list: string, kind: string) => {
      await set(`${list}-kind`, kind)
      await wrapper.get(`[data-testid="${list}-add"]`).trigger('click')
    }
    await set('turn-damage', true)
    await set('turn-dice', '1d6')
    await set('turn-type', 'radiant')
    for (const [i, creature] of ['undead', 'fey'].entries()) {
      await add('parts', 'branch')
      await set(`parts-${String(i)}-when`, 'creature_is')
      await set(`parts-${String(i)}-creature_type`, creature)
      await add(`parts-${String(i)}-then`, 'branch')
      await set(`parts-${String(i)}-then-0-n`, 1)
      await add(`parts-${String(i)}-then-0-then`, 'save_condition')
      await set(`parts-${String(i)}-then-0-then-0-condition`, 'charmed')
    }
    for (const kind of ['light', 'reveal', 'create_surface', 'manual']) await add('parts', kind)
    await set('parts-2-bright_ft', 20)
    await set('parts-2-dim_ft', 20)
    await set('parts-3-qualities-invisible', true)
    await set('parts-4-surface', 'fog')
    await set('parts-4-rounds', 3)
    await set('parts-5-instruction', 'Ring the bell.')
    await wrapper.get('[data-testid="parts-5-remove"]').trigger('click')
    await wrapper.get('[data-testid="parts-4-remove"]').trigger('click')
    await wrapper.get('[data-testid="parts-2-down"]').trigger('click')
    await wrapper.get('[data-testid="parts-3-up"]').trigger('click')
    await wrapper.get('[data-testid="parts-0-up"]').trigger('click')
    await wrapper.get('[data-testid="parts-0"]').trigger('dragstart')
    await wrapper.get('[data-testid="parts-3"]').trigger('drop')
    await wrapper.get('[data-testid="parts-3"]').trigger('dragstart')
    await wrapper.get('[data-testid="parts-0"]').trigger('drop')
    expect(wrapper.findAll('[data-testid="parts"] > li > .head > .kind').map((k) => k.text())).toEqual(['Branch', 'Branch', 'Light', 'Reveal'])
    await expectAccessible(wrapper.element as Element)
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
      turnDamage: { dice: '1d6', damageType: 'radiant' },
      parts: [
        charmedIf('undead'),
        charmedIf('fey'),
        { kind: 'light', values: { bright_ft: { number: 20 }, dim_ft: { number: 20 } }, groups: [] },
        { kind: 'reveal', values: { qualities: { texts: ['invisible'] } }, groups: [] },
      ],
    })
  })

  it('scales its area and its damage with the slot, and saves both rules', async () => {
    let saved: SpellDesign | undefined
    const { wrapper } = await mountApp(`/library/${SPELL}/spell`, {
      ...kinds,
      [`/api/v1/builders/spells/${SPELL}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          saved = (await req.clone().json()) as SpellDesign
          return build(saved, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone({ ...start, save: 'dexterity' as const }))
      },
    })
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    const click = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('click')
    await set('scaling-add-field', 'size_ft')
    await click('scaling-add')
    await set('scaling-0-base', 3)
    await set('scaling-0-per', 5)
    await set('parts-kind', 'save_damage')
    await click('parts-add')
    await set('parts-0-dice', '8d6')
    await click('parts-0-scaling-add')
    await set('parts-0-scaling-0-base', 3)
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="builder-form"]').trigger('submit')
    await flushPromises()
    expect(saved?.scaling).toEqual([{ field: 'size_ft', axis: 'slot_level', rule: 'linear', base: 3, per: { number: 5 } }])
    expect(saved?.parts[0]?.scaling).toEqual([{ field: 'dice', axis: 'slot_level', rule: 'linear', base: 3, per: { text: '1d6' } }])
  })

  it('reads every stored Part back into the form, the ones it would not offer too, and writes them back as they were', async () => {
    const summon: Part = { kind: 'summon', values: { monster: { text: 'wolf' }, count: { number: 2 } }, groups: [] }
    const odd: Part = { kind: 'branch', values: { when: { text: 'hp_at_most' }, n: { number: 10 } }, groups: [{ name: 'then', parts: [{ kind: 'manual', values: { instruction: { text: 'Flee.' } }, groups: [] }] }] }
    const stored: SpellDesign = {
      ...start, save: 'wisdom', turnDamage: { dice: '1d4', damageType: 'acid' },
      parts: [
        { kind: 'save_damage', values: { dice: { text: '8d6' }, damage_type: { text: 'fire' }, half: { flag: true } }, groups: [] },
        { kind: 'branch', values: { when: { text: 'fails_by' }, n: { number: 1 } }, groups: [{ name: 'then', parts: [{ kind: 'save_condition', values: { condition: { text: 'prone' } }, groups: [] }] }] },
        charmedIf('undead'), charmedIf('fey'),
        { kind: 'create_surface', values: { surface: { text: 'fog' }, rounds: { number: 3 } }, groups: [] },
        { kind: 'manual', values: { instruction: { text: 'Ring the bell.' } }, groups: [] },
        summon, odd,
      ],
    }
    const sent: SpellDesign[] = []
    const { wrapper } = await mountApp(`/library/${SPELL}/spell`, {
      ...kinds,
      [`/api/v1/builders/spells/${SPELL}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          sent.push((await req.clone().json()) as SpellDesign)
          return build(stored, { entry: { ...entry, revision: 2 } })
        }
        return build(structuredClone(stored))
      },
    })
    expect(wrapper.findAll('[data-testid="parts"] > li > .head > .kind').map((k) => k.text())).toEqual([
      'Save for damage', 'Branch', 'Branch', 'Branch', 'Surface', 'Manual Effect', 'Summon', 'Branch',
    ])
    const value = (id: string) => (wrapper.get(`[data-testid="${id}"]`).element as HTMLInputElement).value
    expect(value('parts-3-creature_type')).toBe('fey')
    expect(value('parts-3-then-0-then-0-condition')).toBe('charmed')
    expect(value('parts-6-monster')).toBe('wolf')
    expect(value('parts-7-then-0-instruction')).toBe('Flee.')
    expect((wrapper.get('[data-testid="turn-damage"]').element as HTMLInputElement).checked).toBe(true)
    expect(value('turn-dice')).toBe('1d4')
    await wrapper.get('[data-testid="builder-form"]').trigger('submit')
    await flushPromises()
    expect(sent[0]).toEqual(stored)
  })

  it('shows a Shared Library copy without saving, reports a design that will not build, and refuses what is not a spell', async () => {
    const { wrapper } = await mountApp(`/library/${SPELL}/spell`, {
      ...kinds,
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
