import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const methods = { oidc: 'jorisjonkers.dev' }
const order = (root: Element, ...ids: string[]) => ids.map((id) => [...root.querySelectorAll('[data-testid]')].findIndex((el) => el.getAttribute('data-testid') === id))

afterEach(() => { unmountAll() })

describe('the sign-in page', () => {
  it('stands on its own: the brand beside the form, with no app header over it', async () => {
    const { wrapper } = await mountApp('/sign-in', { '/api/v1/sign-in-methods': () => methods })
    expect(wrapper.find('nav[aria-label="Main"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="header-search"]').exists()).toBe(false)
    const brand = wrapper.get('[data-testid="auth-brand"]')
    expect(brand.get('a').text()).toBe('Grimoire')
    expect(brand.get('a').attributes('href')).toBe('/')
    expect(brand.get('p').text()).toBe('The table is set. Take your seat.')
    // The form is the page's main content, in one column the page centres.
    const main = wrapper.get('main')
    expect(main.get('h1').text()).toBe('Sign in')
    expect(main.find('[data-testid="auth-column"]').exists()).toBe(true)
    expect(main.find('[data-testid="auth-brand"]').exists()).toBe(false)
    await expectAccessible(wrapper.element as Element)
  })

  it('offers the external login first and the Username below it, when a login is set up', async () => {
    const { wrapper } = await mountApp('/sign-in', { '/api/v1/sign-in-methods': () => methods })
    expect(wrapper.get('[data-testid="sign-in-intro"]').text()).toBe('Use the account an Admin made for you, or your jorisjonkers.dev login.')
    expect(wrapper.get('[data-testid="sign-in-oidc"]').text()).toBe('Sign in with jorisjonkers.dev')
    expect(wrapper.get('[data-testid="sign-in-or"]').text()).toBe('or with your Username')
    const [external, or, form] = order(wrapper.element as Element, 'sign-in-oidc', 'sign-in-or', 'sign-in-form')
    expect(external).toBeGreaterThanOrEqual(0)
    expect(or).toBeGreaterThan(external ?? 0)
    expect(form).toBeGreaterThan(or ?? 0)
    // The external login is no part of the Username form: Enter in a field never starts it.
    expect(wrapper.get('[data-testid="sign-in-form"]').find('[data-testid="sign-in-oidc"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="sign-in-form"] button[type="submit"]').text()).toBe('Sign in')
    expect(wrapper.get('[data-testid="sign-in-note"]').text()).toBe('No account yet? Grimoire is invite-only: ask the person running your table to send you an Account Invite.')
  })

  it('offers the Username alone when no external login is set up', async () => {
    const { wrapper } = await mountApp('/sign-in', { '/api/v1/sign-in-methods': () => ({}) })
    expect(wrapper.get('[data-testid="sign-in-intro"]').text()).toBe('Use the account an Admin made for you.')
    for (const id of ['sign-in-oidc', 'sign-in-or']) expect(wrapper.find(`[data-testid="${id}"]`).exists(), id).toBe(false)
    expect(wrapper.find('[data-testid="sign-in-form"]').exists()).toBe(true)
    await expectAccessible(wrapper.element as Element)
  })

  it('says so when the external login cannot be reached, and keeps the Username form', async () => {
    const { wrapper } = await mountApp('/sign-in', {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/oidc/sign-ins': () => jsonResponse({ type: 'about:blank', title: 'Bad gateway', status: 502 }, 502),
    })
    await wrapper.get('[data-testid="sign-in-oidc"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="sign-in-oidc-failed"]').text()).toBe('jorisjonkers.dev could not be reached. Try again shortly.')
    expect(wrapper.find('[data-testid="sign-in-form"]').exists()).toBe(true)
  })

  it('keeps the way to a sign-in link under the password, and the brand around that form too', async () => {
    const { wrapper } = await mountApp('/sign-in', { '/api/v1/sign-in-methods': () => methods })
    expect(wrapper.get('[data-testid="sign-in-forgot"]').text()).toBe('Forgot it? Email me a sign-in link')
    await wrapper.get('[data-testid="forgot"]').trigger('click')
    expect(wrapper.get('h1').text()).toBe('Sign in with a link')
    expect(wrapper.find('[data-testid="link-form"]').exists()).toBe(true)
    for (const id of ['sign-in-oidc', 'sign-in-or', 'sign-in-form']) expect(wrapper.find(`[data-testid="${id}"]`).exists(), id).toBe(false)
    expect(wrapper.find('[data-testid="auth-brand"]').exists()).toBe(true)
    await expectAccessible(wrapper.element as Element)
  })
})
