import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { signInOnUnauthorized } from '@/infrastructure/http'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const account = { id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'aria@example.com', admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false }
const token = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'

afterEach(() => { unmountAll(); })

describe('signing in', () => {
  it('signs in with a Username and password and returns to where the person was going', async () => {
    const sent: unknown[] = []
    let signedIn = false
    const { wrapper, router } = await mountApp('/sign-in?next=/campaigns', {
      '/api/v1/account': () => (signedIn ? account : jsonResponse({ status: 401, title: 'Unauthorized' }, 401)),
      '/api/v1/sign-in': async (_u, req) => {
        const body = (await req.json()) as { password: string }
        sent.push(body)
        if (body.password !== 'correct horse battery') return jsonResponse({ status: 401, title: 'Unauthorized' }, 401)
        signedIn = true
        return account
      },
      '/api/v1/campaigns': () => ({ items: [] }),
    })
    expect(wrapper.find('[data-testid="sign-in-link"]').exists()).toBe(false)
    await wrapper.get('[data-testid="sign-in-username"]').setValue(' aria ')
    await wrapper.get('[data-testid="sign-in-password"]').setValue('wrong')
    await wrapper.get('[data-testid="sign-in-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="sign-in-failed"]').text()).toContain('do not match')
    await wrapper.get('[data-testid="sign-in-password"]').setValue('correct horse battery')
    await wrapper.get('[data-testid="sign-in-form"]').trigger('submit')
    await flushPromises()
    expect(sent.at(-1)).toEqual({ username: 'aria', password: 'correct horse battery' })
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe('/campaigns'); })
  })

  it('never follows next out of Grimoire', async () => {
    const { wrapper, router } = await mountApp('/sign-in?next=//evil.example', { '/api/v1/sign-in': () => account })
    await wrapper.get('[data-testid="sign-in-username"]').setValue('aria')
    await wrapper.get('[data-testid="sign-in-password"]').setValue('x')
    await wrapper.get('[data-testid="sign-in-form"]').trigger('submit')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/')
  })

  it('emails a sign-in link for a forgotten password', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/sign-in', {
      '/api/v1/sign-in-links': async (_u, req) => {
        sent.push(await req.json())
        return new Response(null, { status: 202 })
      },
    })
    await wrapper.get('[data-testid="forgot"]').trigger('click')
    await wrapper.get('[data-testid="link-email"]').setValue(' aria@example.com ')
    await wrapper.get('[data-testid="link-form"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ email: 'aria@example.com' }])
    expect(wrapper.get('[data-testid="link-sent"]').text()).toContain('on its way')
  })

  it('sends anyone without a session on a private page to sign in, and lets them sign out', async () => {
    const eject = signInOnUnauthorized
    let signedIn = true
    const { wrapper, router } = await mountApp('/campaigns', {
      '/api/v1/account': () => (signedIn ? account : jsonResponse({ status: 401, title: 'Unauthorized' }, 401)),
      '/api/v1/sign-out': () => new Response(null, { status: 204 }),
      '/api/v1/campaigns': () => (signedIn ? { items: [] } : jsonResponse({ status: 401, title: 'Unauthorized' }, 401)),
    })
    const stop = eject(router)
    expect(wrapper.get('[data-testid="account-link"]').text()).toBe('Aria')
    signedIn = false
    await wrapper.get('[data-testid="sign-out"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('sign-in')
    await router.push('/campaigns')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/sign-in?next=/campaigns')
    stop()
  })
})

