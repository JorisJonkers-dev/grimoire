import { describe, expect, it } from 'vitest'
import type { PartField } from '@/infrastructure/api/types.gen'
import { firstValue, freshRule, onAxis, rulesFor } from './scalingForm'

const field = (type: PartField['type'], extra: Partial<PartField> = {}): PartField => ({
  name: 'f', label: 'F', type, options: [], required: false, min: 0, max: 100, scalable: true, formula: false, ...extra,
})

describe('the rules a field can grow by', () => {
  it('offers each field only what its type takes', () => {
    expect(rulesFor(undefined, 'slot_level')).toEqual(['table'])
    expect(rulesFor(field('choice'), 'table_column')).toEqual(['table'])
    expect(rulesFor(field('number'), 'table_column')).toEqual(['column'])
    expect(rulesFor(field('number'), 'charges')).toEqual(['linear', 'table'])
    expect(rulesFor(field('number', { formula: true }), 'charges')).toEqual(['linear', 'table', 'formula'])
  })

  it('starts each value at its first', () => {
    expect(firstValue(field('number'))).toEqual({ number: 1 })
    expect(firstValue(field('number', { min: 5 }))).toEqual({ number: 5 })
    expect(firstValue(field('choice'))).toEqual({ text: '' })
    expect(firstValue(field('text'))).toEqual({})
    expect(freshRule(field('number'), 'class_level', 'column')).toEqual({ field: 'f', axis: 'class_level', class: '', rule: 'column', column: '' })
  })

  it('keeps a rule and drops its class when it moves to an axis without one', () => {
    const s = freshRule(field('number'), 'class_level', 'linear', 'cleric')
    expect(onAxis(s, field('number'), 'slot_level')).toEqual({ field: 'f', axis: 'slot_level', rule: 'linear', base: 1, per: { number: 1 } })
    expect(onAxis({ field: 'f', axis: 'slot_level', rule: 'table' }, field('number'), 'class_level')).toEqual({ field: 'f', axis: 'class_level', rule: 'table', class: '' })
  })
})
