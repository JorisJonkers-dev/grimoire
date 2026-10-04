import { reducedMotion } from '@/shared/a11y/settings'
import type { FromWorker, ToWorker } from './diceWorker'

/** Remembered for the tab once a device proved too slow, or had no WebGL: it gets the 2D dice from then on. */
const FLAT_KEY = 'grimoire.dice.flat'

/** Whether this screen should try the GPU dice: it has workers and offscreen canvases, wants motion, and has not failed before. */
export function wantsGPU(): boolean {
  const offscreen = typeof HTMLCanvasElement !== 'undefined' && 'transferControlToOffscreen' in HTMLCanvasElement.prototype
  return typeof Worker === 'function' && offscreen && !reducedMotion() && sessionStorage.getItem(FLAT_KEY) === null
}

/** Remembers that this screen draws its dice flat. */
export function goFlat() {
  sessionStorage.setItem(FLAT_KEY, '1')
}

export type DiceWorker = { post: (m: ToWorker, transfer?: Transferable[]) => void; stop: () => void }

/** Starts the worker that draws the GPU dice. */
export function startWorker(onMessage: (m: FromWorker) => void): DiceWorker {
  const worker = new Worker(new URL('./diceWorker.ts', import.meta.url), { type: 'module' })
  worker.onmessage = (e: MessageEvent<FromWorker>) => { onMessage(e.data) }
  worker.onerror = () => { onMessage({ type: 'failed' }) }
  return { post: (m, transfer = []) => { worker.postMessage(m, transfer) }, stop: () => { worker.terminate() } }
}
