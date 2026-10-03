import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { MonsterBuild, MonsterDesign } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const MON = '0190c7a8-0000-7000-8000-0000000000a9'
const at = '2026-10-03T20:00:00Z'
const start = (): MonsterDesign => ({
  size: 'medium', creatureType: 'beast', ac: 12, hp: 11, speedFt: 30, challenge: 0.5,
  abilities: { strength: 12, dexterity: 12, constitution: 12, intelligence: 3, wisdom: 10, charisma: 6 },
  saves: [], senses: [], resistances: [], immunities: [], vulnerabilities: [], threshold: 0, traits: [], multiattack: 0, phases: [],
  actions: [{ name: 'Bite', kind: 'melee', toHit: 3, reachFt: 5, damage: '1d6', damageBonus: 1, damageType: 'piercing' }],
})
const entry = { id: MON, kind: 'creature' as const, name: 'Bog King', fields: [], revision: 1, createdAt: at, updatedAt: at }
const build = (design: MonsterDesign, extra: Partial<MonsterBuild> = {}): MonsterBuild => ({ entry, design, slug: 'hb-m', lines: ['Medium Beast'], estimate: '1/2', ...extra })

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

describe('monster builder', () => {
  it('builds the Bog King with legendary and lair actions and a phase, previews its Challenge and saves it', async () => {
    const sent: { method: string; body: unknown }[] = []
    const { wrapper } = await mountApp(`/library/${MON}/monster`, {
      '/api/v1/builders/monsters/preview': async (_u, req) => {
        const body = (await req.clone().json()) as { name: string; design: MonsterDesign }
        sent.push({ method: 'PREVIEW', body })
        return build(body.design, { lines: ['Large Monstrosity', 'Legendary Actions (3 a round).'], estimate: '11' })
      },
      [`/api/v1/builders/monsters/${MON}`]: async (_u, req) => {
        if (req.method === 'PUT') {
          const body = (await req.clone().json()) as MonsterDesign
          sent.push({ method: 'PUT', body })
          return build(body, { entry: { ...entry, revision: 2 } })
        }
        return build(start())
      },
    })
    expect(wrapper.get('[data-testid="monster-estimate"]').text()).toBe('Estimated Challenge 1/2')
    await expectAccessible(wrapper.element as Element)
    const set = (id: string, value: string | number | boolean) => wrapper.get(`[data-testid="${id}"]`).setValue(value)
    const click = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('click')
    const change = (id: string) => wrapper.get(`[data-testid="${id}"]`).trigger('change')
    await set('monster-size', 'large')
    await set('monster-type', 'monstrosity')
    await set('monster-ac', 16)
    await set('monster-hp', 120)
    await set('monster-speed', 30)
    await set('monster-challenge', 8)
    await set('monster-threshold', 5)
    await set('monster-multiattack', 2)
    await set('monster-swarm', true)
    await set('monster-swarm', false)
    await set('monster-strength', 20)
    await change('monster-save-constitution')
    await change('monster-save-wisdom')
    await change('monster-save-wisdom')
    await change('monster-resistances-poison')
    await change('monster-immunities-acid')
    await change('monster-vulnerabilities-fire')
    await click('monster-add-sense')
    await set('monster-sense-0-feet', 90)
    await click('monster-add-sense')
    await click('monster-sense-1-remove')
    await click('monster-add-trait')
    await set('monster-trait-0-name', 'Bog Stride')
    await set('monster-trait-0-text', 'Mud costs it no extra movement.')
    await click('monster-add-trait')
    await click('monster-trait-1-remove')
    await set('monster-aura', true)
    await set('monster-aura-name', 'Stench')
    await set('monster-aura-feet', 10)
    await set('monster-aura-text', 'Poisoned.')
    await set('monster-aura', false)
    await set('monster-aura', true)
    await set('monster-action-0-hit', 8)
    await set('monster-action-0-reach', 10)
    await set('monster-action-0-damage', '2d8')
    await set('monster-action-0-bonus', 5)
    await set('monster-action-0-type', 'slashing')
    await click('monster-add-action')
    await set('monster-action-1-name', 'Mud Bolt')
    await set('monster-action-1-kind', 'ranged')
    await set('monster-action-1-hit', 6)
    await set('monster-action-1-range', 60)
    await set('monster-action-1-long', 120)
    await set('monster-action-1-damage', '3d6')
    await set('monster-action-1-type', 'acid')
    await click('monster-add-action')
    await set('monster-action-2-name', 'Bog Breath')
    await set('monster-action-2-kind', 'save')
    await set('monster-action-2-save', 'constitution')
    await set('monster-action-2-dc', 15)
    await set('monster-action-2-recharge', 5)
    await set('monster-action-2-text', '15-foot cone.')
    await click('monster-add-action')
    await click('monster-action-3-remove')
    await set('monster-legendary', true)
    await set('monster-legendary-uses', 3)
    await set('monster-legendary-resistance', 2)
    await set('monster-legendary-0-name', 'Tail Sweep')
    await click('monster-add-legendary')
    await set('monster-legendary-1-name', 'Sink')
    await set('monster-legendary-1-cost', 2)
    await set('monster-legendary-1-text', 'It sinks.')
    await click('monster-add-legendary')
    await click('monster-legendary-2-remove')
    await set('monster-lair', true)
    await set('monster-lair-0-name', 'Rising Water')
    await set('monster-lair-0-text', 'The water rises.')
    await click('monster-add-lair')
    await click('monster-lair-1-remove')
    await click('monster-add-regional')
    await set('monster-regional-0', 'Fogs never lift.')
    await click('monster-add-regional')
    await click('monster-regional-1-remove')
    await click('monster-add-phase')
    await set('monster-phase-0-name', 'Drowned King')
    await set('monster-phase-0-hp', 60)
    await set('monster-phase-0-text', 'It rises.')
    await click('monster-add-phase')
    await click('monster-phase-1-remove')
    await click('monster-preview')
    await flushPromises()
    expect(wrapper.get('[data-testid="monster-estimate"]').text()).toBe('Estimated Challenge 11')
    await wrapper.get('[data-testid="monster-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="monster-status"]').text()).toBe('Saved as Revision 2.')
    const saved = sent.find((x) => x.method === 'PUT')?.body as MonsterDesign
    expect(saved).toMatchObject({
      size: 'large', creatureType: 'monstrosity', ac: 16, hp: 120, challenge: 8, threshold: 5, multiattack: 2, swarm: false,
      saves: ['constitution'], resistances: ['poison'], immunities: ['acid'], vulnerabilities: ['fire'], senses: [{ kind: 'darkvision', feet: 90 }],
      traits: [{ name: 'Bog Stride', text: 'Mud costs it no extra movement.' }], aura: { name: 'Aura', feet: 10, text: '' },
      legendary: { uses: 3, resistance: 2, actions: [{ name: 'Tail Sweep', cost: 1, text: 'It makes one attack.' }, { name: 'Sink', cost: 2, text: 'It sinks.' }] },
      lair: { actions: [{ name: 'Rising Water', text: 'The water rises.' }], regional: ['Fogs never lift.'] },
      phases: [{ name: 'Drowned King', hp: 60, text: 'It rises.' }],
    })
    expect(saved.abilities.strength).toBe(20)
    expect(saved.actions.map((a) => a.name)).toEqual(['Bite', 'Mud Bolt', 'Bog Breath'])
    expect(saved.actions[2]).toMatchObject({ kind: 'save', saveAbility: 'constitution', dc: 15, recharge: 5 })
  })

  it('shows a Shared Library copy, reports a design that will not build, and opens from the entry', async () => {
    const { wrapper } = await mountApp(`/library/${MON}/monster`, {
      '/api/v1/builders/monsters/preview': () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422, detail: 'give it 1 to 20 actions' }, 422),
      [`/api/v1/builders/monsters/${MON}`]: () => build(start(), { entry: { ...entry, shared: true } }),
    })
    expect(wrapper.get('[data-testid="monster-readonly"]').text()).toContain('Shared Library copy')
    expect(wrapper.find('[data-testid="monster-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="monster-legendary"]').setValue(true)
    await wrapper.get('[data-testid="monster-legendary"]').setValue(false)
    await wrapper.get('[data-testid="monster-lair"]').setValue(true)
    await wrapper.get('[data-testid="monster-lair"]').setValue(false)
    await wrapper.get('[data-testid="monster-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="monster-problem"]').text()).toBe('give it 1 to 20 actions')
    const refused = await mountApp(`/library/${MON}/monster`, {
      [`/api/v1/builders/monsters/${MON}`]: () => jsonResponse({ type: 'about:blank', title: 'Not allowed', status: 422 }, 422),
    })
    expect(refused.wrapper.get('[data-testid="monster-error"]').text()).toContain('cannot be opened')
    const fromEntry = await mountApp(`/library/${MON}`, {
      '/api/v1/campaigns': () => ({ items: [] }),
      [`/api/v1/builders/monsters/${MON}`]: () => build(start()),
      [`/api/v1/library/${MON}`]: () => ({ entry, revisions: [], uses: [] }),
    })
    await fromEntry.wrapper.get('[data-testid="open-monster-builder"]').trigger('click')
    await flushPromises()
    expect(fromEntry.wrapper.get('h1').text()).toContain('Monster builder')
  })
})
