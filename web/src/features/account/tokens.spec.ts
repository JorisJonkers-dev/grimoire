import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const account = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'aria', nickname: 'Aria', email: 'aria@example.com',
  admin: false, hasPassword: true, twoStep: false, recoveryCodesLeft: 0, adminPowers: false,
}
const used = { id: '0190c7a8-0000-7000-8000-00000000a001', name: 'Claude', scopes: ['read', 'build'], createdAt: '2026-09-01T10:00:00Z', expiresAt: '2026-12-01T10:00:00Z', lastUsedAt: '2026-10-01T10:00:00Z' }
const fresh = 'gmt_abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG'

afterEach(() => { unmountAll() })

describe('Access Tokens on the Account page', () => {
  it('lists tokens with their last use, mints one shown once, and revokes one', async () => {
    let items: unknown[] = [used]
    const sent: unknown[] = []
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => ({}),
      '/api/v1/account/access-tokens': async (url, req) => {
        if (req.method === 'POST') {
          const body = (await req.json()) as { name: string }
          sent.push(body)
          const made = { ...used, id: '0190c7a8-0000-7000-8000-00000000a002', name: body.name, scopes: ['read'], lastUsedAt: undefined }
          items = [made, ...items]
          return jsonResponse({ token: fresh, accessToken: made }, 201)
        }
        if (req.method === 'DELETE') {
          sent.push(url.pathname)
          items = items.filter((t) => !url.pathname.endsWith((t as { id: string }).id))
          return new Response(null, { status: 204 })
        }
        return { items }
      },
      '/api/v1/account': () => account,
    })
    const list = wrapper.get('[data-testid="access-token-list"]').text()
    expect(list).toContain('Claude')
    expect(list).toContain('read, build')
    expect(list).toContain('last used')
    // Tokens are a table: what it is, what it can do, when it ends, when it was last used.
    expect(wrapper.findAll('[data-testid="access-token-list"] th[scope="col"]').map((h) => h.text())).toEqual(['Token', 'Can', 'Expires', 'Last used', 'Actions'])
    expect(wrapper.findAll('[data-testid="access-token-list"] tbody tr')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="access-token-list"] tbody td')[1]?.text()).toBe('read, build')
    await wrapper.get('[data-testid="access-token-name"]').setValue(' Notebook ')
    await wrapper.get('[data-testid="scope-build"]').setValue(false)
    await wrapper.get('[data-testid="access-token-days"]').setValue(7)
    await wrapper.get('[data-testid="access-token-form"]').trigger('submit')
    await flushPromises()
    expect(sent[0]).toEqual({ name: 'Notebook', scopes: ['read'], days: 7 })
    expect(wrapper.get('[data-testid="access-token-fresh"]').text()).toContain(fresh)
    await vi.waitFor(() => { expect(wrapper.get('[data-testid="access-token-list"]').text()).toContain('never used') })
    await wrapper.get(`[data-testid="revoke-${used.id}"]`).trigger('click')
    await flushPromises()
    expect(sent[1]).toBe(`/api/v1/account/access-tokens/${used.id}`)
    await vi.waitFor(() => { expect(wrapper.get('[data-testid="access-token-list"]').text()).not.toContain('Claude') })
  })

  it('says when there are none and when minting fails', async () => {
    const { wrapper } = await mountApp('/account', {
      '/api/v1/sign-in-methods': () => ({}),
      '/api/v1/account/access-tokens': (_u, req) => (req.method === 'POST' ? jsonResponse({ status: 422, title: 'Invalid' }, 422) : { items: [] }),
      '/api/v1/account': () => account,
    })
    expect(wrapper.get('[data-testid="access-token-none"]').text()).toContain('No Access Tokens')
    await wrapper.get('[data-testid="access-token-name"]').setValue('x')
    await wrapper.get('[data-testid="access-token-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('could not be made')
  })
})
