import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { expectAccessible } from '@/test/axe'
import { GAvatar, GField, GPicker, GRow, GTabs, type PickerOption } from '.'

afterEach(() => {
  document.body.innerHTML = ''
  vi.useRealTimers()
})

const attach = { attachTo: document.body }

describe('GField', () => {
  it('floats its label and judges the value only once the field is left', async () => {
    const w = mount(GField, { ...attach, props: { label: 'Name', required: true, modelValue: '' }, attrs: { 'data-testid': 'name', inputmode: 'text', class: 'grow', style: 'order: 2' } })
    const input = w.get('input')
    expect([input.attributes('data-testid'), input.attributes('inputmode'), w.attributes('data-testid'), input.attributes('class')]).toEqual(['name', 'text', undefined, undefined])
    expect([w.classes('grow'), w.attributes('style')]).toEqual([true, 'order: 2;'])
    expect(w.get('label').text()).toBe('Name')
    expect(input.attributes('placeholder')).toBe(' ')
    await input.setValue('')
    expect(w.find('[role="alert"]').exists()).toBe(false)
    await input.trigger('blur')
    expect(w.get('[role="alert"]').text()).toBe('Name is required.')
    expect(input.attributes('aria-invalid')).toBe('true')
    expect(input.attributes('aria-describedby')).toBe(w.get('[role="alert"]').attributes('id'))
    await input.setValue('Aria')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['Aria'])
    await w.setProps({ modelValue: 'Aria' })
    expect(w.find('[role="alert"]').exists()).toBe(false)
    await expectAccessible(w.element as Element)
  })

  it('runs its rules in order and shows the hint until something is wrong', async () => {
    const short = (v: string) => (v.length < 3 ? 'Too short.' : undefined)
    const noDigits = (v: string) => (/\d/.test(v) ? 'No digits.' : undefined)
    const w = mount(GField, { props: { label: 'Handle', hint: 'Shown to your party.', rules: [short, noDigits], modelValue: 'a1' } })
    expect(w.get('.g-float__hint').text()).toBe('Shown to your party.')
    await w.get('input').trigger('blur')
    expect(w.get('[role="alert"]').text()).toBe('Too short.')
    expect(w.find('.g-float__hint').exists()).toBe(false)
    await w.setProps({ modelValue: 'abc1' })
    expect(w.get('[role="alert"]').text()).toBe('No digits.')
    await w.setProps({ modelValue: 'abcd' })
    expect(w.find('[role="alert"]').exists()).toBe(false)
  })

  it('validates on demand, keeps numbers numeric and writes long text in a textarea', async () => {
    const w = mount(GField, { props: { label: 'Level', type: 'number', required: true, modelValue: '' } })
    expect((w.vm as unknown as { validate: () => boolean }).validate()).toBe(false)
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toBe('Level is required.')
    await w.get('input').setValue('7')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([7])
    await w.get('input').setValue('')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([''])
    await w.setProps({ modelValue: 7 })
    expect((w.vm as unknown as { validate: () => boolean }).validate()).toBe(true)
    const notes = mount(GField, { props: { label: 'Notes', multiline: true, modelValue: 'Hello' } })
    expect(notes.get('textarea').element.value).toBe('Hello')
    await notes.get('textarea').setValue('Hello there')
    expect(notes.emitted('update:modelValue')?.at(-1)).toEqual(['Hello there'])
    const story = mount(GField, { props: { label: 'Story', multiline: true, required: true, modelValue: '' } })
    await story.get('textarea').trigger('blur')
    expect(story.get('textarea').attributes('aria-invalid')).toBe('true')
  })
})

const spells: PickerOption[] = [
  { value: 'fireball', label: 'Fireball', hint: '3rd level' },
  { value: 'fire-bolt', label: 'Fire Bolt', hint: 'Cantrip' },
  { value: 'bless', label: 'Bless' },
]

