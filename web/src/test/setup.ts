import { notifyManager } from '@tanstack/vue-query'
import { afterEach, vi } from 'vitest'
import { FakeSocket } from './fakeSocket'
import { unmountAll } from './mountApp'

// Query results reach components on the microtask queue, so a test's fake timers never hold them back.
notifyManager.setScheduler(queueMicrotask)

// A test that fails with fake timers on must not hand them to the next one.
// Nothing a test mounted or opened outlives it: a leftover session's reconnect timer would otherwise
// open a socket in the next test and hand it to FakeSocket.last().
afterEach(() => {
  vi.useRealTimers()
  unmountAll()
  FakeSocket.all = []
  document.body.innerHTML = ''
})
