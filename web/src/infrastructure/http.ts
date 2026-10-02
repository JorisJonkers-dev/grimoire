import type { Router } from 'vue-router'
import { publicPages } from '@/app/router'
import { client } from './api/client.gen'

/** Point the generated client at the page's own origin; the platform serves the API beside the app. */
export function configureApi(options: { baseUrl?: string; fetch?: typeof fetch } = {}): void {
  client.setConfig({ baseUrl: options.baseUrl ?? '', ...(options.fetch ? { fetch: options.fetch } : {}) })
}

/** Send a request the API refuses for want of a session to the sign-in page, then back again. */
export function signInOnUnauthorized(router: Router): () => void {
  const id = client.interceptors.response.use((response) => {
    const here = router.currentRoute.value
    if (response.status === 401 && !publicPages.includes(String(here.name)) && !response.url.endsWith('/api/v1/sign-in')) {
      void router.push({ name: 'sign-in', query: { next: here.fullPath } })
    }
    return response
  })
  return () => { client.interceptors.response.eject(id); }
}
