import { flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const NPC = '0190c7a8-0000-7000-8000-000000000007'
const npc = (extra = {}) => ({
  id: NPC, name: 'Morvain', title: 'Count', description: 'A vampire.', dmNotes: 'Old', disposition: 'hostile', updatedAt: '2026-09-30T20:00:00Z', ...extra,
})
const revisions = [
  { no: 3, action: 'restore', author: 'Joris', origin: 'ui', restoredFrom: 1, createdAt: '2026-09-30T22:00:00Z' },
  { no: 2, action: 'update', author: 'Joris', origin: 'mcp', client: 'claude', createdAt: '2026-09-30T21:00:00Z' },
  { no: 1, action: 'create', author: 'Joris', origin: 'ui', createdAt: '2026-09-30T20:00:00Z' },
]
const problem = (status: number) => () => jsonResponse({ type: 'about:blank', title: 'x', status }, status)
const base = `/api/v1/campaigns/${ID}/npcs`

afterEach(() => {
  document.body.innerHTML = ''
})

describe('npc list', () => {
  it('lists, adds and shows deleted NPCs', async () => {
    const posted: unknown[] = []
    const { wrapper, router } = await mountApp(`/campaigns/${ID}/npcs`, {
      [`${base}/deleted`]: () => [{ id: '0190c7a8-0000-7000-8000-000000000008', name: 'Rahadin', deletedAt: '2026-09-30T20:00:00Z' }],
      [`${base}/${NPC}/revisions`]: () => revisions,
      [`${base}/${NPC}`]: () => npc(),
      [base]: async (_u, req) => {
        if (req.method === 'POST') {
          posted.push(await req.clone().json())
          return npc()
        }
        return [npc({ title: '' })]
      },
    })
    expect(wrapper.get('[data-testid="npc-list"]').text()).toContain('No title')
    expect(wrapper.get('[data-testid="npc-deleted"]').text()).toContain('Rahadin')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="npc-name"]').setValue(' Ismark ')
    await wrapper.get('[data-testid="npc-create"] select').setValue('friendly')
    await wrapper.get('[data-testid="npc-create"]').trigger('submit')
    await vi.waitFor(() => { expect(router.currentRoute.value.name).toBe('npc') }, { timeout: 5000 })
    expect(posted).toEqual([{ name: 'Ismark', disposition: 'friendly' }])
  })

  it('refuses players', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/npcs`, { [base]: problem(403) })
    expect(wrapper.find('[data-testid="npcs-refused"]').exists()).toBe(true)
  })
})

describe('npc page', () => {
  it('edits, compares revisions and restores', async () => {
    const writes: string[] = []
    const { wrapper } = await mountApp(`/campaigns/${ID}/npcs/${NPC}`, {
      [`${base}/${NPC}/revisions/diff`]: (url) =>
        url.searchParams.get('from') === '1' && url.searchParams.get('to') === '2'
          ? [{ field: 'dmNotes', before: 'Old', after: 'New' }]
          : [],
      [`${base}/${NPC}/revisions/1/restore`]: () => {
        writes.push('restore 1')
        return npc()
      },
      [`${base}/${NPC}/revisions`]: () => revisions,
      [`${base}/${NPC}`]: async (_u, req) => {
        if (req.method === 'PUT') writes.push(`PUT ${(await req.clone().json() as { dmNotes: string }).dmNotes}`)
        return npc()
      },
    })
    expect(wrapper.get('h1').text()).toBe('Morvain')
    const history = wrapper.get('[data-testid="npc-history"]')
    expect(history.text()).toContain('#2 update')
    expect(history.text()).toContain('mcp (claude)')
    expect(history.text()).toContain('from #1')
    const fields = wrapper.findAll('[data-testid="npc-form"] input')
    await fields[0]?.setValue('Morvain')
    await fields[1]?.setValue('Count')
    await wrapper.get('[data-testid="npc-form"] select').setValue('hostile')
    await wrapper.findAll('[data-testid="npc-form"] textarea')[0]?.setValue('A vampire.')
    await wrapper.get('[data-testid="npc-notes"]').setValue('New')
    await wrapper.get('[data-testid="npc-form"]').trigger('submit')
    await flushPromises()
    const boxes = history.findAll('input[type="checkbox"]')
    await boxes[1]?.setValue(true)
    await boxes[2]?.setValue(true)
    await flushPromises()
    expect(wrapper.get('[data-testid="npc-diff"]').text()).toContain('Old')
    await boxes[0]?.setValue(true)
    await flushPromises()
    expect(wrapper.get('[data-testid="npc-diff"]').text()).toContain('No differences')
    await boxes[0]?.setValue(false)
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="restore-1"]').trigger('click')
    await flushPromises()
    expect(writes).toEqual(['PUT New', 'restore 1'])
  })

  it('shows deleted NPCs by their history and reports failures', async () => {
    const { wrapper } = await mountApp(`/campaigns/${ID}/npcs/${NPC}`, {
      [`${base}/${NPC}/revisions/2/restore`]: problem(503),
      [`${base}/${NPC}/revisions`]: () => revisions,
      [`${base}/${NPC}`]: problem(404),
    })
    expect(wrapper.get('h1').text()).toBe('Deleted NPC')
    expect(wrapper.find('[data-testid="npc-deleted-banner"]').exists()).toBe(true)
    await wrapper.get('[data-testid="restore-2"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('could not be restored')
  })

  it('saves and deletes, reporting failures', async () => {
    const failing = await mountApp(`/campaigns/${ID}/npcs/${NPC}`, {
      [`${base}/${NPC}/revisions`]: () => revisions,
      [`${base}/${NPC}`]: (_u, req) => (req.method === 'GET' ? npc() : problem(503)()),
    })
    await failing.wrapper.get('[data-testid="npc-form"]').trigger('submit')
    await flushPromises()
    expect(failing.wrapper.get('[role="alert"]').text()).toContain('could not be saved')
    await failing.wrapper.get('[data-testid="npc-delete"]').trigger('click')
    await flushPromises()
    expect(failing.wrapper.get('[role="alert"]').text()).toContain('could not be deleted')
    document.body.innerHTML = ''
    const ok = await mountApp(`/campaigns/${ID}/npcs/${NPC}`, {
      [`${base}/${NPC}/revisions`]: () => revisions,
      [`${base}/${NPC}`]: (_u, req) => (req.method === 'DELETE' ? new Response(null, { status: 204 }) : npc()),
      [base]: () => [],
    })
    await ok.wrapper.get('[data-testid="npc-delete"]').trigger('click')
    await vi.waitFor(() => { expect(ok.router.currentRoute.value.name).toBe('npcs') }, { timeout: 5000 })
    document.body.innerHTML = ''
    const missing = await mountApp(`/campaigns/${ID}/npcs/${NPC}`, {})
    expect(missing.wrapper.find('[data-testid="npc-missing"]').exists()).toBe(true)
  })
})
