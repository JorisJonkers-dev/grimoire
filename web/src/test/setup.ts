import { notifyManager } from '@tanstack/vue-query'
import { config } from '@vue/test-utils'
import { afterEach, vi } from 'vitest'
import { FakeSocket } from './fakeSocket'
import { unmountAll } from './mountApp'

// jsdom lays nothing out, so it has no scrollIntoView; the roster strip calls it to keep a face in view.
Element.prototype.scrollIntoView = function scrollIntoView() {}

// The stub of a TransitionGroup drops its tag, which would leave the roster's faces outside their list.
config.global.stubs = { ...config.global.stubs, TransitionGroup: false }

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