describe('GPicker', () => {
  it('filters its options as you type and picks with the keyboard', async () => {
    const w = mount(GPicker, { ...attach, props: { label: 'Spell', options: spells, modelValue: '' } })
    const input = w.get('input')
    expect(input.attributes('role')).toBe('combobox')
    expect(input.attributes('aria-expanded')).toBe('false')
    await input.trigger('focus')
    expect(w.findAll('[role="option"]').map((o) => o.text())).toEqual(['Fireball3rd level', 'Fire BoltCantrip', 'Bless'])
    await input.setValue('FIRE')
    expect(w.findAll('[role="option"]').map((o) => o.get('.g-picker__label').text())).toEqual(['Fireball', 'Fire Bolt'])
    await input.trigger('keydown', { key: 'ArrowDown' })
    await input.trigger('keydown', { key: 'ArrowDown' })
    await input.trigger('keydown', { key: 'ArrowDown' })
    expect(input.attributes('aria-activedescendant')).toBe(w.findAll('[role="option"]')[0]?.attributes('id'))
    await input.trigger('keydown', { key: 'ArrowUp' })
    await input.trigger('keydown', { key: 'ArrowUp' })
    await input.trigger('keydown', { key: 'ArrowDown' })
    await input.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['fire-bolt'])
    expect(w.emitted('select')?.at(-1)).toEqual([spells[1]])
    expect(input.element.value).toBe('Fire Bolt')
    expect(input.attributes('aria-expanded')).toBe('false')
    await expectAccessible(w.element as Element)
  })

  it('says when nothing matches, closes on Escape and picks with a click', async () => {
    const w = mount(GPicker, { props: { label: 'Spell', options: spells, modelValue: 'bless' } })
    const input = w.get('input')
    expect(input.element.value).toBe('Bless')
    await input.setValue('wish')
    expect(w.get('.g-picker__empty').text()).toBe('Nothing matches “wish”.')
    await input.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('select')).toBeUndefined()
    await input.trigger('keydown', { key: 'Escape' })
    expect(w.find('[role="listbox"]').exists()).toBe(false)
    await input.trigger('keydown', { key: 'ArrowDown' })
    expect(w.find('[role="listbox"]').exists()).toBe(true)
    await input.setValue('')
    await w.findAll('[role="option"]')[0]?.trigger('mousedown')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['fireball'])
    await input.trigger('blur')
    expect(w.find('[role="listbox"]').exists()).toBe(false)
    await w.setProps({ modelValue: 'wish' })
    expect(input.element.value).toBe('Fireball')
    await input.setValue('bles')
    await input.trigger('keydown', { key: 'Enter' })
    expect(w.emitted('select')?.at(-1)).toEqual([spells[2]])
    await input.setValue('zzz')
    await input.trigger('keydown', { key: 'ArrowUp' })
    await input.trigger('keydown', { key: 'ArrowDown' })
    expect(input.attributes('aria-activedescendant')).toBeUndefined()
    await input.trigger('keydown', { key: 'a' })
  })

  it('asks the server once the typing settles, and keeps only the latest answer', async () => {
    vi.useFakeTimers()
    const answers: Record<string, (o: PickerOption[]) => void> = {}
    const search = vi.fn((q: string) => new Promise<PickerOption[]>((resolve) => (answers[q] = resolve)))
    const w = mount(GPicker, { props: { label: 'Monster', options: [], search, debounceMs: 250, modelValue: '' } })
    const input = w.get('input')
    await input.trigger('focus')
    await input.setValue('g')
    vi.advanceTimersByTime(300)
    expect(search).not.toHaveBeenCalled()
    await input.setValue('go')
    vi.advanceTimersByTime(100)
    await input.setValue('gob')
    vi.advanceTimersByTime(249)
    expect(search).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(search).toHaveBeenCalledTimes(1)
    expect(search).toHaveBeenLastCalledWith('gob')
    expect(w.get('.g-picker__empty').text()).toBe('Searching…')
    await input.setValue('gobl')
    vi.advanceTimersByTime(250)
    answers.gobl?.([{ value: 'goblin', label: 'Goblin' }])
    await flushPromises()
    answers.gob?.([{ value: 'gibberling', label: 'Gibberling' }])
    await flushPromises()
    expect(w.findAll('[role="option"]').map((o) => o.text())).toEqual(['Goblin'])
    await input.setValue('o')
    expect(w.findAll('[role="option"]')).toHaveLength(0)
  })

  it('shows a failed search instead of a stale list, and forgets a failure it no longer needs', async () => {
    vi.useFakeTimers()
    const fails: Record<string, () => void> = {}
    const search = (q: string) =>
      new Promise<PickerOption[]>((_, reject) => {
        fails[q] = () => {
          reject(new Error('down'))
        }
      })
    const w = mount(GPicker, { props: { label: 'Monster', options: [], search, debounceMs: 10, modelValue: '' } })
    await w.get('input').trigger('focus')
    await w.get('input').setValue('orc')
    vi.advanceTimersByTime(10)
    await w.get('input').setValue('ogre')
    vi.advanceTimersByTime(10)
    fails.orc?.()
    await flushPromises()
    expect(w.get('.g-picker__empty').text()).toBe('Searching…')
    fails.ogre?.()
    await flushPromises()
    expect(w.get('.g-picker__empty').text()).toBe('The search failed. Try again.')
    w.unmount()
  })
})

