import { flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { jsonResponse, mountWithQuery } from '@/test/mountWithQuery'
import WhoAmI from './WhoAmI.vue'

describe('WhoAmI', () => {
  it('greets the signed-in account', async () => {
    const wrapper = mountWithQuery(WhoAmI, async () => jsonResponse({ subject: 'aria-player' }))
    expect(wrapper.text()).toContain('Checking')
    await flushPromises()
    expect(wrapper.text()).toContain('Signed in as aria-player')
  })

  it('says so when nobody is signed in', async () => {
    const wrapper = mountWithQuery(WhoAmI, async () =>
      jsonResponse({ type: 'about:blank', title: 'Unauthorized', status: 401 }, 401),
    )
    await flushPromises()
    expect(wrapper.text()).toContain('not signed in')
  })
})
