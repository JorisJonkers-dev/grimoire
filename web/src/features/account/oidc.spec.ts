import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const leave = vi.hoisted(() => ({ leaveFor: vi.fn(), returnTo: vi.fn(() => '/campaigns') }))
vi.mock('@/features/account/leave', () => leave)

const account = { id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'aria@example.com', admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false }
const linked = { ...account, oidc: { email: 'aria@jorisjonkers.dev', username: 'aria.j', name: 'Aria Jonk', linkedAt: '2026-10-02T12:00:00Z' } }
const methods = { oidc: 'jorisjonkers.dev' }
const token = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'
const state = 'stateabcdefghijklmnopqrstuvwxyz'
const choose = { status: 'choose', pending: { token, email: 'mira@jorisjonkers.dev', username: 'Mira', name: 'Mira Vale' } }

beforeEach(() => { vi.clearAllMocks() })
afterEach(() => { unmountAll() })

describe('signing in with jorisjonkers.dev', () => {
  it('offers the external login only when one is set up, and sends the browser there', async () => {
    const { wrapper } = await mountApp('/sign-in?next=/campaigns', {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/oidc/sign-ins': () => jsonResponse({ url: 'https://auth.example/authorize?state=x' }, 201),
    })
    await wrapper.get('[data-testid="sign-in-oidc"]').trigger('click')
    await flushPromises()
    expect(leave.leaveFor).toHaveBeenCalledWith('https://auth.example/authorize?state=x', '/campaigns')
    unmountAll()
    const plain = await mountApp('/sign-in', { '/api/v1/sign-in-methods': () => ({}) })
    expect(plain.wrapper.find('[data-testid="sign-in-oidc"]').exists()).toBe(false)
  })

  it('signs a linked login in and returns where the person was going, with the code gone from the address', async () => {
    const sent: unknown[] = []
    const { router } = await mountApp(`/oidc/callback?code=the-code&state=${state}`, {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/oidc/callback': async (_u, req) => {
        sent.push(await req.json())
        return { status: 'signed_in', account }
      },
      '/api/v1/campaigns': () => ({ items: [] }),
    })
    expect(sent).toEqual([{ code: 'the-code', state }])
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe('/campaigns') })
  })

  it('creates an Account for a login no Account has yet', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp(`/oidc/callback?code=c&state=${state}`, {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/oidc/callback': () => choose,
      '/api/v1/oidc/accounts': async (_u, req) => {
        const body = (await req.json()) as { username: string }
        sent.push(body)
        return body.username === 'taken' ? jsonResponse({ status: 409, title: 'Taken' }, 409) : jsonResponse(account, 201)
      },
    })
    expect(wrapper.get('[data-testid="oidc-choose"]').text()).toContain('mira@jorisjonkers.dev')
    expect((wrapper.get('[data-testid="oidc-username"]').element as HTMLInputElement).value).toBe('mira')
    expect((wrapper.get('[data-testid="oidc-nickname"]').element as HTMLInputElement).value).toBe('Mira Vale')
    await wrapper.get('[data-testid="oidc-username"]').setValue('taken')
    await wrapper.get('[data-testid="oidc-choose"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="oidc-choose-failed"]').text()).toContain('Link to it instead')
    await wrapper.get('[data-testid="oidc-username"]').setValue('mira')
    await wrapper.get('[data-testid="oidc-choose"]').trigger('submit')
    await flushPromises()
    expect(sent.at(-1)).toEqual({ token, username: 'mira', nickname: 'Mira Vale' })
    expect(leave.returnTo).toHaveBeenCalled()
  })

  it('links a waiting login to an existing Account with its password', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp(`/oidc/callback?code=c&state=${state}`, {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/oidc/callback': () => choose,
      '/api/v1/oidc/links': async (_u, req) => {
        const body = (await req.json()) as { password: string }
        sent.push(body)
        return body.password === 'right password' ? linked : jsonResponse({ status: 401, title: 'Unauthorized' }, 401)
      },
    })
    await wrapper.get('[data-testid="oidc-mode-link"]').setValue(true)
    await wrapper.get('[data-testid="oidc-username"]').setValue('aria')
    await wrapper.get('[data-testid="oidc-password"]').setValue('wrong')
    await wrapper.get('[data-testid="oidc-choose"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="oidc-choose-failed"]').text()).toContain('do not match')
    await wrapper.get('[data-testid="oidc-password"]').setValue('right password')
    await wrapper.get('[data-testid="oidc-choose"]').trigger('submit')
    await flushPromises()
    expect(sent.at(-1)).toEqual({ token, username: 'aria', password: 'right password' })
    expect(leave.returnTo).toHaveBeenCalled()
  })

  it.each([
    [403, 'does not have access'],
    [409, 'already linked'],
    [410, 'another browser'],
    [503, 'could not be finished'],
  ])('explains a %i from the callback', async (status, text) => {
    const { wrapper } = await mountApp(`/oidc/callback?code=c&state=${state}`, {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/oidc/callback': () => jsonResponse({ status, title: 'No' }, status),
    })
    expect(wrapper.get('[data-testid="oidc-failed"]').text()).toContain(text)
  })

  it('says when the provider sent no code back', async () => {
    const { wrapper, calls } = await mountApp('/oidc/callback?error=access_denied', { '/api/v1/sign-in-methods': () => methods })
    expect(wrapper.get('[data-testid="oidc-failed"]').text()).toContain('cancelled or refused')
    expect(calls.some((u) => u.pathname === '/api/v1/oidc/callback')).toBe(false)
  })
})

