import AxeBuilder from '@axe-core/playwright'
import type { Page } from '@playwright/test'

/**
 * What axe finds wrong on the page. A map draws hundreds of identical cells, which make a scan take
 * half a minute on CI, so only the cell at 0,0 and the cells that show text are checked.
 */
export async function violations(page: Page) {
  const results = await new AxeBuilder({ page }).exclude('[data-hex]:not([data-hex="0,0"]):not(:has(text))').analyze()
  return results.violations
}