describe('setting up an Account', () => {
  it('turns an invite into an Account', async () => {
    const sent: unknown[] = []
    const { wrapper, router } = await mountApp(`/account-invite#${token}`, {
      '/api/v1/account-invites/preview': () => ({ expiresAt: '2026-10-05T12:00:00Z', admin: true }),
      '/api/v1/account-invites/accept': async (_u, req) => {
        sent.push(await req.json())
        return account
      },
    })
    expect(wrapper.text()).toContain('You are invited as an Admin.')
    await wrapper.get('[data-testid="setup-username"]').setValue(' aria ')
    await wrapper.get('[data-testid="setup-nickname"]').setValue('Aria')
    await wrapper.get('[data-testid="setup-email"]').setValue('aria@example.com')
    await wrapper.get('[data-testid="setup-password"]').setValue('correct horse battery')
    await wrapper.get('[data-testid="account-setup"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ token, username: 'aria', nickname: 'Aria', email: 'aria@example.com', password: 'correct horse battery' }])
    expect(router.currentRoute.value.name).toBe('home')
  })

  it('says when an invite is used, expired or malformed', async () => {
    const { wrapper } = await mountApp('/account-invite#short', {})
    expect(wrapper.get('[data-testid="account-invite-invalid"]').text()).toContain('used or has expired')
    const gone = await mountApp(`/account-invite#${token}`, { '/api/v1/account-invites/preview': () => jsonResponse({ status: 410, title: 'Gone' }, 410) })
    expect(gone.wrapper.find('[data-testid="account-invite-invalid"]').exists()).toBe(true)
    const taken = await mountApp(`/account-invite#${token}`, {
      '/api/v1/account-invites/preview': () => ({ expiresAt: '2026-10-05T12:00:00Z', admin: false }),
      '/api/v1/account-invites/accept': () => jsonResponse({ status: 409, title: 'Taken' }, 409),
    })
    for (const [id, v] of [['setup-username', 'aria'], ['setup-nickname', 'A'], ['setup-email', 'a@b.c'], ['setup-password', '0123456789']]) {
      await taken.wrapper.get(`[data-testid="${id}"]`).setValue(v)
    }
    await taken.wrapper.get('[data-testid="account-setup"]').trigger('submit')
    await flushPromises()
    expect(taken.wrapper.get('[data-testid="setup-failed"]').text()).toContain('taken')
  })
})

describe('sign-in links', () => {
  it('signs in with a link and offers a new password', async () => {
    const sent: unknown[] = []
    const { wrapper, router } = await mountApp(`/sign-in-link#${token}`, {
      '/api/v1/sign-in-links/use': () => account,
      '/api/v1/account/password': async (_u, req) => {
        sent.push(await req.json())
        return new Response(null, { status: 204 })
      },
    })
    expect(wrapper.text()).toContain('Welcome back, Aria.')
    await wrapper.get('[data-testid="link-password"]').setValue('a new long password')
    await wrapper.get('[data-testid="new-password"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ password: 'a new long password' }])
    expect(router.currentRoute.value.name).toBe('home')
  })

  it('says when a link is used or expired', async () => {
    const { wrapper } = await mountApp(`/sign-in-link#${token}`, { '/api/v1/sign-in-links/use': () => jsonResponse({ status: 410, title: 'Gone' }, 410) })
    expect(wrapper.get('[data-testid="link-invalid"]').text()).toContain('used or has expired')
  })
})

describe('the Account page', () => {
  it('changes the password and points an Admin to the Admin page', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/account', {
      '/api/v1/account/password': async (_u, req) => {
        sent.push(await req.json())
        return new Response(null, { status: 204 })
      },
      '/api/v1/account/history': () => ({ items: [{ at: '2026-10-02T12:00:00Z', actor: 'aria', action: 'created', detail: 'from an invite' }] }),
      '/api/v1/account': () => ({ ...account, admin: true, adminPowers: true }),
    })
    expect(wrapper.get('[data-testid="profile-form"]').text()).toContain('You are an Admin.')
    expect(wrapper.get('[data-testid="account-admin-link"]').attributes('href')).toBe('/admin')
    expect(wrapper.get('[data-testid="admin-link"]').text()).toBe('Admin')
    expect(wrapper.get('[data-testid="account-history"]').text()).toContain('Account created')
    await wrapper.get('[data-testid="account-password"]').setValue('a new long password')
    await wrapper.get('[data-testid="password-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="password-saved"]').text()).toBe('Saved.')
    expect(sent).toEqual([{ password: 'a new long password' }])
  })

  it('says when the Account cannot be read', async () => {
    const { wrapper } = await mountApp('/account', { '/api/v1/account': () => jsonResponse({ status: 500, title: 'Boom' }, 500) })
    expect(wrapper.text()).toContain('could not be read')
  })
})
