import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import type { LibraryEntry, LibraryEntryDetail } from '@/infrastructure/api/types.gen'
import { expectAccessible } from '@/test/axe'
import { mountApp, unmountAll } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const HAG = '0190c7a8-0000-7000-8000-0000000000e1'
const at = '2026-10-01T20:00:00Z'
const hag: LibraryEntry = { id: HAG, kind: 'creature', name: 'Bog Hag Matriarch', revision: 3, createdAt: at, updatedAt: at, fields: [{ name: 'HP', value: '80' }] }
const detail: LibraryEntryDetail = {
  entry: hag,
  revisions: [
    { no: 3, name: 'Bog Hag Matriarch', fields: hag.fields, createdAt: at, origin: 'mcp', client: 'Claude' },
    { no: 2, name: 'Bog Hag', fields: [], createdAt: at, origin: 'mcp' },
    { no: 1, name: 'Hag', fields: [], createdAt: at, origin: 'ui' },
  ],
  uses: [],
}
const restored: LibraryEntryDetail = {
  entry: { ...hag, name: 'Hag', revision: 4, fields: [] },
  revisions: [{ no: 4, name: 'Hag', fields: [], createdAt: at, origin: 'ui' }, ...detail.revisions],
  uses: [],
}

afterEach(() => {
  unmountAll()
  document.body.innerHTML = ''
})

function backend(entry: LibraryEntryDetail, shared = false) {
  const calls: string[] = []
  const state = { fail: 0, done: false }
  const routes = {
    [`/api/v1/library/${HAG}/revisions/`]: (url: URL, req: Request) => {
      calls.push(`${req.method} ${url.pathname.split('/').slice(5).join('/')}`)
      if (state.fail) return jsonResponse({ type: 'about:blank', title: 'No', status: state.fail }, state.fail)
      state.done = true
      return restored
    },
    [`/api/v1/library/${HAG}`]: () => (state.done ? restored : { ...entry, entry: { ...entry.entry, shared } }),
    '/api/v1/library/submissions': () => [],
    '/api/v1/campaigns': () => ({ items: [] }),
  }
  return { calls, routes, state }
}

describe('the Revisions of a Library entry', () => {
  it('say which were made through an agent, and bring an earlier one back', async () => {
    const { calls, routes, state } = backend(detail)
    const { wrapper } = await mountApp(`/library/${HAG}`, routes)
    await flushPromises()
    const lines = () => wrapper.findAll('[data-testid="entry-revision"]')
    const when = new Date(at).toLocaleDateString()
    expect(lines().map((li) => li.get('[data-testid="revision-line"]').text())).toEqual([
      `Revision 3 · Bog Hag Matriarch · ${when} · made through Claude`,
      `Revision 2 · Bog Hag · ${when} · made through an agent`,
      `Revision 1 · Hag · ${when}`,
    ])
    // The newest is what the entry is now: there is nothing to bring back to.
    expect(lines().map((li) => li.find('[data-testid="revision-restore"]').exists())).toEqual([false, true, true])
    expect(lines()[2]?.get('[data-testid="revision-restore"]').attributes('aria-label')).toBe('Bring Revision 1 back')
    await expectAccessible(wrapper.element as Element)

    // A failure says so and changes nothing.
    state.fail = 404
    await lines()[1]?.get('[data-testid="revision-restore"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="revision-problem"]').text()).toBe('That Revision could not be brought back.')
    expect(lines()).toHaveLength(3)

    state.fail = 0
    await lines()[2]?.get('[data-testid="revision-restore"]').trigger('click')
    await flushPromises()
    expect(calls).toEqual(['POST revisions/2/restore', 'POST revisions/1/restore'])
    expect(wrapper.find('[data-testid="revision-problem"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="entry-status"]').text()).toBe('Revision 1 brought back as Revision 4.')
    expect(wrapper.get('h1').text()).toContain('Hag')
    expect(lines()).toHaveLength(4)
    expect((wrapper.get('[data-testid="entry-name"]').element as HTMLInputElement).value).toBe('Hag')
  })

  it('offers no restore on a read-only copy from the Shared Library', async () => {
    const { routes } = backend(detail, true)
    const { wrapper } = await mountApp(`/library/${HAG}`, routes)
    await flushPromises()
    expect(wrapper.findAll('[data-testid="entry-revision"]')).toHaveLength(3)
    expect(wrapper.find('[data-testid="revision-restore"]').exists()).toBe(false)
  })
})
