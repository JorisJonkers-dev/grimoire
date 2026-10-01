import { Capacitor } from '@capacitor/core'
import { KeepAwake } from '@capacitor-community/keep-awake'
import { onBeforeUnmount, watch } from 'vue'

type Lock = { release(): Promise<void> }

// screenLock holds the screen on: natively in the Android shell, through the Wake Lock API elsewhere.
async function screenLock(onLost: () => void): Promise<Lock | null> {
  if (Capacitor.isNativePlatform()) {
    await KeepAwake.keepAwake()
    return { release: () => KeepAwake.allowSleep() }
  }
  if (!('wakeLock' in navigator)) return null
  const lock = await navigator.wakeLock.request('screen')
  lock.addEventListener('release', onLost)
  return lock
}

// useWakeLock keeps the screen on while active() holds and the page is in view; the browser drops
// the lock whenever the page is hidden, so it is taken again on return.
export function useWakeLock(active: () => boolean) {
  let lock: Lock | null = null
  let stopped = false
  async function sync() {
    const want = !stopped && active() && document.visibilityState === 'visible'
    if (want && !lock) {
      lock = await screenLock(() => {
        lock = null
      }).catch(() => null)
    } else if (!want && lock) {
      const held = lock
      lock = null
      await held.release()
    }
  }
  const onVisible = () => void sync()
  watch(active, () => void sync(), { immediate: true })
  document.addEventListener('visibilitychange', onVisible)
  onBeforeUnmount(() => {
    stopped = true
    document.removeEventListener('visibilitychange', onVisible)
    void sync()
  })
}
