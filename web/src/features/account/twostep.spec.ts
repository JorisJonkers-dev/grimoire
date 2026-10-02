import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const account = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'aria@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const challenge = 'challengeabcdefghijklmnopqrstuvwxyz'
const linkToken = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'
const secret = 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP'
const codes = ['abcde-fghjk', 'mnpqr-stuvw']

afterEach(() => { unmountAll() })

async function toCodeStep(routes: Record<string, (u: URL, r: Request) => unknown>) {
  const app = await mountApp('/sign-in?next=/campaigns', {
    '/api/v1/sign-in/two-step': routes.pass ?? (() => account),
    '/api/v1/sign-in': () => jsonResponse({ challenge }, 202),
    '/api/v1/campaigns': () => ({ items: [] }),
  })
  await app.wrapper.get('[data-testid="sign-in-username"]').setValue('aria')
  await app.wrapper.get('[data-testid="sign-in-password"]').setValue('a long password')
  await app.wrapper.get('[data-testid="sign-in-form"]').trigger('submit')
  await flushPromises()
  return app
}

describe('the two-step code page', () => {
  it('asks for one numeric code the phone can autofill, then signs in', async () => {
    const sent: unknown[] = []
    const { wrapper, router } = await toCodeStep({
      pass: async (_u, req) => {
        const body = (await req.json()) as { code: string }
        sent.push(body)
        return body.code === '654321' ? account : jsonResponse({ status: 401, title: 'Wrong code' }, 401)
      },
    })
    const field = wrapper.get('[data-testid="two-step-code"]')
    expect(field.attributes('inputmode')).toBe('numeric')
    expect(field.attributes('autocomplete')).toBe('one-time-code')
    expect(field.attributes('maxlength')).toBe('6')
    await field.setValue('12 34 56 78')
    expect((field.element as HTMLInputElement).value).toBe('123456')
    await wrapper.get('[data-testid="two-step-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="two-step-wrong"]').text()).toContain('wrong')
    await wrapper.get('[data-testid="two-step-code"]').setValue('654321')
    await wrapper.get('[data-testid="two-step-form"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ challenge, code: '123456' }, { challenge, code: '654321' }])
    await vi.waitFor(() => { expect(router.currentRoute.value.fullPath).toBe('/campaigns') })
  })

  it('takes a recovery code instead', async () => {
    const sent: unknown[] = []
    const { wrapper } = await toCodeStep({
      pass: async (_u, req) => {
        sent.push(await req.json())
        return account
      },
    })
    await wrapper.get('[data-testid="two-step-toggle"]').trigger('click')
    await wrapper.get('[data-testid="two-step-recovery"]').setValue(' abcde-fghjk ')
    await wrapper.get('[data-testid="two-step-form"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ challenge, code: 'abcde-fghjk' }])
  })

  it('starts over once the challenge has expired', async () => {
    const { wrapper } = await toCodeStep({ pass: () => jsonResponse({ status: 410, title: 'Gone' }, 410) })
    await wrapper.get('[data-testid="two-step-code"]').setValue('123456')
    await wrapper.get('[data-testid="two-step-form"]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid="two-step-expired"] button').trigger('click')
    expect(wrapper.find('[data-testid="sign-in-form"]').exists()).toBe(true)
  })

  it('asks for the second step after an emailed link too', async () => {
    const { wrapper } = await mountApp(`/sign-in-link#${linkToken}`, {
      '/api/v1/sign-in-links/use': () => jsonResponse({ challenge }, 202),
      '/api/v1/sign-in/two-step': () => account,
    })
    await wrapper.get('[data-testid="two-step-code"]').setValue('123456')
    await wrapper.get('[data-testid="two-step-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="new-password"]').text()).toContain('Welcome back, Aria')
  })
})

describe('two-step on the Account page', () => {
  it('sets two-step up with a QR code and shows the recovery codes once', async () => {
    let current = account
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => ({}),
      '/api/v1/account/two-step/confirm': async (_u, req) => {
        const body = (await req.json()) as { code: string }
        sent.push(body)
        if (body.code !== '111111') return jsonResponse({ status: 401, title: 'Wrong code' }, 401)
        current = { ...account, twoStep: true, recoveryCodesLeft: 2 }
        return { codes }
      },
      '/api/v1/account/two-step': () => jsonResponse({ secret, uri: `otpauth://totp/Grimoire:aria?secret=${secret}&issuer=Grimoire` }, 201),
      '/api/v1/account': () => current,
    })
    await wrapper.get('[data-testid="two-step-begin"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="two-step-uri"]').attributes('href')).toContain('otpauth://totp/Grimoire:aria')
    expect(wrapper.find('[data-testid="two-step-uri"] svg path').attributes('d')).toMatch(/^M\d+ \d+h1v1h-1z/)
    expect(wrapper.get('[data-testid="two-step-secret"]').text()).toBe(secret)
    await wrapper.get('[data-testid="two-step-confirm-code"]').setValue('000000')
    await wrapper.get('[data-testid="two-step-confirm"]').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('does not match')
    await wrapper.get('[data-testid="two-step-confirm-code"]').setValue('111111')
    await wrapper.get('[data-testid="two-step-confirm"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="recovery-codes"]').text()).toContain('abcde-fghjk')
    await vi.waitFor(() => { expect(wrapper.get('[data-testid="two-step-on"]').text()).toContain('2 recovery codes left') })
  })

  it('replaces the recovery codes and turns two-step off with a current code', async () => {
    let current = { ...account, twoStep: true, recoveryCodesLeft: 3 }
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => ({}),
      '/api/v1/account/two-step/recovery-codes': async (_u, req) => {
        const body = (await req.json()) as { code: string }
        return body.code === '222222' ? { codes } : jsonResponse({ status: 401, title: 'Wrong code' }, 401)
      },
      '/api/v1/account/two-step/disable': () => {
        current = account
        return new Response(null, { status: 204 })
      },
      '/api/v1/account': () => current,
    })
    await wrapper.get('[data-testid="two-step-manage-code"]').setValue('999999')
    await wrapper.get('[data-testid="two-step-reset"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="two-step-change-failed"]').text()).toContain('wrong')
    await wrapper.get('[data-testid="two-step-manage-code"]').setValue('222222')
    await wrapper.get('[data-testid="two-step-reset"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="recovery-codes"]').text()).toContain('mnpqr-stuvw')
    await wrapper.get('[data-testid="two-step-manage-code"]').setValue('333333')
    await wrapper.get('[data-testid="two-step-disable"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.find('[data-testid="two-step-begin"]').exists()).toBe(true) })
  })

  it('tells an Admin without two-step that their powers wait for it', async () => {
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => ({}),
      '/api/v1/account': () => ({ ...account, admin: true }),
    })
    expect(wrapper.get('[data-testid="admin-needs-two-step"]').text()).toContain('Admin powers')
    expect(wrapper.find('[data-testid="invite-form"]').exists()).toBe(false)
  })
})
