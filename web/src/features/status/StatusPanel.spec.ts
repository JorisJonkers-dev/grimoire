import { flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { jsonResponse, mountWithQuery } from '@/test/mountWithQuery'
import StatusPanel from './StatusPanel.vue'

const ok = { service: 'grimoire', version: '1.2.3', database: 'up', startedAt: '2026-09-29T20:00:00Z' }

describe('StatusPanel', () => {
  it('shows a loading state, then the status from the API', async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => jsonResponse(ok))
    const wrapper = mountWithQuery(StatusPanel, fetchMock)
    expect(wrapper.find('[data-testid="status-loading"]').exists()).toBe(true)
    await flushPromises()
    const text = wrapper.get('[data-testid="status-ok"]').text()
    expect(text).toContain('1.2.3')
    expect(text).toContain('up')
    expect(text).toContain('29 Sept 2026')
    const request = fetchMock.mock.calls[0]?.[0] as Request
    expect(new URL(request.url).pathname).toBe('/api/v1/status')
  })

  it('shows a generic error when the API fails', async () => {
    const wrapper = mountWithQuery(StatusPanel, async () =>
      jsonResponse({ type: 'about:blank', title: 'Service unavailable', status: 503 }, 503),
    )
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('not answering')
  })

  it('rejects a response that breaks the contract', async () => {
    const wrapper = mountWithQuery(StatusPanel, async () => jsonResponse({ ...ok, database: 'maybe' }))
    await flushPromises()
    expect(wrapper.find('[data-testid="status-error"]').exists()).toBe(true)
  })
})
