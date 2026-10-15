import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { reactive } from 'vue'
import type { Part, PartHolds, PartKind } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import served from '../../../../fixtures/part-kinds.json'
import PartsEditor from './PartsEditor.vue'

// What the server serves: every kind's definition, and what each builder takes as Parts.
const kinds = served.kinds as PartKind[]
const spell: PartHolds = served.holds.spell
const condition: PartHolds = served.holds.condition

const mounted: { unmount: () => void }[] = []
afterEach(() => {
  for (const w of mounted.splice(0)) w.unmount()
  document.body.innerHTML = ''
})

function editor(holds: PartHolds, parts: Part[] = [], extra: PartKind[] = []) {
  const model = reactive(parts)
  const wrapper = mount(PartsEditor, {
    props: { modelValue: model, kinds: [...kinds, ...extra], holds, allowed: holds.kinds, label: 'Parts', adds: 'Add a Part', noun: 'part', prefix: 'parts' },
    attachTo: document.body,
  })
  mounted.push(wrapper)
  const get = (id: string) => wrapper.get(`[data-testid="${id}"]`)
  const add = async (list: string, kind: string) => {
    await get(`${list}-kind`).setValue(kind)
    await get(`${list}-add`).trigger('click')
  }
  const labels = (item: string) => get(item).findAll(':scope > .fields label > span, :scope > .fields fieldset > legend').map((s) => s.text())
  return { wrapper, model, get, add, labels }
}

