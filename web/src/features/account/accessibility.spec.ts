import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { still, vibrating } from '@/test/haptics'
import { mountApp, unmountAll } from '@/test/mountApp'
import { buzz, reducedMotion } from '@/shared/a11y/settings'

const KEY = 'grimoire.accessibility'
const root = () => document.documentElement.dataset
const applied = () => ({ palette: root().palette, text: root().text, font: root().font, motion: root().motion })
const kept = () => JSON.parse(localStorage.getItem(KEY) ?? 'null') as Record<string, unknown> | null
const systemMotion = (reduce: boolean) => vi.stubGlobal('matchMedia', (query: string) => ({ matches: reduce && query === '(prefers-reduced-motion: reduce)' }))

beforeEach(() => {
  localStorage.clear()
})
afterEach(() => {
  unmountAll()
  still()
  vi.unstubAllGlobals()
  localStorage.clear()
})

describe('accessibility settings', () => {
  it('starts from the standard look, and is reached from the foot of every page', async () => {
    const { wrapper, router } = await mountApp('/', {})
    expect(applied()).toEqual({ palette: 'standard', text: 'normal', font: 'standard', motion: 'system' })
    expect(wrapper.get('[data-testid="accessibility-link"]').attributes('href')).toBe('/accessibility')
    await router.push('/accessibility')
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Accessibility')
    expect((wrapper.get('[data-testid="palette-standard"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-testid="text-normal"]').element as HTMLInputElement).checked).toBe(true)
    for (const id of ['dyslexia-font', 'reduce-motion']) expect((wrapper.get(`[data-testid="${id}"]`).element as HTMLInputElement).checked, id).toBe(false)
    for (const id of ['haptics', 'announce-turns', 'announce-rolls']) expect((wrapper.get(`[data-testid="${id}"]`).element as HTMLInputElement).checked, id).toBe(true)
    expect(kept()).toBeNull()
    await expectAccessible(wrapper.element as Element)
  })

  it('applies each choice to the whole app at once and keeps it on this device', async () => {
    const { wrapper } = await mountApp('/accessibility', {})
    await wrapper.get('[data-testid="palette-red-green"]').setValue()
    expect(root().palette).toBe('red-green')
    expect(kept()).toMatchObject({ palette: 'red-green', textSize: 'normal' })
    await wrapper.get('[data-testid="palette-blue-yellow"]').setValue()
    expect(root().palette).toBe('blue-yellow')
    await wrapper.get('[data-testid="text-large"]').setValue()
    expect(root().text).toBe('large')
    await wrapper.get('[data-testid="text-largest"]').setValue()
    expect(root().text).toBe('largest')
    await wrapper.get('[data-testid="dyslexia-font"]').setValue(true)
    expect(root().font).toBe('dyslexia')
    await wrapper.get('[data-testid="reduce-motion"]').setValue(true)
    expect(root().motion).toBe('reduced')
    await wrapper.get('[data-testid="haptics"]').setValue(false)
    await wrapper.get('[data-testid="announce-turns"]').setValue(false)
    await wrapper.get('[data-testid="announce-rolls"]').setValue(false)
    expect(kept()).toEqual({ palette: 'blue-yellow', textSize: 'largest', dyslexiaFont: true, reduceMotion: true, haptics: false, announceTurns: false, announceRolls: false })
    await expectAccessible(wrapper.element as Element)

    // The next visit starts as this one was left, on any page.
    unmountAll()
    for (const k of ['palette', 'text', 'font', 'motion']) Reflect.deleteProperty(document.documentElement.dataset, k)
    const again = await mountApp('/', {})
    expect(applied()).toEqual({ palette: 'blue-yellow', text: 'largest', font: 'dyslexia', motion: 'reduced' })
    await again.router.push('/accessibility')
    await flushPromises()
    expect((again.wrapper.get('[data-testid="palette-blue-yellow"]').element as HTMLInputElement).checked).toBe(true)
    expect((again.wrapper.get('[data-testid="text-largest"]').element as HTMLInputElement).checked).toBe(true)
    for (const id of ['dyslexia-font', 'reduce-motion']) expect((again.wrapper.get(`[data-testid="${id}"]`).element as HTMLInputElement).checked, id).toBe(true)
    for (const id of ['haptics', 'announce-turns', 'announce-rolls']) expect((again.wrapper.get(`[data-testid="${id}"]`).element as HTMLInputElement).checked, id).toBe(false)

    // Going back to the standard look undoes it.
    await again.wrapper.get('[data-testid="reset-accessibility"]').trigger('click')
    expect(applied()).toEqual({ palette: 'standard', text: 'normal', font: 'standard', motion: 'system' })
    expect(kept()).toBeNull()
    expect((again.wrapper.get('[data-testid="haptics"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('takes nothing it does not know from what was kept', async () => {
    localStorage.setItem(KEY, JSON.stringify({ palette: 'sepia', textSize: 'huge', dyslexiaFont: 'yes', reduceMotion: 1, haptics: false, announceTurns: null }))
    const { wrapper } = await mountApp('/accessibility', {})
    expect(applied()).toEqual({ palette: 'standard', text: 'normal', font: 'standard', motion: 'system' })
    expect((wrapper.get('[data-testid="haptics"]').element as HTMLInputElement).checked).toBe(false)
    expect((wrapper.get('[data-testid="announce-turns"]').element as HTMLInputElement).checked).toBe(true)
    unmountAll()
    for (const broken of ['{', '"text"', 'null', '[1]']) {
      localStorage.setItem(KEY, broken)
      await mountApp('/', {})
      expect(applied(), broken).toEqual({ palette: 'standard', text: 'normal', font: 'standard', motion: 'system' })
      unmountAll()
    }
  })

  it('still works where the browser keeps nothing', async () => {
    const refuse = () => { throw new Error('denied') }
    vi.stubGlobal('localStorage', { getItem: refuse, setItem: refuse, removeItem: refuse })
    const { wrapper } = await mountApp('/accessibility', {})
    await wrapper.get('[data-testid="text-large"]').setValue()
    expect(root().text).toBe('large')
    await wrapper.get('[data-testid="reset-accessibility"]').trigger('click')
    expect(root().text).toBe('normal')
  })

  it('reduces motion when the app or the device asks for it, and says when the device already does', async () => {
    systemMotion(false)
    const { wrapper } = await mountApp('/accessibility', {})
    expect(reducedMotion()).toBe(false)
    expect(wrapper.find('[data-testid="device-reduces-motion"]').exists()).toBe(false)
    await wrapper.get('[data-testid="reduce-motion"]').setValue(true)
    expect(reducedMotion()).toBe(true)
    await wrapper.get('[data-testid="reduce-motion"]').setValue(false)
    expect(reducedMotion()).toBe(false)
    unmountAll()
    systemMotion(true)
    const device = await mountApp('/accessibility', {})
    expect(reducedMotion()).toBe(true)
    expect(device.wrapper.get('[data-testid="device-reduces-motion"]').text()).toBe('Your device already asks for reduced motion, so motion is reduced whatever is chosen here.')
    // A browser that cannot say is taken not to ask.
    vi.stubGlobal('matchMedia', undefined)
    expect(reducedMotion()).toBe(false)
  })

  it('buzzes only while haptics are on, and only where the device can', async () => {
    const buzzed = vibrating()
    const { wrapper } = await mountApp('/accessibility', {})
    expect(wrapper.find('[data-testid="no-haptics"]').exists()).toBe(false)
    buzz([120, 60, 120])
    expect(buzzed).toEqual([[120, 60, 120]])
    await wrapper.get('[data-testid="haptics"]').setValue(false)
    buzz(40)
    expect(buzzed).toHaveLength(1)
    await wrapper.get('[data-testid="haptics"]').setValue(true)
    // Turning them on is felt at once.
    expect(buzzed).toEqual([[120, 60, 120], 40])
    unmountAll()
    still()
    const none = await mountApp('/accessibility', {})
    expect(none.wrapper.get('[data-testid="no-haptics"]').text()).toBe('This device cannot vibrate.')
    expect(() => { buzz(40) }).not.toThrow()
  })
})
