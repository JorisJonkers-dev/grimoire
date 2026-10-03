import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { FromWorker } from './diceWorker'
import { goFlat, startWorker, wantsGPU } from './gpu'

class FakeWorker {
  static last: FakeWorker
  onmessage: ((e: MessageEvent<FromWorker>) => void) | null = null
  onerror: (() => void) | null = null
  posted: unknown[][] = []
  stopped = false
  constructor(readonly url: URL, readonly options: WorkerOptions) {
    FakeWorker.last = this
  }
  postMessage(...args: unknown[]) {
    this.posted.push(args)
  }
  terminate() {
    this.stopped = true
  }
}

beforeEach(() => {
  sessionStorage.clear()
  vi.stubGlobal('Worker', FakeWorker)
  Object.defineProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen', { configurable: true, value: () => ({}) })
})
afterEach(() => {
  vi.unstubAllGlobals()
  Reflect.deleteProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen')
})

describe('GPU dice', () => {
  it('are tried only where they can run, motion is wanted, and they have not failed before', () => {
    expect(wantsGPU()).toBe(true)
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    expect(wantsGPU()).toBe(false)
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    expect(wantsGPU()).toBe(true)
    goFlat()
    expect(wantsGPU()).toBe(false)
    sessionStorage.clear()
    Reflect.deleteProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen')
    expect(wantsGPU()).toBe(false)
    Object.defineProperty(HTMLCanvasElement.prototype, 'transferControlToOffscreen', { configurable: true, value: () => ({}) })
    vi.stubGlobal('Worker', undefined)
    expect(wantsGPU()).toBe(false)
  })

  it('run in a module worker that answers the page, and count an error as a failure', () => {
    const heard: FromWorker[] = []
    const worker = startWorker((m) => heard.push(m))
    expect(FakeWorker.last.options).toEqual({ type: 'module' })
    expect(String(FakeWorker.last.url)).toContain('diceWorker')
    worker.post({ type: 'clear' })
    const canvas = {} as OffscreenCanvas
    worker.post({ type: 'init', canvas, width: 2, height: 1, dpr: 1 }, [canvas])
    expect(FakeWorker.last.posted).toEqual([[{ type: 'clear' }, []], [{ type: 'init', canvas, width: 2, height: 1, dpr: 1 }, [canvas]]])
    FakeWorker.last.onmessage?.(new MessageEvent('message', { data: { type: 'ready' } }))
    FakeWorker.last.onerror?.()
    expect(heard).toEqual([{ type: 'ready' }, { type: 'failed' }])
    worker.stop()
    expect(FakeWorker.last.stopped).toBe(true)
  })
})
