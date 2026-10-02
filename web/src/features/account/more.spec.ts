import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const state = 'stateabcdefghijklmnopqrstuvwxyz'
const choose = { status: 'choose', pending: { token: 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG', email: 'm@example.org', username: 'mira', name: '' } }
const linkToken = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'

afterEach(() => { unmountAll() })

describe('the edges of signing in', () => {
  it.each([
    [410, 'expired'],
    [422, 'Choose a Username'],
    [500, 'did not work'],
  ])('explains a %i when creating an Account for a waiting login', async (status, text) => {
    const { wrapper } = await mountApp(`/oidc/callback?code=c&state=${state}`, {
      '/api/v1/oidc/callback': () => choose,
      '/api/v1/oidc/accounts': () => jsonResponse({ status, title: 'No' }, status),
    })
    expect((wrapper.get('[data-testid="oidc-nickname"]').element as HTMLInputElement).value).toBe('mira')
    await wrapper.get('[data-testid="oidc-mode-link"]').setValue(true)
    await wrapper.get('[data-testid="oidc-mode-create"]').setValue(true)
    await wrapper.get('[data-testid="oidc-nickname"]').setValue('Mira')
    await wrapper.get('[data-testid="oidc-choose"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="oidc-choose-failed"]').text()).toContain(text)
  })

  it('goes back from the forgotten-password form', async () => {
    const { wrapper } = await mountApp('/sign-in', {})
    await wrapper.get('[data-testid="forgot"]').trigger('click')
    await wrapper.get('[data-testid="link-form"] button.link').trigger('click')
    expect(wrapper.find('[data-testid="sign-in-form"]').exists()).toBe(true)
  })

  it('starts over from an emailed link whose second step expired', async () => {
    const { wrapper, router } = await mountApp(`/sign-in-link#${linkToken}`, {
      '/api/v1/sign-in-links/use': () => jsonResponse({ challenge: 'challengeabcdefghijklmnopqrstuvwxyz' }, 202),
      '/api/v1/sign-in/two-step': () => jsonResponse({ status: 410, title: 'Gone' }, 410),
    })
    await wrapper.get('[data-testid="two-step-code"]').setValue('123456')
    await wrapper.get('[data-testid="two-step-form"]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid="two-step-expired"] button').trigger('click')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('sign-in') })
  })
})
