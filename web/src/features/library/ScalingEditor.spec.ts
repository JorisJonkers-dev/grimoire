import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { ref } from 'vue'
import type { PartField, PartKind, PartScaling } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import served from '../../../../fixtures/part-kinds.json'
import ScalingEditor from './ScalingEditor.vue'

// What the server serves: every kind's fields marked as they scale, and a spell's own.
const kinds = served.kinds as PartKind[]
const activation = served.activation.spell as PartField[]
const scalable = (slug: string) => kinds.find((k) => k.slug === slug)?.fields.filter((f) => f.scalable) ?? []

const mounted: { unmount: () => void }[] = []
afterEach(() => {
  for (const w of mounted.splice(0)) w.unmount()
  document.body.innerHTML = ''
})

function editor(fields: PartField[], start?: PartScaling[]) {
  const model = ref<PartScaling[] | undefined>(start)
  const wrapper = mount(ScalingEditor, {
    props: { modelValue: model.value, fields, prefix: 'sc', label: 'Scaling', 'onUpdate:modelValue': (v: PartScaling[] | undefined) => (model.value = v) },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  const sync = async () => {
    await wrapper.setProps({ modelValue: model.value })
  }
  const get = (id: string) => wrapper.get(`[data-testid="${id}"]`)
  const options = (id: string) => get(id).findAll('option').map((o) => o.attributes('value'))
  return { wrapper, model, get, sync, options }
}

describe('the Scaling editor drawn from the served fields', () => {
  it('grows a Part field by a linear rule, then by a table, and writes nothing when no rule is left', async () => {
    const { model, get, sync, options, wrapper } = editor(scalable('save_damage'))
    await get('sc-add').trigger('click')
    await sync()
    expect(model.value).toEqual([{ field: 'dice', axis: 'slot_level', rule: 'linear', base: 1, per: { text: '1d6' } }])
    expect(options('sc-0-field')).toEqual(['dice'])
    expect(options('sc-0-axis')).toEqual(['slot_level', 'character_level', 'class_level', 'table_column', 'charges'])
    // Dice take no formula; a column only along a class table.
    expect(options('sc-0-rule')).toEqual(['linear', 'table'])
    await get('sc-0-base').setValue('3')
    await get('sc-0-per').setValue('1d8')
    expect(model.value?.[0]).toMatchObject({ base: 3, per: { text: '1d8' } })

    await get('sc-0-rule').setValue('table')
    await sync()
    expect(model.value?.[0]).toEqual({ field: 'dice', axis: 'slot_level', rule: 'table', steps: [{ at: 2, value: { text: '1d6' } }] })
    await get('sc-0-add-step').trigger('click')
    await sync()
    await get('sc-0-step-1-value').setValue('3d6')
    await get('sc-0-step-1-at').setValue('5')
    expect(model.value?.[0]?.steps).toEqual([{ at: 2, value: { text: '1d6' } }, { at: 5, value: { text: '3d6' } }])
    await get('sc-0-step-0-remove').trigger('click')
    await sync()
    expect(model.value?.[0]?.steps).toEqual([{ at: 5, value: { text: '3d6' } }])
    await expectAccessible(wrapper.element as Element)

    await get('sc-0-remove').trigger('click')
    await sync()
    expect(model.value).toBeUndefined()
  })

  it('names the class of a class level, takes a class table only by its column, and a formula where the field allows one', async () => {
    const { model, get, sync, options } = editor(scalable('temp_hp'), [{ field: 'amount', axis: 'slot_level', rule: 'linear', base: 1, per: { number: 5 } }])
    expect(options('sc-0-rule')).toEqual(['linear', 'table', 'formula'])
    await get('sc-0-per').setValue('7')
    expect(model.value?.[0]?.per).toEqual({ number: 7 })
    await get('sc-0-rule').setValue('formula')
    await sync()
    await get('sc-0-formula').setValue('11 + level')
    expect(model.value?.[0]).toEqual({ field: 'amount', axis: 'slot_level', rule: 'formula', formula: '11 + level' })

    await get('sc-0-axis').setValue('class_level')
    await sync()
    await get('sc-0-class').setValue('cleric')
    expect(model.value?.[0]).toMatchObject({ axis: 'class_level', class: 'cleric', rule: 'formula' })
    await get('sc-0-axis').setValue('table_column')
    await sync()
    expect(options('sc-0-rule')).toEqual(['column'])
    await get('sc-0-column').setValue('Rage Damage')
    expect(model.value?.[0]).toEqual({ field: 'amount', axis: 'table_column', class: 'cleric', rule: 'column', column: 'Rage Damage' })
    await get('sc-0-axis').setValue('charges')
    await sync()
    expect(model.value?.[0]).toEqual({ field: 'amount', axis: 'charges', rule: 'linear', base: 1, per: { number: 1 } })
    expect(get('sc-0').find('[data-testid="sc-0-class"]').exists()).toBe(false)
  })

  it("grows the spell's own fields: a flag and a choice only by a table, each step its field's own control", async () => {
    const { model, get, sync, options } = editor(activation)
    expect(get('sc-add-field').findAll('option').map((o) => o.text())).toEqual(['Size in feet', 'Range in feet', 'Duration', 'Duration unit', 'Concentration'])
    await get('sc-add-field').setValue('concentration')
    await get('sc-add').trigger('click')
    await sync()
    expect(options('sc-0-rule')).toEqual(['table'])
    expect(model.value).toEqual([{ field: 'concentration', axis: 'slot_level', rule: 'table', steps: [{ at: 2, value: {} }] }])
    await get('sc-0-step-0-value').setValue(true)
    expect(model.value?.[0]?.steps?.[0]?.value).toEqual({ flag: true })

    await get('sc-0-field').setValue('duration_unit')
    await sync()
    expect(model.value?.[0]).toEqual({ field: 'duration_unit', axis: 'slot_level', rule: 'table', steps: [{ at: 2, value: { text: 'instant' } }] })
    await get('sc-0-step-0-value').setValue('hours')
    expect(model.value?.[0]?.steps?.[0]?.value).toEqual({ text: 'hours' })
    await get('sc-0-field').setValue('size_ft')
    await sync()
    expect(model.value?.[0]).toEqual({ field: 'size_ft', axis: 'slot_level', rule: 'linear', base: 1, per: { number: 1 } })
  })

  it('offers nothing to add when no field scales', () => {
    const { wrapper } = editor([])
    expect(wrapper.find('[data-testid="sc-add"]').exists()).toBe(false)
  })
})
