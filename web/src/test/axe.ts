import axe from 'axe-core'
import { expect } from 'vitest'

/** jsdom cannot compute colour, so contrast is checked by Playwright + axe in a real browser instead. */
export async function expectAccessible(element: Element): Promise<void> {
  const results = await axe.run(element, { rules: { 'color-contrast': { enabled: false } } })
  expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([])
}