describe('the Part form drawn from the served definitions', () => {
  it('offers only the kinds the builder takes, and draws each one field for field in its hinted order', async () => {
    const { model, get, add, labels, wrapper } = editor(spell)
    expect(get('parts-kind').findAll('option').map((o) => o.text())).toEqual(['Branch', 'Surface', 'Light', 'Manual Effect', 'Reveal', 'Save or take a condition', 'Save for damage'])
    await add('parts', 'save_damage')
    // The spell fills in its own save, so the form leaves it out.
    expect(labels('parts-0')).toEqual(['Damage', 'Damage type', 'Half damage on a successful save'])
    expect(get('parts-0').find('.kind').text()).toBe('Save for damage')
    const dice = get('parts-0-dice')
    expect(document.getElementById(dice.attributes('aria-describedby') ?? '')?.textContent).toBe('Like 8d6.')
    await dice.setValue('8d6')
    await get('parts-0-damage_type').setValue('fire')
    await get('parts-0-half').setValue(true)
    await get('parts-0-half').setValue(false)
    await dice.setValue('8d6')
    expect(model).toEqual([{ kind: 'save_damage', values: { dice: { text: '8d6' }, damage_type: { text: 'fire' } }, groups: [] }])

    await add('parts', 'light')
    expect(get('parts-1-bright_ft').attributes()).toMatchObject({ type: 'number', min: '5', max: '120' })
    await get('parts-1-dim_ft').setValue('20')
    await get('parts-1-bright_ft').setValue('')
    await add('parts', 'reveal')
    await get('parts-2-qualities-invisible').setValue(true)
    await get('parts-2-qualities-hidden').setValue(true)
    await get('parts-2-qualities-hidden').setValue(false)
    await add('parts', 'create_surface')
    // A Surface comes from the catalogue the spell checks against, not free text.
    expect(get('parts-3-surface').element.tagName).toBe('SELECT')
    await get('parts-3-surface').setValue('grease')
    await add('parts', 'manual')
    expect(get('parts-4-instruction').attributes('maxlength')).toBe('500')
    await get('parts-4-instruction').setValue('Ring the bell.')
    await get('parts-4-instruction').setValue('')
    expect(model.slice(1)).toEqual([
      { kind: 'light', values: { dim_ft: { number: 20 } }, groups: [] },
      { kind: 'reveal', values: { qualities: { texts: ['invisible'] } }, groups: [] },
      { kind: 'create_surface', values: { surface: { text: 'grease' }, rounds: { number: 1 } }, groups: [] },
      { kind: 'manual', values: {}, groups: [] },
    ])
    await expectAccessible(wrapper.element as Element)
  })

  it('draws sections as named groups of fields and starts each required field at its first value', async () => {
    const { model, get, add, labels, wrapper } = editor(condition)
    expect(get('parts-kind').findAll('option').map((o) => o.attributes('value'))).toEqual(condition.kinds)
    for (const kind of ['edge', 'save_edge', 'speed_penalty', 'immobile']) await add('parts', kind)
    expect(labels('parts-0')).toEqual(['Advantage', 'Which attacks', 'On attacks against the bearer', 'From', "Only for the Effect's source"])
    expect(get('parts-0').get('fieldset > legend').text()).toBe('Which attacks')
    expect(labels('parts-1')).toEqual(['Edge', 'Saving throw'])
    expect(labels('parts-3')).toEqual([])
    await get('parts-0-against').setValue(true)
    await get('parts-0-advantage').setValue(true)
    await get('parts-1-mode').setValue('disadvantage')
    await get('parts-1-ability').setValue('dexterity')
    await get('parts-2-ft').setValue('10')
    expect(model).toEqual([
      { kind: 'edge', values: { range: { text: 'any' }, against: { flag: true }, advantage: { flag: true } }, groups: [] },
      { kind: 'save_edge', values: { ability: { text: 'dexterity' }, mode: { text: 'disadvantage' } }, groups: [] },
      { kind: 'speed_penalty', values: { ft: { number: 10 } }, groups: [] },
      { kind: 'immobile', values: {}, groups: [] },
    ])
    await expectAccessible(wrapper.element as Element)
  })

  it('draws the Parts beneath a branch through the same form, offering only what a branch takes', async () => {
    const { model, get, add, labels, wrapper } = editor(spell)
    await add('parts', 'branch')
    expect(get('parts-0').get('fieldset.group > legend').text()).toBe('Then')
    expect(get('parts-0-then').attributes('aria-label')).toBe('Then')
    expect(get('parts-0-then-kind').findAll('option').map((o) => o.text())).toEqual(['Branch', 'Manual Effect', 'Save or take a condition'])
    await get('parts-0-when').setValue('creature_is')
    await get('parts-0-creature_type').setValue('fey')
    await add('parts-0-then', 'branch')
    await get('parts-0-then-0-n').setValue('1')
    await add('parts-0-then-0-then', 'save_condition')
    // A condition is one play knows: a choice from the spell's list, and the save is the spell's own.
    expect(labels('parts-0-then-0-then-0')).toEqual(['Condition'])
    expect(get('parts-0-then-0-then-0-condition').findAll('option').map((o) => o.text())).toContain('charmed')
    await get('parts-0-then-0-then-0-condition').setValue('charmed')
    expect(get('parts-0-then-0-then-0-remove').attributes('aria-label')).toBe('Remove part 1, then, part 1, then, part 1')
    expect(model).toEqual([{
      kind: 'branch', values: { when: { text: 'creature_is' }, creature_type: { text: 'fey' } },
      groups: [{ name: 'then', parts: [{
        kind: 'branch', values: { when: { text: 'fails_by' }, n: { number: 1 } },
        groups: [{ name: 'then', parts: [{ kind: 'save_condition', values: { condition: { text: 'charmed' } }, groups: [] }] }],
      }] }],
    }])
    await expectAccessible(wrapper.element as Element)
  })

  it('lets the author name the open groups of a choice, each with its own Parts', async () => {
    const holds: PartHolds = { kinds: ['choice'], beneath: ['manual'], fills: [], narrows: [] }
    const { model, get, add, wrapper } = editor(holds)
    await add('parts', 'choice')
    expect(get('parts-0').findAll('fieldset.group')).toHaveLength(0)
    await get('parts-0-add-mode').trigger('click')
    await get('parts-0-add-mode').trigger('click')
    expect(get('parts-0').findAll('fieldset.group > legend').map((l) => l.text())).toEqual(['Mode 1', 'Mode 2'])
    await get('parts-0-mode-0-name').setValue('Enlarge')
    expect(get('parts-0-mode-0-kind').findAll('option').map((o) => o.text())).toEqual(['Manual Effect'])
    await add('parts-0-mode-0', 'manual')
    await get('parts-0-mode-0-0-instruction').setValue('Grow.')
    await get('parts-0-mode-1-remove').trigger('click')
    expect(model).toEqual([{ kind: 'choice', values: {}, groups: [{ name: 'Enlarge', parts: [{ kind: 'manual', values: { instruction: { text: 'Grow.' } }, groups: [] }] }] }])
    await expectAccessible(wrapper.element as Element)
  })

  it('moves, drags and removes Parts, and keeps a Part of a kind it does not know as it is', async () => {
    const odd: Part = { kind: 'song', values: { verse: { text: 'la' } } }
    const lonely: Part = { kind: 'branch', values: { when: { text: 'fails_by' } } }
    const { model, get, wrapper } = editor(spell, [odd, lonely, { kind: 'light', values: { bright_ft: { number: 5 } }, groups: [] }])
    const order = () => model.map((p) => p.kind)
    expect(get('parts-0').find('.kind').text()).toBe('song')
    expect(get('parts-1').findAll('fieldset.group')).toHaveLength(0)
    await get('parts-0-down').trigger('click')
    expect(order()).toEqual(['branch', 'song', 'light'])
    await get('parts-0-up').trigger('click')
    await get('parts-2-down').trigger('click')
    expect(order()).toEqual(['branch', 'song', 'light'])
    await get('parts-2').trigger('dragstart')
    await get('parts-0').trigger('drop')
    expect(order()).toEqual(['light', 'branch', 'song'])
    await get('parts-1-remove').trigger('click')
    expect(model).toEqual([{ kind: 'light', values: { bright_ft: { number: 5 } }, groups: [] }, odd])
    expect(wrapper.get('[data-testid="parts"]').attributes('aria-label')).toBe('Parts')
  })

  it('offers no value for a choice an author may leave out', async () => {
    const mood: PartKind = {
      slug: 'mood', label: 'Mood', summary: 'A mood.', groups: [], openGroups: false, hints: { order: [], sections: [], help: [] },
      fields: [{ name: 'tone', label: 'Tone', type: 'choice', options: ['calm', 'wild'], required: false, min: 0, max: 0, scalable: false, formula: false }],
    }
    const { model, get, add } = editor({ kinds: ['mood'], beneath: [], fills: [], narrows: [] }, [], [mood])
    await add('parts', 'mood')
    expect(get('parts-0-tone').findAll('option').map((o) => o.text())).toEqual(['None', 'calm', 'wild'])
    await get('parts-0-tone').setValue('wild')
    await get('parts-0-tone').setValue('')
    expect(model).toEqual([{ kind: 'mood', values: {}, groups: [] }])
  })
})
