import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { fakeClock, mountApp, unmountAll } from '@/test/mountApp'
import { POLL_MS } from './gamepad'

const key = (init: KeyboardEventInit, on: EventTarget = document.body) => {
  const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })
  on.dispatchEvent(e)
  return e
}
const quiet = { live: [], needs: [] }
/** Plugs a gamepad in and returns how to press its buttons. */
function gamepad() {
  const pad = { connected: true, buttons: Array.from({ length: 16 }, () => ({ pressed: false })), axes: [0, 0] }
  Object.defineProperty(navigator, 'getGamepads', { configurable: true, value: () => [pad] })
  window.dispatchEvent(new Event('gamepadconnected'))
  return async (...buttons: number[]) => {
    for (const b of buttons) (pad.buttons[b] as { pressed: boolean }).pressed = true
    vi.advanceTimersByTime(POLL_MS)
    for (const b of buttons) (pad.buttons[b] as { pressed: boolean }).pressed = false
    vi.advanceTimersByTime(POLL_MS)
    await flushPromises()
  }
}

afterEach(() => {
  unmountAll()
  vi.useRealTimers()
  Reflect.deleteProperty(navigator, 'getGamepads')
})

describe('the keys', () => {
  it('are shown with ?, listing what this page answers to, and put away with Escape', async () => {
    const { wrapper } = await mountApp('/', { '/api/v1/dashboard': () => quiet })
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
    const search = wrapper.get('[data-testid="header-search-input"]')
    expect(search.attributes('aria-keyshortcuts')).toBe('/')
    ;(wrapper.get('a.brand').element as HTMLElement).focus()
    expect(key({ key: '?', code: 'Slash', shiftKey: true }).defaultPrevented).toBe(true)
    await flushPromises()
    const map = wrapper.get('[data-testid="key-map"]')
    expect(map.attributes()).toMatchObject({ role: 'dialog', 'aria-modal': 'true', 'aria-label': 'Keys and gamepad' })
    const rows = map.findAll('[data-testid="key-row"]').map((r) => `${r.get('dt').text()} ${r.get('dd').text()}`)
    expect(rows).toContain('? Show or hide these keys')
    expect(rows).toContain('/ Search everything')
    expect(rows).toContain('Arrows On the map, move from hex to hex')
    expect(map.findAll('[data-testid="pad-row"]').map((r) => `${r.get('dt').text()} ${r.get('dd').text()}`)).toContain('A Press what has the focus')
    // The focus goes into it, and back where it was when it closes.
    expect(document.activeElement).toBe(map.get('[data-testid="key-map-close"]').element)
    // The page behind takes no focus and no keys while the list shows; Enter still presses its button.
    expect(wrapper.get('.shell').element.closest('[inert]')).not.toBeNull()
    expect(key({ key: 'Enter' }, map.get('[data-testid="key-map-close"]').element).defaultPrevented).toBe(false)
    expect(key({ key: '/', code: 'Slash' }).defaultPrevented).toBe(false)
    expect(document.activeElement).toBe(map.get('[data-testid="key-map-close"]').element)
    await expectAccessible(wrapper.element as Element)
    key({ key: 'Escape' })
    await flushPromises()
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
    expect(document.activeElement).toBe(wrapper.get('a.brand').element)
    expect(wrapper.get('.shell').element.closest('[inert]')).toBeNull()
    // ? puts it away as well, and so does its button.
    key({ key: '?', shiftKey: true })
    await flushPromises()
    key({ key: '?', shiftKey: true })
    await flushPromises()
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
    key({ key: '?', shiftKey: true })
    await flushPromises()
    await wrapper.get('[data-testid="key-map-close"]').trigger('click')
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
  })

  it('press the control that names them, and leave typing alone', async () => {
    const { wrapper } = await mountApp('/', { '/api/v1/dashboard': () => quiet })
    const search = wrapper.get('[data-testid="header-search-input"]').element as HTMLInputElement
    expect(key({ key: '/', code: 'Slash' }).defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(search)
    // In a field a key is a letter, not a shortcut.
    expect(key({ key: '?', shiftKey: true }, search).defaultPrevented).toBe(false)
    await flushPromises()
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
    // A key nothing answers to is left to the browser.
    search.blur()
    expect(key({ key: 'q' }).defaultPrevented).toBe(false)
    expect(key({ key: 'Enter' }).defaultPrevented).toBe(false)
    expect(key({ key: 'ArrowDown' }).defaultPrevented).toBe(false)
  })

  it('are on the Table Display too, which has no header', async () => {
    const { wrapper } = await mountApp('/sign-in', { '/api/v1/sign-in-methods': () => ({}) })
    key({ key: '?', shiftKey: true })
    await flushPromises()
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(true)
  })
})

describe('a gamepad', () => {
  it('moves the focus with the D-pad, presses with A, jumps regions with the bumpers and shows the keys with Start', async () => {
    fakeClock()
    const { wrapper, router } = await mountApp('/', { '/api/v1/dashboard': () => quiet, '/api/v1/campaigns': () => ({ items: [] }) })
    const press = gamepad()
    const at = () => document.activeElement
    await press(13)
    expect(at()).toBe(wrapper.get('a.brand').element)
    await press(15)
    expect(at()).toBe(wrapper.get('nav[aria-label="Main"] a').element)
    await press(12)
    expect(at()).toBe(wrapper.get('a.brand').element)
    await press(13)
    // A presses what has the focus.
    await press(0)
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('campaigns') })
    // The bumpers jump from region to region.
    await press(5)
    const first = at()
    await press(5)
    expect(at()).not.toBe(first)
    await press(4)
    expect(at()).toBe(first)
    // Start shows the keys, B puts them away.
    await press(9)
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(true)
    await press(1)
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
    // X presses the control that answers to E, as the key does: here nothing does, and nothing breaks.
    await press(2)
    expect(wrapper.find('[data-testid="key-map"]').exists()).toBe(false)
  })

  it('is not read once the app is gone', async () => {
    fakeClock()
    const reads = vi.fn(() => [{ connected: true, buttons: [], axes: [] }])
    Object.defineProperty(navigator, 'getGamepads', { configurable: true, value: reads })
    await mountApp('/', { '/api/v1/dashboard': () => quiet })
    vi.advanceTimersByTime(POLL_MS * 2)
    expect(reads).toHaveBeenCalled()
    unmountAll()
    reads.mockClear()
    expect(key({ key: '?', shiftKey: true }).defaultPrevented).toBe(false)
    vi.advanceTimersByTime(POLL_MS * 4)
    window.dispatchEvent(new Event('gamepadconnected'))
    vi.advanceTimersByTime(POLL_MS * 4)
    expect(reads).not.toHaveBeenCalled()
  })
})
