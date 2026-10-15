import type { Part, PartField, PartGroup, PartHolds, PartKind, PartValue } from '@/infrastructure/api/types.gen'

/** Fields a form draws together; a label makes them a named section. */
export type Shown = { label?: string; fields: PartField[] }

/** The fields a form shows for a kind: in its hinted order, without those the builder fills itself, gathered into its sections. */
export function shownFields(kind: PartKind, fills: string[]): Shown[] {
  const { order, sections } = kind.hints
  const rank = (f: PartField) => (order.includes(f.name) ? order.indexOf(f.name) : order.length + kind.fields.indexOf(f))
  const out: Shown[] = []
  for (const f of kind.fields.filter((x) => !fills.includes(x.name)).sort((a, b) => rank(a) - rank(b))) {
    const label = sections.find((s) => s.fields.includes(f.name))?.label
    const at = label === undefined ? undefined : out.find((s) => s.label === label)
    if (at) at.fields.push(f)
    else out.push({ label, fields: [f] })
  }
  return out
}

/** A field's line of help, if its kind gives one. */
export const helpOf = (kind: PartKind, field: string) => kind.hints.help.find((h) => h.field === field)?.text

/** The list a builder narrows a text field to, if it does. */
export const narrowed = (holds: PartHolds, kind: string, field: string) => holds.narrows.find((n) => n.kind === kind && n.field === field)?.options

/** A new Part of a kind: each required choice at its first value, each required number at its least, and its named groups empty. */
export function freshPart(kind: PartKind, holds: PartHolds): Part {
  const values: Record<string, PartValue> = {}
  for (const f of kind.fields.filter((x) => x.required && !holds.fills.includes(x.name))) {
    const first = (narrowed(holds, kind.slug, f.name) ?? f.options)[0]
    if (f.type === 'number') values[f.name] = { number: f.min }
    else if (first !== undefined && f.type !== 'choices') values[f.name] = { text: first }
  }
  return { kind: kind.slug, values, groups: kind.groups.map((name) => ({ name, parts: [] })) }
}

/** A Part's own named groups, in the order its kind names them. */
export const namedGroups = (part: Part, kind: PartKind): PartGroup[] =>
  kind.groups.flatMap((name) => part.groups?.filter((g) => g.name === name) ?? [])

/** Sets one value; an empty one is left out, as the server reads an absent value. */
export function setValue(part: Part, field: string, value: PartValue | undefined) {
  const rest = Object.fromEntries(Object.entries(part.values).filter(([name]) => name !== field))
  part.values = value === undefined ? rest : { ...rest, [field]: value }
}

export const textValue = (s: string): PartValue | undefined => (s === '' ? undefined : { text: s })
export const numberValue = (s: string): PartValue | undefined => (s === '' ? undefined : { number: Number(s) })
export const flagValue = (on: boolean): PartValue | undefined => (on ? { flag: true } : undefined)

/** Several choices with one more or one fewer. */
export function toggled(value: PartValue | undefined, option: string): PartValue | undefined {
  const now = value?.texts ?? []
  const next = now.includes(option) ? now.filter((t) => t !== option) : [...now, option]
  return next.length === 0 ? undefined : { texts: next }
}

/** A slug as words: fails_by reads fails by. */
export const words = (s: string) => s.replaceAll('_', ' ')

/** A group's name as a heading: then reads Then. */
export const heading = (s: string) => words(s).charAt(0).toUpperCase() + words(s).slice(1)
