import { flushPromises } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import { jsonResponse, mountWithQuery } from '@/test/mountWithQuery'
import App from './App.vue'
import { createAppRouter } from './router'
import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { mount } from '@vue/test-utils'
import { configureApi } from '@/infrastructure/http'

describe('router', () => {
  it('renders the home page at /', async () => {
    configureApi({ baseUrl: 'http://localhost', fetch: async () => jsonResponse({ service: 'grimoire', version: '1', database: 'up', startedAt: '2026-09-29T20:00:00Z' }) })
    const router = createAppRouter(createMemoryHistory())
    await router.push('/')
    const wrapper = mount(App, {
      global: { plugins: [router, [VueQueryPlugin, { queryClient: new QueryClient() }]] },
    })
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Grimoire')
    expect(wrapper.find('[data-testid="status-panel"]').exists()).toBe(true)
    expect(typeof mountWithQuery).toBe('function')
  })

  it('uses browser history by default', () => {
    expect(createAppRouter().options.history.base).toBe('')
  })
})

describe('gallery route', () => {
  it('lazy-loads the component gallery', async () => {
    const router = createAppRouter(createMemoryHistory())
    await router.push('/gallery')
    const wrapper = mount(App, { global: { plugins: [router, [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('Component gallery')
  })
})
