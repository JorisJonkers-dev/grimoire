import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const me = {
  id: '0190c7a8-0000-7000-8000-0000000000c1', username: 'root', nickname: 'Root', email: 'r@example.com',
  admin: true, hasPassword: true, twoStep: true, recoveryCodesLeft: 10, adminPowers: true,
}
const note = (extra = {}) => ({
  id: '0190c7a8-0000-7000-8000-0000000000d1', version: '1.1.0', title: 'What is new in Grimoire 1.1.0', body: '- Social: Friends\n- Release Notes',
  status: 'published', publishAt: '2026-10-02T12:00:00Z', createdAt: '2026-10-02T11:00:00Z', updatedAt: '2026-10-02T11:00:00Z', ...extra,
})

afterEach(() => { unmountAll() })

describe('the Release Note on the Dashboard', () => {
  it('shows the newest unseen Release Note once', async () => {
    let seen = false
    const { wrapper } = await mountApp('/', {
      '/api/v1/release-notes/unseen': () => (seen ? {} : { note: note() }),
      '/api/v1/release-notes/': () => {
        seen = true
        return new Response(null, { status: 204 })
      },
      '/api/v1/notifications': () => ({ items: [], unread: 0 }),
      '/api/v1/account': () => me,
    })
    const card = wrapper.get('[data-testid="release-note"]')
    expect(card.text()).toContain('New in 1.1.0')
    expect(card.findAll('li').map((l) => l.text())).toEqual(['Social: Friends', 'Release Notes'])
    await wrapper.get('[data-testid="release-note-seen"]').trigger('click')
    await flushPromises()
    await vi.waitFor(() => { expect(wrapper.find('[data-testid="release-note"]').exists()).toBe(false) })
  })

  it('reads a body without dashes as paragraphs, and shows nothing signed out', async () => {
    const { wrapper } = await mountApp('/', {
      '/api/v1/release-notes/unseen': () => ({ note: note({ body: 'A quiet release.' }) }),
      '/api/v1/notifications': () => ({ items: [], unread: 0 }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="release-note"] p:not(.eyebrow)').text()).toBe('A quiet release.')
    unmountAll()
    const out = await mountApp('/', { '/api/v1/release-notes/unseen': () => ({ note: note() }) })
    expect(out.wrapper.find('[data-testid="release-note"]').exists()).toBe(false)
  })
})

describe('Release Notes on the Admin page', () => {
  it('drafts from a release, edits, schedules and publishes', async () => {
    const calls: unknown[] = []
    let items = [note({ id: '0190c7a8-0000-7000-8000-0000000000d0', version: '1.0.0' }), note({ id: '0190c7a8-0000-7000-8000-0000000000c9', version: '0.9.0', status: 'draft', title: 'Old draft' })]
    const { wrapper } = await mountApp('/admin', {
      '/api/v1/admin/release-notes/': async (url, req) => {
        const body = (await req.json()) as Record<string, unknown>
        calls.push([url.pathname.split('/').pop(), body])
        return url.pathname.endsWith('/publish') ? note({ status: 'scheduled' }) : note({ status: 'draft', title: String(body.title), publishAt: undefined })
      },
      '/api/v1/admin/release-notes': async (_u, req) => {
        if (req.method === 'POST') {
          const body = (await req.json()) as { version: string }
          calls.push(['draft', body])
          if (body.version === '1.0.0') return jsonResponse({ status: 409, title: 'Already there' }, 409)
          const made = note({ status: 'draft', publishAt: undefined })
          items = [made, ...items]
          return jsonResponse(made, 201)
        }
        return { items }
      },
      '/api/v1/admin/accounts': () => ({ accounts: [], invites: [] }),
      '/api/v1/notifications': () => ({ items: [], unread: 0 }),
      '/api/v1/account': () => me,
    })
    expect(wrapper.get('[data-testid="release-1.0.0"]').text()).toContain('Published')
    await wrapper.get('[data-testid="release-open-0.9.0"]').trigger('click')
    expect((wrapper.get('[data-testid="release-title"]').element as HTMLInputElement).value).toBe('Old draft')
    await wrapper.get('[data-testid="release-version"]').setValue('1.0.0')
    await wrapper.get('[data-testid="release-draft"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="release-failed"]').text()).toContain('already has its Release Note')
    await wrapper.get('[data-testid="release-version"]').setValue(' 1.1.0 ')
    await wrapper.get('[data-testid="release-draft"]').trigger('submit')
    await flushPromises()
    expect((wrapper.get('[data-testid="release-body"]').element as HTMLTextAreaElement).value).toContain('Social: Friends')
    await wrapper.get('[data-testid="release-title"]').setValue('Friends arrive')
    await wrapper.get('[data-testid="release-body"]').setValue('- Social: Friends\n- Release Notes')
    await wrapper.get('[data-testid="release-editor"]').trigger('submit')
    await flushPromises()
    await wrapper.get('[data-testid="release-at"]').setValue('2026-10-09T18:00')
    expect(wrapper.get('[data-testid="release-publish"]').text()).toBe('Schedule')
    await wrapper.get('[data-testid="release-publish"]').trigger('click')
    await flushPromises()
    expect(calls[2]).toEqual(['0190c7a8-0000-7000-8000-0000000000d1', { title: 'Friends arrive', body: '- Social: Friends\n- Release Notes' }])
    const [, published] = calls[3] as [string, { at: string }]
    expect(new Date(published.at).getTime()).toBe(new Date('2026-10-09T18:00').getTime())
    expect(wrapper.find('[data-testid="release-editor"]').exists()).toBe(false)
  })
})
