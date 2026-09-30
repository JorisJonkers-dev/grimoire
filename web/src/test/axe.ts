import axe from 'axe-core'
import { expect, vi } from 'vitest'

/** jsdom cannot compute colour, so contrast is checked by Playwright + axe in a real browser instead. */
export async function expectAccessible(element: Element): Promise<void> {
  // axe schedules its work with timers: under a fake clock it never finishes and blocks every later run.
  if (vi.isFakeTimers()) throw new Error('Restore real timers before checking accessibility.')
  const results = await axe.run(element, { rules: { 'color-contrast': { enabled: false } } })
  expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
}
