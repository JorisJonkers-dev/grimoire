import type { ConditionDesign, Part, PartGroup, PartValue, SpellDesign } from '@/infrastructure/api/types.gen'

// The builders' hand-made rows, read from and written as Parts of the kinds the rules know. A Part no row
// can show is kept as it is.

export type SpellRowType = 'damage' | 'condition' | 'light' | 'reveal' | 'surface' | 'manual'

export type SpellRow = {
  type: SpellRowType | 'kept'
  when?: 'on_cast' | 'start_of_turn'
  dice?: string
  damageType?: string
  half?: boolean
  condition?: string
  onlyTypes?: string[]
  brightFt?: number
  dimFt?: number
  qualities?: string[]
  surface?: string
  rounds?: number
  text?: string
  part?: Part
}

export type SpellForm = Omit<SpellDesign, 'parts' | 'turnDamage'> & { parts: SpellRow[] }

export type ConditionRow = { type: string; ability?: string; feet?: number; text?: string; part?: Part }

export type ConditionForm = Omit<ConditionDesign, 'parts'> & { parts: ConditionRow[] }

const text = (s: string | undefined): PartValue => ({ text: s ?? '' })
const number = (n: number | undefined): PartValue => ({ number: n ?? 0 })
const flag = (b: boolean): PartValue => ({ flag: b })
const part = (kind: string, values: Record<string, PartValue>, groups: PartGroup[] = []): Part => ({ kind, values, groups })
const then = (p: Part): PartGroup[] => [{ name: 'then', parts: [p] }]

// failed puts a condition on a creature that fails the spell's save.
const failed = (condition: string | undefined): Part =>
  part('branch', { when: text('fails_by'), n: number(1) }, then(part('save_condition', { condition: text(condition) })))

function spellParts(r: SpellRow): Part[] {
  switch (r.type) {
    case 'damage':
      return [part('save_damage', { dice: text(r.dice), damage_type: text(r.damageType), ...(r.half ? { half: flag(true) } : {}) })]
    case 'condition': {
      const types = r.onlyTypes ?? []
      if (types.length === 0) return [failed(r.condition)]
      return types.map((t) => part('branch', { when: text('creature_is'), creature_type: text(t) }, then(failed(r.condition))))
    }
    case 'light':
      return [part('light', { bright_ft: number(r.brightFt), dim_ft: number(r.dimFt) })]
    case 'reveal':
      return [part('reveal', { qualities: { texts: r.qualities ?? [] } })]
    case 'surface':
      return [part('create_surface', { surface: text(r.surface), rounds: number(r.rounds) })]
    case 'manual':
      return [part('manual', { instruction: text(r.text) })]
    case 'kept':
      return r.part ? [r.part] : []
  }
}

/** The design a form's rows make: start-of-turn damage is the spell's own, every other row its Parts. */
export function spellDesign(form: SpellForm): SpellDesign {
  const { parts: rows, ...rest } = form
  const turn = rows.find((r) => r.type === 'damage' && r.when === 'start_of_turn')
  const parts = rows.filter((r) => r !== turn).flatMap(spellParts)
  return turn ? { ...rest, turnDamage: { dice: turn.dice ?? '', damageType: turn.damageType ?? '' }, parts } : { ...rest, parts }
}

const only = (p: Part): Part | undefined => (p.groups?.length === 1 && p.groups[0]?.name === 'then' && p.groups[0].parts.length === 1 ? p.groups[0].parts[0] : undefined)

// failedCondition is the condition a branch puts on a creature that fails the save, when that is all it does.
function failedCondition(p: Part | undefined): string | undefined {
  const inner = p && only(p)
  if (!p || p.kind !== 'branch' || p.values.when?.text !== 'fails_by' || p.values.n?.number !== 1 || inner?.kind !== 'save_condition') return undefined
  return inner.values.condition?.text
}

