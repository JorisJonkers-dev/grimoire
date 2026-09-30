import { notifyManager } from '@tanstack/vue-query'
import { afterEach, vi } from 'vitest'

// Query results reach components on the microtask queue, so a test's fake timers never hold them back.
notifyManager.setScheduler(queueMicrotask)

// A test that fails with fake timers on must not hand them to the next one.
afterEach(() => {
  vi.useRealTimers()
})
