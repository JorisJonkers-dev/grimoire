import { client } from './api/client.gen'

/** Point the generated client at the page's own origin; the platform serves the API beside the app. */
export function configureApi(options: { baseUrl?: string; fetch?: typeof fetch } = {}): void {
  client.setConfig({ baseUrl: options.baseUrl ?? '', ...(options.fetch ? { fetch: options.fetch } : {}) })
}