describe('the Account page and jorisjonkers.dev', () => {
  it('shows the linked login read-only and unlinks it', async () => {
    let current: Record<string, unknown> = linked
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/account/oidc-link': () => {
        current = account
        return new Response(null, { status: 204 })
      },
      '/api/v1/account': () => current,
    })
    const fields = wrapper.get('[data-testid="oidc-fields"]').text()
    expect(fields).toContain('Aria Jonk')
    expect(fields).toContain('aria@jorisjonkers.dev')
    expect(wrapper.find('[data-testid="oidc-fields"] input').exists()).toBe(false)
    await wrapper.get('[data-testid="oidc-unlink"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.find('[data-testid="oidc-link"]').exists()).toBe(true) })
  })

  it('asks for a password before unlinking an Account without one', async () => {
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/account': () => ({ ...linked, hasPassword: false }),
    })
    expect(wrapper.get('[data-testid="unlink-needs-password"]').text()).toContain('Set a password')
    expect(wrapper.get('[data-testid="oidc-unlink"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="password-form"]').text()).toContain('Set a password')
  })

  it('starts linking from the Account page and comes back to it', async () => {
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => methods,
      '/api/v1/account/oidc-link': () => jsonResponse({ url: 'https://auth.example/authorize?state=y' }, 201),
      '/api/v1/account': () => account,
    })
    await wrapper.get('[data-testid="oidc-link"]').trigger('click')
    await flushPromises()
    expect(leave.leaveFor).toHaveBeenCalledWith('https://auth.example/authorize?state=y', '/account')
  })

  it('edits the Username, Nickname and email', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => ({}),
      '/api/v1/account': async (_u, req) => {
        if (req.method !== 'PUT') return account
        const body = (await req.json()) as { username: string }
        sent.push(body)
        return body.username === 'taken' ? jsonResponse({ status: 409, title: 'Taken' }, 409) : { ...account, ...body }
      },
    })
    expect(wrapper.find('[data-testid="oidc-section"]').exists()).toBe(false)
    await wrapper.get('[data-testid="profile-username"]').setValue('taken')
    await wrapper.get('[data-testid="profile-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="profile-failed"]').text()).toContain('already has an Account')
    await wrapper.get('[data-testid="profile-username"]').setValue('aria.v')
    await wrapper.get('[data-testid="profile-nickname"]').setValue(' Aria V ')
    await wrapper.get('[data-testid="profile-form"]').trigger('submit')
    await flushPromises()
    expect(sent.at(-1)).toEqual({ username: 'aria.v', nickname: 'Aria V', email: 'aria@example.com' })
    expect(wrapper.get('[data-testid="profile-saved"]').text()).toBe('Saved.')
  })
})
