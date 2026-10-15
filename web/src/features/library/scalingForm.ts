import type { PartField, PartScaling, PartValue } from '@/infrastructure/api/types.gen'
import { zPartScaling } from '@/infrastructure/api/zod.gen'

export type Axis = PartScaling['axis']
export type Rule = PartScaling['rule']

/** The axes Scaling grows along, as the contract names them. */
export const axes: Axis[] = zPartScaling.shape.axis.options

/** Whether an axis names a class: a class level, or a class table. */
export const classed = (axis: Axis) => axis === 'class_level' || axis === 'table_column'

/** The rules a field can grow by along an axis: a class table only by its column; a flag or a choice only by a table. */
export function rulesFor(field: PartField | undefined, axis: Axis): Rule[] {
  if (field === undefined || (field.type !== 'dice' && field.type !== 'number')) return ['table']
  if (axis === 'table_column') return ['column']
  return field.type === 'number' && field.formula ? ['linear', 'table', 'formula'] : ['linear', 'table']
}

/** A field's first value: one die, its least number, no flag, or its first option. */
export function firstValue(field: PartField): PartValue {
  if (field.type === 'dice') return { text: '1d6' }
  if (field.type === 'number') return { number: Math.max(field.min, 1) }
  if (field.type === 'choice') return { text: field.options[0] ?? '' }
  return {}
}

/** A rule over a field along an axis, with the rule's own values at their first. */
export function freshRule(field: PartField, axis: Axis, rule: Rule, cls?: string): PartScaling {
  const out: PartScaling = { field: field.name, axis, rule }
  if (classed(axis)) out.class = cls ?? ''
  if (rule === 'linear') Object.assign(out, { base: 1, per: field.type === 'dice' ? { text: '1d6' } : { number: 1 } })
  if (rule === 'table') out.steps = [{ at: 2, value: firstValue(field) }]
  if (rule === 'formula') out.formula = 'level'
  if (rule === 'column') out.column = ''
  return out
}

/** A rule moved to another axis: its class kept where the axis has one, its rule kept where the field still takes it. */
export function onAxis(s: PartScaling, field: PartField, axis: Axis): PartScaling {
  const rules = rulesFor(field, axis)
  if (!rules.includes(s.rule)) return freshRule(field, axis, rules[0] ?? 'table', s.class)
  const { class: cls, ...rest } = s
  return classed(axis) ? { ...rest, axis, class: cls ?? '' } : { ...rest, axis }
}
