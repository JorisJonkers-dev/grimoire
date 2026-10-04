/** Gives the test's browser a vibration motor and returns what it was asked to buzz; `still` takes it away again. */
export function vibrating(): unknown[] {
  const buzzed: unknown[] = []
  Object.defineProperty(navigator, 'vibrate', { configurable: true, value: (pattern: unknown) => { buzzed.push(pattern); return true } })
  return buzzed
}

export function still() {
  Reflect.deleteProperty(navigator, 'vibrate')
}
