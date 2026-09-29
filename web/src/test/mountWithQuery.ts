import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { mount } from '@vue/test-utils'
import type { Component } from 'vue'
import { configureApi } from '@/infrastructure/http'

export function jsonResponse(body: unknown, status = 200): Response {
  const type = status >= 400 ? 'application/problem+json' : 'application/json'
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': type } })
}

export function mountWithQuery(component: Component, fetchImpl: typeof fetch) {
  configureApi({ baseUrl: "http://localhost", fetch: fetchImpl })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return mount(component, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
}
