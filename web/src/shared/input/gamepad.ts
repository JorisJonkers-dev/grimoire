// A gamepad plays through the keyboard's shortcuts: each button stands for a key.

export type PadState = { buttons: readonly boolean[]; axes: readonly number[] }

/** The key each button of a standard gamepad stands for: A, B, X, Y, the bumpers, the triggers, Back, Start and the D-pad. */
const BUTTONS: Record<number, string> = {
  0: 'Enter', 1: 'Escape', 2: 'E', 3: 'M', 4: 'PreviousRegion', 5: 'NextRegion', 6: 'C', 7: 'R', 8: '?', 9: '?',
  12: 'ArrowUp', 13: 'ArrowDown', 14: 'ArrowLeft', 15: 'ArrowRight',
}
const HALFWAY = 0.5

// The left stick is a second D-pad once it is pushed past halfway.
function stick(axes: readonly number[]): string[] {
  const [x = 0, y = 0] = axes
  return [x < -HALFWAY ? 'ArrowLeft' : '', x > HALFWAY ? 'ArrowRight' : '', y < -HALFWAY ? 'ArrowUp' : '', y > HALFWAY ? 'ArrowDown' : ''].filter(Boolean)
}

/** The keys a gamepad pressed since it was last read: one for each button newly down and each new push of the stick. */
export function padKeys(before: PadState, now: PadState): string[] {
  const buttons = now.buttons.flatMap((down, i) => (down && !before.buttons[i] && BUTTONS[i] ? [BUTTONS[i]] : []))
  const held = stick(before.axes)
  return [...buttons, ...stick(now.axes).filter((key) => !held.includes(key))]
}

/** How often a connected gamepad is read. */
export const POLL_MS = 50

/** Reads the first connected gamepad while one is connected, handing each key it presses to `onKey`; returns how to stop. */
export function watchGamepads(onKey: (key: string) => void): () => void {
  const idle: PadState = { buttons: [], axes: [] }
  let before = idle
  let timer: ReturnType<typeof setInterval> | undefined
  const read = (): PadState | null => {
    const pad = (typeof navigator.getGamepads === 'function' ? [...navigator.getGamepads()] : []).find((p) => p?.connected)
    return pad ? { buttons: pad.buttons.map((b) => b.pressed), axes: [...pad.axes] } : null
  }
  const poll = () => {
    const now = read()
    if (!now) {
      stop()
      return
    }
    for (const key of padKeys(before, now)) onKey(key)
    before = now
  }
  const start = () => {
    if (timer === undefined && read()) timer = setInterval(poll, POLL_MS)
  }
  const stop = () => {
    clearInterval(timer)
    timer = undefined
    before = idle
  }
  window.addEventListener('gamepadconnected', start)
  start()
  return () => {
    window.removeEventListener('gamepadconnected', start)
    stop()
  }
}
