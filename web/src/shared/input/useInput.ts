import { onBeforeUnmount, onMounted, ref } from 'vue'
import { stepFocus, stepHex, stepRegion } from './focus'
import { watchGamepads } from './gamepad'
import { keyOf, listed, press } from './shortcuts'

// What a gamepad's D-pad and bumpers do off the map, where the keyboard has Tab.
const PAD_MOVES: Record<string, () => void> = {
  ArrowUp: () => { stepFocus(-1) },
  ArrowLeft: () => { stepFocus(-1) },
  ArrowDown: () => { stepFocus(1) },
  ArrowRight: () => { stepFocus(1) },
  PreviousRegion: () => { stepRegion(-1) },
  NextRegion: () => { stepRegion(1) },
  Enter: () => { (document.activeElement as HTMLElement | null)?.click() },
}

/** Listens for shortcut keys and a gamepad for the whole app; `open` says whether the list of keys shows. */
export function useInput() {
  const open = ref(false)
  // What the page answered to as the list was opened: behind the open list the page is inert.
  const rows = ref<ReturnType<typeof listed>>([])
  let back: Element | null = null
  function show(on: boolean) {
    if (on) {
      back = document.activeElement
      rows.value = listed()
    }
    open.value = on
    if (!on) (back as HTMLElement | null)?.focus()
  }
  /** Does what a key stands for, and says whether anything answered to it. */
  function run(key: string, pad: boolean): boolean {
    if (key === '?') {
      show(!open.value)
      return true
    }
    // While the keys show, Escape puts them away and nothing behind them is pressed.
    if (open.value) {
      if (key === 'Escape') show(false)
      return key === 'Escape'
    }
    if (stepHex(key)) return true
    const move = pad ? PAD_MOVES[key] : undefined
    if (move) {
      move()
      return true
    }
    return press(key)
  }
  function onKey(e: KeyboardEvent) {
    const key = keyOf(e)
    if (key && run(key, false)) e.preventDefault()
  }
  let unplug: () => void = () => undefined
  onMounted(() => {
    window.addEventListener('keydown', onKey)
    unplug = watchGamepads((key) => { run(key, true) })
  })
  onBeforeUnmount(() => {
    window.removeEventListener('keydown', onKey)
    unplug()
  })
  return { open, rows, show }
}
