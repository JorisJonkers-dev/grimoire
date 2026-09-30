import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import App from '@/app/App.vue'
import { createAppRouter } from '@/app/router'
import { configureApi } from '@/infrastructure/http'
import { jsonResponse } from './mountWithQuery'

export type Route = (url: URL, request: Request) => unknown

const mounted: { unmount: () => void }[] = []

/** Unmounts every app a test mounted, so nothing it left running reaches the next test. */
export function unmountAll() {
  for (const w of mounted.splice(0)) w.unmount()
}

/** Mounts the whole app at a path, answering API calls with the first matching route. */
export async function mountApp(path: string, routes: Record<string, Route>) {
  const calls: URL[] = []
  const fetchImpl = vi.fn<typeof fetch>(async (input) => {
    const request = input as Request
    const url = new URL(request.url)
    calls.push(url)
    const key = Object.keys(routes).find((prefix) => url.pathname.startsWith(prefix))
    if (!key) return jsonResponse({ type: 'about:blank', title: 'Not found', status: 404 }, 404)
    const body = await routes[key]?.(url, request)
    if (body instanceof Response) return body
    return jsonResponse(body)
  })
  configureApi({ baseUrl: 'http://localhost', fetch: fetchImpl })
  const router = createAppRouter(createMemoryHistory())
  await router.push(path)
  const wrapper = mount(App, {
    attachTo: document.body,
    global: { plugins: [router, [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  mounted.push(wrapper)
  await flushPromises()
  return { wrapper, router, calls }
}

/** Fakes only the clocks a component schedules with; flushPromises and fetch keep running on their own. */
export function fakeClock() {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] })
}
