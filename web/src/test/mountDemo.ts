import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '@/app/router'
import { createDemo } from '@/demo/demo'
import DemoRoot from '@/demo/DemoRoot.vue'
import type { Persona } from '@/demo/role'
import { configureApi, signInOnUnauthorized } from '@/infrastructure/http'

const mounted: (() => void)[] = []

/** Unmounts every demo a test mounted and drops its sign-in redirect, so none reaches the next test. */
export function unmountDemos() {
  for (const undo of mounted.splice(0)) undo()
}

/** Waits until the page stops asking: a list that answers may send for its details next. */
async function settle(demo: { answers: unknown[]; unhandled: unknown[] }) {
  let seen = -1
  for (let round = 0; round < 20; round++) {
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))
    const now = demo.answers.length + demo.unhandled.length
    if (now === seen) return
    seen = now
  }
}

/** Mounts the demo build's root at a path as a persona, the way the demo bundle starts. */
export async function mountDemo(path: string, persona: Persona) {
  const demo = createDemo(persona)
  // Anything that tried the real network would show here; the demo must never reach it.
  const network = vi.spyOn(globalThis, 'fetch').mockImplementation(() => Promise.reject(new Error('network reached')))
  vi.stubGlobal('WebSocket', demo.Socket)
  configureApi({ baseUrl: 'https://demo.invalid', fetch: demo.fetch })
  const router = createAppRouter(createMemoryHistory())
  const eject = signInOnUnauthorized(router)
  await router.push(path)
  const wrapper = mount(DemoRoot, {
    props: { world: demo.world },
    attachTo: document.body,
    global: { plugins: [router, [VueQueryPlugin, { queryClient: new QueryClient({ defaultOptions: { queries: { retry: false } } }) }]] },
  })
  mounted.push(() => { wrapper.unmount(); eject() })
  await settle(demo)
  const done = async () => { await settle(demo) }
  return { wrapper, router, demo, network, settle: done }
}