function spellRow(p: Part, rows: SpellRow[]): SpellRow | undefined {
  const v = p.values
  switch (p.kind) {
    case 'save_damage':
      return { type: 'damage', when: 'on_cast', dice: v.dice?.text ?? '', damageType: v.damage_type?.text ?? '', half: v.half?.flag ?? false }
    case 'light':
      return { type: 'light', brightFt: v.bright_ft?.number ?? 0, dimFt: v.dim_ft?.number ?? 0 }
    case 'reveal':
      return { type: 'reveal', qualities: v.qualities?.texts ?? [] }
    case 'create_surface':
      return { type: 'surface', surface: v.surface?.text ?? '', rounds: v.rounds?.number ?? 0 }
    case 'manual':
      return { type: 'manual', text: v.instruction?.text ?? '' }
  }
  const condition = failedCondition(p)
  if (condition !== undefined) return { type: 'condition', condition, onlyTypes: [] }
  const typed = p.kind === 'branch' && v.when?.text === 'creature_is' ? failedCondition(only(p)) : undefined
  const creature = v.creature_type?.text
  if (typed === undefined || creature === undefined) return { type: 'kept', part: p }
  const last = rows.at(-1)
  if (last?.type === 'condition' && last.condition === typed && (last.onlyTypes?.length ?? 0) > 0) {
    last.onlyTypes?.push(creature)
    return undefined
  }
  return { type: 'condition', condition: typed, onlyTypes: [creature] }
}

/** The rows a design shows: one for each Part a row can show, and its start-of-turn damage last. */
export function spellForm(design: SpellDesign): SpellForm {
  const { parts, turnDamage, ...rest } = design
  const rows: SpellRow[] = []
  for (const p of parts) {
    const row = spellRow(p, rows)
    if (row) rows.push(row)
  }
  if (turnDamage) rows.push({ type: 'damage', when: 'start_of_turn', dice: turnDamage.dice, damageType: turnDamage.damageType, half: false })
  return { ...rest, parts: rows }
}

const edges: Record<string, [boolean, boolean]> = {
  attack_advantage: [false, true],
  attack_disadvantage: [false, false],
  attacked_advantage: [true, true],
  attacked_disadvantage: [true, false],
}
const saveModes: Record<string, string> = { save_advantage: 'advantage', save_disadvantage: 'disadvantage', save_fails: 'fail' }

function conditionPart(r: ConditionRow): Part {
  if (r.type === 'kept' && r.part) return r.part
  const edge = edges[r.type]
  if (edge) return part('edge', { against: flag(edge[0]), advantage: flag(edge[1]), range: text('any') })
  const mode = saveModes[r.type]
  if (mode) return part('save_edge', { ability: text(r.ability), mode: text(mode) })
  switch (r.type) {
    case 'speed_penalty':
      return part('speed_penalty', { ft: number(r.feet) })
    case 'crit_within':
      return part('crit_within', { feet: number(r.feet) })
    case 'manual':
      return part('manual', { instruction: text(r.text) })
  }
  return part(r.type, {})
}

function conditionRow(p: Part): ConditionRow {
  const v = p.values
  if (p.kind === 'edge' && v.range?.text === 'any' && !v.source_only?.flag) {
    const against = v.against?.flag ?? false
    const advantage = v.advantage?.flag ?? false
    return { type: `${against ? 'attacked' : 'attack'}_${advantage ? 'advantage' : 'disadvantage'}` }
  }
  const mode = Object.entries(saveModes).find(([, m]) => m === v.mode?.text)
  if (p.kind === 'save_edge' && mode) return { type: mode[0], ability: v.ability?.text ?? '' }
  switch (p.kind) {
    case 'incapacitated':
    case 'immobile':
      return { type: p.kind }
    case 'speed_penalty':
      return { type: p.kind, feet: v.ft?.number ?? 0 }
    case 'crit_within':
      return { type: p.kind, feet: v.feet?.number ?? 0 }
    case 'manual':
      return { type: p.kind, text: v.instruction?.text ?? '' }
  }
  return { type: 'kept', part: p }
}

/** The rows a condition's design shows. */
export function conditionForm(design: ConditionDesign): ConditionForm {
  return { ...design, parts: design.parts.map(conditionRow) }
}

/** The design a condition form's rows make. */
export function conditionDesign(form: ConditionForm): ConditionDesign {
  return { ...form, parts: form.parts.map(conditionPart) }
}
