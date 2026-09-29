import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { ConditionChip, DieFace, EconomyPips, GButton, HitChance, HotbarSlot, ReactionTimer, TokenBadge } from '.'

afterEach(() => {
  document.body.innerHTML = ''
})

const attach = { attachTo: document.body }

describe('GButton', () => {
  it('renders each variant as a real button', async () => {
    for (const variant of ['primary', 'secondary', 'danger'] as const) {
      const w = mount(GButton, { ...attach, props: { variant }, slots: { default: 'Go' } })
      expect(w.get('button').classes()).toContain(`g-button--${variant}`)
      expect(w.get('button').attributes('type')).toBe('button')
      await expectAccessible(w.element as Element)
    }
  })

  it('defaults to secondary and can be disabled', () => {
    const w = mount(GButton, { props: { disabled: true, type: 'submit' }, slots: { default: 'Save' } })
    expect(w.get('button').classes()).toContain('g-button--secondary')
    expect(w.get('button').attributes('disabled')).toBeDefined()
    expect(w.get('button').attributes('type')).toBe('submit')
  })
})

describe('HotbarSlot', () => {
  it('names the action and emits use', async () => {
    const w = mount(HotbarSlot, { ...attach, props: { label: 'Longbow' } })
    expect(w.get('button').attributes('aria-label')).toBe('Longbow, action')
    await w.get('button').trigger('click')
    expect(w.emitted('use')).toHaveLength(1)
    await expectAccessible(w.element as Element)
  })

  it('marks bonus, suggested and unavailable slots', async () => {
    const bonus = mount(HotbarSlot, { props: { label: 'Mark', kind: 'bonus', suggested: true } })
    expect(bonus.get('button').attributes('aria-label')).toBe('Mark, bonus action, suggested')
    expect(bonus.text()).toContain('Suggested')
    const off = mount(HotbarSlot, { ...attach, props: { label: 'Dash', available: false, reason: 'No action left' } })
    expect(off.get('button').attributes('aria-label')).toBe('Dash, action, unavailable: No action left')
    expect(off.get('button').attributes('title')).toBe('No action left')
    expect(off.get('button').attributes('disabled')).toBeDefined()
    const noReason = mount(HotbarSlot, { props: { label: 'Hide', available: false } })
    expect(noReason.get('button').attributes('aria-label')).toBe('Hide, action, unavailable: not now')
    await expectAccessible(off.element as Element)
  })
})

describe('EconomyPips', () => {
  it('announces what is available and the movement left', async () => {
    const w = mount(EconomyPips, {
      ...attach,
      props: { action: true, bonus: false, reaction: true, movementLeft: 10, movementTotal: 30 },
    })
    expect(w.text()).toContain('Action available')
    expect(w.text()).toContain('Bonus used')
    expect(w.text()).toContain('Reaction available')
    expect(w.text()).toContain('10 / 30 ft')
    await expectAccessible(w.element as Element)
    const spent = mount(EconomyPips, { props: { action: false, bonus: true, reaction: false, movementLeft: 0, movementTotal: 30 } })
    expect(spent.text()).toContain('Action used')
    expect(spent.text()).toContain('Reaction used')
  })
})

describe('ConditionChip', () => {
  it('shows boons and banes with optional detail', async () => {
    const boon = mount(ConditionChip, { ...attach, props: { label: 'Blessed', detail: '7 rounds' } })
    expect(boon.text()).toBe('Blessed · 7 rounds')
    expect(boon.classes()).toContain('chip--boon')
    const bane = mount(ConditionChip, { props: { label: 'Prone', tone: 'bane' } })
    expect(bane.text()).toBe('Prone')
    expect(bane.classes()).toContain('chip--bane')
    await expectAccessible(boon.element as Element)
  })
})

describe('TokenBadge', () => {
  it('describes allegiance, state and initials', async () => {
    const w = mount(TokenBadge, { ...attach, props: { name: 'Aria  Vale', allegiance: 'party', active: true } })
    expect(w.attributes('aria-label')).toBe('Aria  Vale, ally, acting now')
    expect(w.text()).toBe('AV')
    expect(w.classes()).toEqual(expect.arrayContaining(['token--party', 'token--active']))
    await expectAccessible(w.element as Element)
    const enemy = mount(TokenBadge, { props: { name: 'Wight', allegiance: 'enemy', hidden: true, size: 60 } })
    expect(enemy.attributes('aria-label')).toBe('Wight, enemy, hidden')
    expect(enemy.attributes('style')).toContain('width: 60px')
  })

  it('uses an uploaded icon when there is one', () => {
    const w = mount(TokenBadge, { props: { name: 'Brom', allegiance: 'party', iconUrl: '/icons/brom.png' } })
    expect(w.get('img').attributes('src')).toBe('/icons/brom.png')
    expect(w.get('img').attributes('alt')).toBe('')
  })
})

describe('HitChance', () => {
  it('rounds the percentage and shows damage', async () => {
    const w = mount(HitChance, { ...attach, props: { percent: 91.6, damage: '1d8+4 piercing', detail: 'advantage' } })
    expect(w.text()).toContain('92%')
    expect(w.text()).toContain('1d8+4 piercing')
    await expectAccessible(w.element as Element)
  })
})

describe('ReactionTimer', () => {
  it('draws the remaining share and clamps odd input', async () => {
    const w = mount(ReactionTimer, { ...attach, props: { secondsLeft: 5, total: 10 } })
    expect(w.attributes('aria-label')).toBe('5 seconds left')
    const ring = w.findAll('circle')[1]
    expect(ring?.attributes('stroke-dasharray')).toBe('84.8 169.6')
    await expectAccessible(w.element as Element)
    const over = mount(ReactionTimer, { props: { secondsLeft: 20, total: 10 } })
    expect(over.findAll('circle')[1]?.attributes('stroke-dasharray')).toBe('169.6 169.6')
    const none = mount(ReactionTimer, { props: { secondsLeft: 3, total: 0 } })
    expect(none.findAll('circle')[1]?.attributes('stroke-dasharray')).toBe('0.0 169.6')
  })
})

describe('DieFace', () => {
  it('draws each die with its face and state', async () => {
    for (const sides of [4, 6, 8, 20] as const) {
      const w = mount(DieFace, { ...attach, props: { sides, value: 3 } })
      expect(w.attributes('aria-label')).toBe(`d${String(sides)} showing 3`)
      expect(w.get('text').text()).toBe('3')
      await expectAccessible(w.element as Element)
    }
    const unrolled = mount(DieFace, { props: { sides: 20, state: 'rolling' } })
    expect(unrolled.attributes('aria-label')).toBe('d20 not rolled yet, rolling')
    expect(unrolled.get('text').text()).toBe('?')
    expect(unrolled.classes()).toContain('die--rolling')
  })
})
