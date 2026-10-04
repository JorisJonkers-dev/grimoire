import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'root', nickname: 'Root', email: 'root@example.com',
  admin: true, hasPassword: true, twoStep: true, recoveryCodesLeft: 10, adminPowers: true,
}
const aria = { id: '0190c7a8-0000-7000-8000-0000000000a1', username: 'aria', nickname: 'Aria', email: 'aria@example.com', admin: false, status: 'active', createdAt: '2026-09-01T10:00:00Z', lastSeenAt: '2026-10-01T10:00:00Z' }
const bram = { ...aria, id: '0190c7a8-0000-7000-8000-0000000000b1', username: 'bram', nickname: 'Bram', email: 'bram@example.com', status: 'disabled', lastSeenAt: undefined }
const token = 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'

afterEach(() => { unmountAll() })

describe('the Admin page', () => {
  it('lists Accounts and unused Invites, filters them, and makes an invite', async () => {
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/admin', {
      '/api/v1/admin/account-invites': async (_u, req) => {
        sent.push(await req.json())
        return jsonResponse({ token, expiresAt: '2026-10-05T12:00:00Z' }, 201)
      },
      '/api/v1/admin/accounts': () => ({
        accounts: [aria, bram],
        invites: [{ id: '0190c7a8-0000-7000-8000-0000000000e1', admin: true, status: 'invited', createdAt: '2026-10-01T10:00:00Z', expiresAt: '2026-10-04T10:00:00Z' },
          { id: '0190c7a8-0000-7000-8000-0000000000e2', admin: false, status: 'expired', createdAt: '2026-09-01T10:00:00Z', expiresAt: '2026-09-02T10:00:00Z' }],
      }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="admin-account-aria"]').text()).toContain('seen')
    expect(wrapper.get('[data-testid="admin-account-bram"]').text()).toContain('Disabled')
    expect(wrapper.get('[data-testid="admin-account-bram"]').text()).toContain('never seen')
    expect(wrapper.get('[data-testid="admin-invites"]').text()).toContain('Admin invite')
    expect(wrapper.get('[data-testid="admin-invites"]').text()).toContain('Expired')
    await wrapper.get('[data-testid="admin-filter"]').setValue('BRAM')
    expect(wrapper.find('[data-testid="admin-account-aria"]').exists()).toBe(false)
    await wrapper.get('[data-testid="admin-filter"]').setValue('nobody')
    expect(wrapper.find('[data-testid="admin-none"]').exists()).toBe(true)
    await wrapper.get('[data-testid="invite-hours"]').setValue(168)
    await wrapper.get('[data-testid="invite-admin"]').setValue(true)
    await wrapper.get('[data-testid="invite-form"]').trigger('submit')
    await flushPromises()
    expect(sent).toEqual([{ hours: 168, admin: true }])
    expect(wrapper.get('[data-testid="invite-link"]').text()).toContain(`/account-invite#${token}`)
  })

  it('heads the page, counts what is there, and keeps Accounts, Invites and Release Notes each on a tab', async () => {
    const { wrapper, router } = await mountApp('/admin', {
      '/api/v1/admin/accounts': () => ({
        accounts: [{ ...aria, admin: true }, bram],
        invites: [{ id: '0190c7a8-0000-7000-8000-0000000000e1', admin: false, status: 'invited', createdAt: '2026-10-01T10:00:00Z', expiresAt: '2026-10-04T10:00:00Z' },
          { id: '0190c7a8-0000-7000-8000-0000000000e2', admin: false, status: 'expired', createdAt: '2026-09-01T10:00:00Z', expiresAt: '2026-09-02T10:00:00Z' }],
      }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="page-eyebrow"]').text()).toBe('Admin')
    expect(wrapper.get('h1').text()).toBe('Accounts')
    const tabs = () => wrapper.findAll('nav[aria-label="Admin"] a')
    expect(tabs().map((a) => a.text())).toEqual(['Accounts2', 'Invites1', 'Release Notes', 'Shared Library', 'Dice Sets'])
    expect(tabs().map((a) => a.attributes('href'))).toEqual(['/admin', '/admin?tab=invites', '/admin?tab=notes', '/admin/shared-library', '/admin/dice-sets'])
    expect(tabs()[0]?.attributes('aria-current')).toBe('page')
    expect(wrapper.get('[data-testid="admin-tally"]').text()).toBe('2 accounts · 1 invite not yet used · 1 disabled')
    expect(wrapper.findAll('[data-testid="admin-accounts"] th').map((h) => h.text())).toEqual(['Account', 'Email', 'Role', 'Status'])
    const cells = wrapper.findAll('[data-testid="admin-account-aria"] td').map((c) => c.text())
    expect(cells[0]).toBe('AAria@aria')
    expect(cells.slice(1, 3)).toEqual(['aria@example.com', 'Admin'])
    expect(cells[3]).toMatch(/^Activeseen /)
    expect(wrapper.findAll('[data-testid="admin-account-bram"] td').map((c) => c.text()).slice(2)).toEqual(['Member', 'Disablednever seen'])
    expect(wrapper.get('[data-testid="admin-accounts"]').isVisible()).toBe(true)
    expect(wrapper.get('[data-testid="admin-invites"]').isVisible()).toBe(false)
    expect(wrapper.get('[data-testid="release-notes"]').isVisible()).toBe(false)

    // The button in the head leads to where an invite is made.
    await wrapper.get('[data-testid="new-invite"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/admin?tab=invites')
    expect(wrapper.get('h1').text()).toBe('Invites')
    expect(tabs()[1]?.attributes('aria-current')).toBe('page')
    expect(wrapper.get('[data-testid="admin-accounts"]').isVisible()).toBe(false)
    expect(wrapper.get('[data-testid="admin-invites"]').isVisible()).toBe(true)
    await router.push('/admin?tab=notes')
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Release Notes')
    expect(wrapper.get('[data-testid="release-notes"]').isVisible()).toBe(true)
    // A tab nobody knows falls back to the Accounts.
    await router.push('/admin?tab=nonsense')
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Accounts')
  })

  it('tells someone without Admin powers why the page is closed', async () => {
    const { wrapper } = await mountApp('/admin', {
      '/api/v1/admin/accounts': () => jsonResponse({ status: 403, title: 'Forbidden' }, 403),
      '/api/v1/account': () => ({ ...me, adminPowers: false }),
    })
    expect(wrapper.get('[data-testid="admin-forbidden"]').text()).toContain('two-step')
    expect(wrapper.find('[data-testid="admin-link"]').exists()).toBe(false)
  })
})

describe('an Account on the Admin page', () => {
  const detail = (over: Record<string, unknown> = {}) => ({
    account: { ...me, id: aria.id, username: 'aria', nickname: 'Aria', email: 'aria@example.com', admin: false, adminPowers: false,
      oidc: { email: 'aria@jorisjonkers.dev', username: 'aria.j', name: 'Aria J', linkedAt: '2026-10-01T10:00:00Z' } },
    status: 'active', createdAt: '2026-09-01T10:00:00Z', sessions: 2, tokens: 1,
    campaigns: [{ id: '0190c7a8-0000-7000-8000-0000000000f1', name: 'Morvain', role: 'dm' }],
    history: [{ at: '2026-10-02T12:00:00Z', actor: 'root', action: 'sign_in_link_sent', detail: '' }, { at: '2026-09-01T10:00:00Z', actor: 'aria', action: 'created', detail: 'from an invite' }],
    ...over,
  })

  it('shows how they sign in, their Campaigns and history, and runs each control', async () => {
    let state = detail()
    const calls: string[] = []
    const base = `/api/v1/admin/accounts/${aria.id}`
    const { wrapper } = await mountApp(`/admin/accounts/${aria.id}`, {
      [`${base}/sign-in-link`]: () => {
        calls.push('link')
        return new Response(null, { status: 202 })
      },
      [`${base}/admin`]: async (_u, req) => {
        calls.push(`admin ${JSON.stringify(await req.json())}`)
        state = detail({ account: { ...state.account, admin: true } })
        return new Response(null, { status: 204 })
      },
      [`${base}/two-step/reset`]: () => {
        calls.push('reset')
        state = detail({ account: { ...state.account, twoStep: false } })
        return new Response(null, { status: 204 })
      },
      [`${base}/disabled`]: async (_u, req) => {
        const body = (await req.json()) as { value: boolean }
        calls.push(`disabled ${String(body.value)}`)
        if (calls.filter((c) => c.startsWith('disabled')).length > 1) return jsonResponse({ status: 409, title: 'Not allowed' }, 409)
        state = detail({ status: 'disabled', sessions: 0, tokens: 0 })
        return new Response(null, { status: 204 })
      },
      [base]: () => state,
      '/api/v1/account': () => me,
    })
    // The page says where it is, is headed by who this is, and keeps the controls to one side.
    expect(wrapper.get('nav[aria-label="Breadcrumb"]').text().replace(/\s*›\s*/g, ' › ')).toBe('Admin › Accounts › Aria')
    expect(wrapper.findAll('nav[aria-label="Breadcrumb"] a').map((a) => a.attributes('href'))).toEqual(['/admin', '/admin'])
    expect(wrapper.get('h1').text()).toBe('Aria @aria')
    expect(wrapper.get('[data-testid="admin-account-summary"] .g-meta').text()).toContain('Active')
    expect(wrapper.get('[data-testid="admin-account-summary"] .g-meta').text()).toContain('Member')
    expect(wrapper.findAll('[data-testid="admin-account-fields"] dt').map((d) => d.text())).toEqual(['Username', 'Nickname', 'Email'])
    expect(wrapper.findAll('[data-testid="admin-account-fields"] dd').map((d) => d.text())).toEqual(['aria', 'Aria', 'aria@example.com'])
    expect(wrapper.get('aside').attributes('data-testid')).toBe('admin-account-controls')
    expect(wrapper.get('[data-testid="admin-account-methods"]').text()).toContain('with two-step')
    expect(wrapper.get('[data-testid="admin-account-methods"]').text()).toContain('aria@jorisjonkers.dev')
    expect(wrapper.get('[data-testid="admin-account-methods"]').text()).toContain('2 signed-in devices')
    expect(wrapper.get('[data-testid="admin-account-campaigns"]').text()).toContain('Morvain · DM')
    expect(wrapper.get('[data-testid="admin-account-history"]').text()).toContain('Sign-in link emailed')
    await wrapper.get('[data-testid="admin-send-link"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="admin-control-done"]').text()).toContain('on its way')
    await wrapper.get('[data-testid="admin-toggle-admin"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.get('[data-testid="admin-toggle-admin"]').text()).toContain('Remove the Admin role') })
    await wrapper.get('[data-testid="admin-reset-two-step"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.find('[data-testid="admin-reset-two-step"]').exists()).toBe(false) })
    await wrapper.get('[data-testid="admin-toggle-disabled"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.get('[data-testid="admin-toggle-disabled"]').text()).toBe('Enable') })
    await wrapper.get('[data-testid="admin-toggle-disabled"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="admin-control-failed"]').text()).toContain('your own Account')
    expect(calls).toEqual(['link', 'admin {"value":true}', 'reset', 'disabled true', 'disabled false'])
  })

  it('says when the Account cannot be read', async () => {
    const { wrapper } = await mountApp(`/admin/accounts/${aria.id}`, {
      '/api/v1/admin/accounts': () => jsonResponse({ status: 404, title: 'Not found' }, 404),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="admin-account-error"]').text()).toContain('could not be read')
  })
})
