import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const account = { id: '0190c7a8-0000-7000-8000-0000000000a1', username: 'aria', nickname: 'Aria', email: 'aria@example.com', admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false }
const signedOut = () => jsonResponse({ type: 'about:blank', title: 'Unauthorized', status: 401 }, 401)
const quiet = { live: [], needs: [] }
const routes = (who: object | null) => ({
  '/api/v1/account': () => (who ? who : signedOut()),
  '/api/v1/dashboard': () => quiet,
  '/api/v1/library': () => ({ items: [], collections: [] }),
  '/api/v1/notifications': () => ({ items: [], unread: 0 }),
})
const places = (root: ReturnType<typeof Object>, nav: string) =>
  (root as { findAll: (s: string) => { text: () => string }[] }).findAll(`nav[aria-label="${nav}"] a`).map((a) => a.text())

afterEach(() => { unmountAll() })

describe('the app bar', () => {
  it('shows a visitor the places open to them and the way in', async () => {
    const { wrapper } = await mountApp('/', routes(null))
    expect(wrapper.get('a.brand').text()).toBe('Grimoire')
    expect(places(wrapper, 'Main')).toEqual(['Dashboard', 'Campaigns', 'Compendium'])
    expect(wrapper.get('nav[aria-label="Main"] [aria-current="page"]').text()).toBe('Dashboard')
    expect(wrapper.get('[data-testid="sign-in-link"]').text()).toBe('Sign in')
    expect(wrapper.find('[data-testid="account-menu"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('shows the five places to somebody signed in and marks the one they are in', async () => {
    const { wrapper, router } = await mountApp('/', routes(account))
    expect(places(wrapper, 'Main')).toEqual(['Dashboard', 'Campaigns', 'Characters', 'Library', 'Compendium'])
    // A phone's tab bar holds the same places.
    expect(places(wrapper, 'Places')).toEqual(['Dashboard', 'Campaigns', 'Characters', 'Library', 'Compendium'])
    const current = () => wrapper.findAll('[aria-current="page"]').filter((a) => a.element.closest('nav[aria-label="Main"], nav[aria-label="Places"]')).map((a) => a.text())
    expect(current()).toEqual(['Dashboard', 'Dashboard'])
    for (const [path, place] of [['/library', 'Library'], ['/shared-library', 'Library'], ['/compendium/spells', 'Compendium'], ['/characters', 'Characters'], ['/campaigns', 'Campaigns'], ['/join', 'Campaigns']] as const) {
      await router.push(path)
      await flushPromises()
      expect(current(), path).toEqual([place, place])
    }
    // A page that is none of the five marks none.
    await router.push('/accessibility')
    await flushPromises()
    expect(current()).toEqual([])
    expect(wrapper.find('[data-testid="sign-in-link"]').exists()).toBe(false)
  })

  it('keeps the account\'s own places behind its picture', async () => {
    const { wrapper, router } = await mountApp('/', routes(account))
    const toggle = wrapper.get('[data-testid="account-menu-toggle"]')
    const menu = wrapper.get('#account-places')
    expect(toggle.attributes()).toMatchObject({ 'aria-expanded': 'false', 'aria-label': 'Account menu, Aria', 'aria-controls': 'account-places' })
    expect(toggle.text()).toBe('A')
    expect(menu.isVisible()).toBe(false)
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(menu.isVisible()).toBe(true)
    expect(menu.findAll('a, button').map((el) => el.text())).toEqual(['Aria', 'Friends', 'Dice', 'Accessibility', 'Sign out'])
    expect(wrapper.get('[data-testid="account-link"]').attributes('href')).toBe('/account')
    await expectAccessible(wrapper.element as Element)
    // Going somewhere closes it; pressing the picture again closes it too.
    await router.push('/accessibility')
    await flushPromises()
    expect(menu.isVisible()).toBe(false)
    await toggle.trigger('click')
    await toggle.trigger('click')
    expect(menu.isVisible()).toBe(false)
  })

  it('adds the Admin page for an Admin, and Talk beside the bell', async () => {
    const { wrapper } = await mountApp('/', routes({ ...account, admin: true, adminPowers: true }))
    expect(wrapper.get('[data-testid="admin-link"]').attributes('href')).toBe('/admin')
    expect(wrapper.get('[data-testid="conversations-link"]').attributes('href')).toBe('/conversations')
    unmountAll()
    const plain = await mountApp('/', routes(account))
    expect(plain.wrapper.find('[data-testid="admin-link"]').exists()).toBe(false)
  })

  it('signs out from the menu and closes it', async () => {
    let out = false
    const { wrapper, router } = await mountApp('/', { ...routes(account), '/api/v1/sign-out': () => { out = true; return new Response(null, { status: 204 }) } })
    await wrapper.get('[data-testid="account-menu-toggle"]').trigger('click')
    await wrapper.get('[data-testid="sign-out"]').trigger('click')
    await flushPromises()
    expect(out).toBe(true)
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('sign-in') })
  })
})
