import { flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import { startDemo } from './start'

const RealSocket = globalThis.WebSocket

afterEach(() => {
  globalThis.WebSocket = RealSocket
  vi.restoreAllMocks()
  window.location.hash = ''
})

it('starts the real app on the mock, on a hash route, with the role switcher', async () => {
  const network = vi.spyOn(globalThis, 'fetch')
  document.body.innerHTML = '<div id="app"></div>'
  window.location.hash = '#/compendium/spells'
  const app = startDemo()
  await flushPromises()
  expect(document.querySelector('[data-testid="demo-switcher"]')).not.toBeNull()
  expect(document.querySelector('[data-testid="compendium-link"]')).not.toBeNull()
  expect(network).not.toHaveBeenCalled()
  app.unmount()
})

it('answers a live Session socket in the browser, not on the network', () => {
  document.body.innerHTML = '<div id="app"></div>'
  const app = startDemo()
  const socket = new WebSocket('wss://demo.invalid/api/v1/campaigns/c/sessions/s/live?audience=party')
  expect(globalThis.WebSocket).not.toBe(RealSocket)
  socket.close()
  app.unmount()
})