describe('GTabs', () => {
  it('underlines the chosen tab and moves with the arrow keys', async () => {
    const tabs = [
      { value: 'spells', label: 'Spells', count: 12 },
      { value: 'items', label: 'Items' },
      { value: 'feats', label: 'Feats' },
    ]
    const w = mount(GTabs, { ...attach, props: { label: 'Compendium', tabs, modelValue: 'items' }, slots: { default: '<p>Panel</p>' } })
    const buttons = w.findAll('[role="tab"]')
    expect(buttons.map((b) => b.attributes('aria-selected'))).toEqual(['false', 'true', 'false'])
    expect(buttons.map((b) => b.attributes('tabindex'))).toEqual(['-1', '0', '-1'])
    expect(buttons[0]?.text()).toBe('Spells12')
    expect(w.get('[role="tabpanel"]').attributes('aria-labelledby')).toBe(buttons[1]?.attributes('id'))
    for (const [key, want] of [['ArrowRight', 'feats'], ['ArrowLeft', 'items'], ['End', 'feats'], ['Home', 'spells'], ['ArrowLeft', 'feats'], ['ArrowRight', 'spells']] as const) {
      await buttons[1]?.trigger('keydown', { key })
      expect(w.emitted('update:modelValue')?.at(-1)).toEqual([want])
    }
    expect(document.activeElement).toBe(buttons[0]?.element)
    const sent = w.emitted('update:modelValue')?.length
    await buttons[0]?.trigger('keydown', { key: 'Tab' })
    expect(w.emitted('update:modelValue')?.length).toBe(sent)
    await buttons[1]?.trigger('click')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual(['items'])
    await expectAccessible(w.element as Element)
  })
})

describe('GRow and GAvatar', () => {
  it('links a row with a chevron, or makes it a button', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<p />' } }, { path: '/npcs/:id', name: 'npc', component: { template: '<p />' } }] })
    const link = mount(GRow, { ...attach, global: { plugins: [router] }, props: { title: 'Tamsin', subtitle: 'Innkeeper', to: '/npcs/1' }, slots: { trailing: '<span>Ally</span>' } })
    expect(link.get('a').attributes('href')).toBe('/npcs/1')
    expect(link.text()).toBe('TamsinInnkeeperAlly')
    expect(link.find('.g-row__chevron').attributes('aria-hidden')).toBe('true')
    await link.get('a').trigger('click')
    expect(link.emitted('click')).toBeUndefined()
    await expectAccessible(link.element as Element)
    const button = mount(GRow, { props: { title: 'Roll loot' } })
    await button.get('button').trigger('click')
    expect(button.emitted('click')).toHaveLength(1)
    expect(button.find('.g-row__subtitle').exists()).toBe(false)
  })

  it('shows a portrait or the initials of a name', async () => {
    const pic = mount(GAvatar, { ...attach, props: { name: 'Aria Vale', src: '/a.png', size: 56 } })
    expect(pic.get('img').attributes('alt')).toBe('Aria Vale')
    expect(pic.get('.g-avatar').attributes('style')).toContain('56px')
    await expectAccessible(pic.element as Element)
    const initials = mount(GAvatar, { props: { name: 'brom of the hills' } })
    expect(initials.get('[role="img"]').attributes('aria-label')).toBe('brom of the hills')
    expect(initials.text()).toBe('BO')
    expect(mount(GAvatar, { props: { name: '  ' } }).text()).toBe('?')
  })
})
