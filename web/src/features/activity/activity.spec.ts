import { flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { expectAccessible } from '@/test/axe'
import { mountApp } from '@/test/mountApp'
import { jsonResponse } from '@/test/mountWithQuery'

const ID = '0190c7a8-0000-7000-8000-000000000001'
const base = `/api/v1/campaigns/${ID}/activity`
const change = (no: number, extra: object = {}) => ({
  revisionId: `0190c7a8-0000-7000-8000-00000000009${String(no)}`, entityType: 'npc', entityId: '0190c7a8-0000-7000-8000-000000000081', name: 'Tamsin',
  no, action: 'update', author: 'Joris', origin: 'mcp', client: 'prep-agent', createdAt: '2026-10-01T20:00:00Z', undoable: false, ...extra,
})

describe('AI activity page', () => {
  it('lists what the agent changed and undoes the latest change', async () => {
    const undone: string[] = []
    let fail = false
    const { wrapper } = await mountApp(`/campaigns/${ID}/activity`, {
      [`${base}/`]: (u) => {
        undone.push(u.pathname.split('/')[6] ?? '')
        return fail ? jsonResponse({ type: 'about:blank', title: 'x', status: 422, detail: 'Undo the later changes to Tamsin first.' }, 422) : change(3, { action: 'restore', origin: 'ui' })
      },
      [base]: () => [
        change(2, { undoable: true }),
        change(1, { action: 'create' }),
        change(1, { revisionId: '0190c7a8-0000-7000-8000-000000000099', entityType: 'shop', name: 'Store', action: 'delete', client: undefined, undoable: true }),
      ],
    })
    expect(wrapper.get('[data-testid="mcp-url"]').element).toHaveProperty('value', `${window.location.origin}/mcp`)
    expect(wrapper.get('[data-testid="activity-Tamsin-2"]').text()).toContain('prep-agent changed the NPC Tamsin · revision 2 · Joris')
    expect(wrapper.get('[data-testid="activity-Tamsin-1"]').text()).toContain('prep-agent created the NPC Tamsin')
    expect(wrapper.get('[data-testid="activity-Tamsin-1"]').text()).toContain('changed since')
    expect(wrapper.find('[data-testid="undo-Tamsin-1"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="activity-Store-1"]').text()).toContain('An agent deleted the shop Store')
    await expectAccessible(wrapper.element as Element)
    await wrapper.get('[data-testid="undo-Tamsin-2"]').trigger('click')
    await flushPromises()
    expect(undone).toEqual(['0190c7a8-0000-7000-8000-000000000092'])
    expect(wrapper.find('[data-testid="activity-error"]').exists()).toBe(false)
    fail = true
    await wrapper.get('[aria-label="Undo: An agent deleted the shop Store"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="activity-error"]').text()).toBe('Undo: Undo the later changes to Tamsin first.')
  })

  it('says when nothing happened yet, when undo breaks, and that players cannot look', async () => {
    const empty = await mountApp(`/campaigns/${ID}/activity`, {
      [`${base}/`]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 503 }, 503),
      [base]: () => [change(1, { undoable: true })],
    })
    await empty.wrapper.get('[data-testid="undo-Tamsin-1"]').trigger('click')
    await flushPromises()
    expect(empty.wrapper.get('[data-testid="activity-error"]').text()).toBe('Undo failed. Try again shortly.')
    const none = await mountApp(`/campaigns/${ID}/activity`, { [base]: () => [] })
    expect(none.wrapper.get('[data-testid="activity-empty"]').text()).toContain('Nothing yet.')
    const { wrapper } = await mountApp(`/campaigns/${ID}/activity`, { [base]: () => jsonResponse({ type: 'about:blank', title: 'x', status: 403 }, 403) })
    expect(wrapper.get('[data-testid="activity-refused"]').text()).toBe('Only the DM can see AI activity.')
  })
})
