import { describe, expect, it } from 'vitest'
import type { ConditionDesign, Part } from '@/infrastructure/api/types.gen'
import { conditionDesign, conditionForm, spellDesign, spellForm, type SpellForm } from './partRows'

const part = (kind: string, values: Part['values'] = {}, groups: Part['groups'] = []): Part => ({ kind, values, groups })

describe('builder rows', () => {
  it('reads every condition row a Part can show, keeps the rest, and writes them back as they were', () => {
    const parts: Part[] = [
      part('edge', { against: { flag: false }, advantage: { flag: true }, range: { text: 'any' } }),
      part('edge', { against: { flag: false }, advantage: { flag: false }, range: { text: 'any' } }),
      part('edge', { against: { flag: true }, advantage: { flag: true }, range: { text: 'any' } }),
      part('edge', { against: { flag: true }, advantage: { flag: false }, range: { text: 'any' } }),
      part('save_edge', { ability: { text: 'wisdom' }, mode: { text: 'advantage' } }),
      part('save_edge', { ability: { text: 'wisdom' }, mode: { text: 'disadvantage' } }),
      part('save_edge', { ability: { text: 'wisdom' }, mode: { text: 'fail' } }),
      part('incapacitated'), part('immobile'),
      part('speed_penalty', { ft: { number: 10 } }), part('crit_within', { feet: { number: 5 } }),
      part('manual', { instruction: { text: 'Shiver.' } }),
      part('edge', { against: { flag: true }, advantage: { flag: true }, range: { text: 'within_five' } }),
      part('bonus_die', { dice: { text: '1d4' } }),
    ]
    const design: ConditionDesign = { icon: 'snow', color: '#7fa8dd', text: '', ends: 'rest', perLevel: { d20: 0, speedFt: 0, deathAt: 0 }, parts }
    const form = conditionForm(design)
    expect(form.parts.map((r) => r.type)).toEqual([
      'attack_advantage', 'attack_disadvantage', 'attacked_advantage', 'attacked_disadvantage', 'save_advantage', 'save_disadvantage', 'save_fails',
      'incapacitated', 'immobile', 'speed_penalty', 'crit_within', 'manual', 'kept', 'kept',
    ])
    expect(conditionDesign(form)).toEqual(design)
    expect(conditionForm({ ...design, parts: [part('edge'), part('save_edge'), part('speed_penalty'), part('crit_within'), part('manual')] }).parts).toEqual([
      { type: 'kept', part: part('edge') }, { type: 'kept', part: part('save_edge') }, { type: 'speed_penalty', feet: 0 }, { type: 'crit_within', feet: 0 }, { type: 'manual', text: '' },
    ])
    expect(conditionDesign({ ...design, parts: [{ type: 'save_fails' }, { type: 'speed_penalty' }, { type: 'kept' }] }).parts).toEqual([
      part('save_edge', { ability: { text: '' }, mode: { text: 'fail' } }), part('speed_penalty', { ft: { number: 0 } }), part('kept'),
    ])
  })

  it('writes spell rows as Parts and reads them back, with unfilled rows written empty', () => {
    const base: SpellForm = {
      targeting: { shape: 'sphere', sizeFt: 20, rangeFt: 0 }, concentration: false, duration: { unit: 'instant' }, ritual: false,
      castingTime: { kind: 'action' }, components: { verbal: true, somatic: false }, parts: [],
    }
    const bare = spellDesign({
      ...base,
      parts: [{ type: 'damage' }, { type: 'condition' }, { type: 'light' }, { type: 'reveal' }, { type: 'surface' }, { type: 'manual' }, { type: 'kept' }, { type: 'damage', when: 'start_of_turn' }],
    })
    expect(bare.turnDamage).toEqual({ dice: '', damageType: '' })
    expect(bare.parts.map((p) => p.kind)).toEqual(['save_damage', 'branch', 'light', 'reveal', 'create_surface', 'manual'])
    expect(bare.parts[0]?.values).toEqual({ dice: { text: '' }, damage_type: { text: '' } })
    const back = spellForm({ ...bare, turnDamage: undefined })
    expect(back.parts.map((r) => r.type)).toEqual(['damage', 'condition', 'light', 'reveal', 'surface', 'manual'])
    expect(spellForm({ ...bare, parts: [part('save_damage'), part('light'), part('reveal'), part('create_surface'), part('manual')] }).parts).toEqual([
      { type: 'damage', when: 'on_cast', dice: '', damageType: '', half: false }, { type: 'light', brightFt: 0, dimFt: 0 }, { type: 'reveal', qualities: [] },
      { type: 'surface', surface: '', rounds: 0 }, { type: 'manual', text: '' },
      { type: 'damage', when: 'start_of_turn', dice: '', damageType: '', half: false },
    ])
    const charm = (creature?: string, condition = 'charmed'): Part => part('branch', { when: { text: 'creature_is' }, ...(creature ? { creature_type: { text: creature } } : {}) }, [{
      name: 'then', parts: [part('branch', { when: { text: 'fails_by' }, n: { number: 1 } }, [{ name: 'then', parts: [part('save_condition', { condition: { text: condition } })] }])],
    }])
    const rows = spellForm({ ...bare, turnDamage: undefined, parts: [charm('undead'), charm('fey', 'frightened'), charm(), part('branch', { when: { text: 'creature_is' } })] }).parts
    expect(rows).toEqual([
      { type: 'condition', condition: 'charmed', onlyTypes: ['undead'] }, { type: 'condition', condition: 'frightened', onlyTypes: ['fey'] },
      { type: 'kept', part: charm() }, { type: 'kept', part: part('branch', { when: { text: 'creature_is' } }) },
    ])
  })
})
