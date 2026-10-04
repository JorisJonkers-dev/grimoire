import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

const focusedHex = (page: Page) => page.evaluate(() => document.activeElement?.getAttribute('data-hex') ?? null)

test('a player takes a turn with the keyboard alone', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Keys ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-keys-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await player.goto(new URL(page.url()).pathname)
  await page.getByTestId('token-label').fill('Aria')
  await page.getByTestId('token-kind').selectOption('party')
  await page.getByTestId('token-controller').selectOption({ label: 'Aria' })
  await page.locator('[data-hex="0,0"]').click()
  await page.getByTestId('token-label').fill('Goblin')
  await page.getByTestId('token-kind').selectOption('enemy')
  await page.getByTestId('token-controller').selectOption('')
  await page.locator('[data-hex="2,-2"]').click()
  await expect(page.locator('[data-hex="2,-2"]')).toHaveAttribute('aria-label', /Goblin/)
  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()

  // From here on neither screen is touched with the mouse. R rolls the dice waiting on each.
  await expect(page.getByTestId('roll-rest')).toBeVisible()
  await page.keyboard.press('r')
  await expect(player.getByTestId('roll-rest')).toBeVisible()
  await player.keyboard.press('r')
  await expect(player.getByTestId('initiative-rail')).toContainText('Round 1')
  // Whoever won the roll, the DM ends the Goblin's turn with E when it comes first.
  if (await page.getByTestId('turn-Goblin').isVisible()) await page.keyboard.press('e')
  await expect(player.getByTestId('turn-Aria')).toBeVisible()

  // M finds my Character on the map; the arrows walk the hexes; Enter plans the walk; C confirms it.
  await player.keyboard.press('m')
  expect(await focusedHex(player)).toBe('0,0')
  await player.keyboard.press('ArrowRight')
  expect(await focusedHex(player)).toBe('1,0')
  await player.keyboard.press('ArrowDown')
  await player.keyboard.press('ArrowUp')
  expect(await focusedHex(player)).toBe('1,0')
  await player.keyboard.press('Enter')
  await expect(player.getByTestId('confirm-walk')).toBeVisible()
  // Escape takes the plan back; planning again and C walks it.
  await player.keyboard.press('Escape')
  await expect(player.getByTestId('confirm-walk')).toHaveCount(0)
  await player.keyboard.press('Enter')
  await expect(player.getByTestId('confirm-walk')).toBeVisible()
  await player.keyboard.press('c')
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Aria/)
  await expect(player.getByTestId('confirm-walk')).toHaveCount(0)
  // M follows the Character to where it stands now.
  await player.keyboard.press('m')
  expect(await focusedHex(player)).toBe('1,0')

  // ? lists the keys this page answers to, and Escape puts the list away.
  await player.keyboard.press('?')
  const keys = player.getByTestId('key-map')
  await expect(keys).toBeVisible()
  await expect(keys.getByTestId('key-row').filter({ hasText: 'End turn' })).toHaveText(/^E\s*End turn$/)
  await expect(keys.getByTestId('key-map-close')).toBeFocused()
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])
  // While the list shows, the keys behind it do nothing.
  await player.keyboard.press('e')
  await expect(player.getByTestId('turn-Aria')).toBeVisible()
  await player.keyboard.press('Escape')
  await expect(keys).toHaveCount(0)

  // E ends the turn.
  await player.keyboard.press('e')
  await expect(player.getByTestId('turn-Aria')).toHaveCount(0)
  await expect(page.getByTestId('turn-Goblin')).toBeVisible()
})

test('the keys are listed on any page, and / goes to the search', async ({ page }) => {
  await page.goto('/compendium/spells')
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
  await page.keyboard.press('/')
  await expect(page.getByTestId('header-search-input')).toBeFocused()
  await page.keyboard.type('fire')
  await expect(page.getByTestId('header-search-input')).toHaveValue('fire')
  await page.keyboard.press('Escape')
  await page.getByTestId('header-search-input').blur()
  await page.keyboard.press('?')
  await expect(page.getByTestId('key-map')).toBeVisible()
  await expect(page.getByTestId('key-map').getByTestId('key-row').filter({ hasText: 'Search everything' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.keyboard.press('Escape')
  await expect(page.getByTestId('key-map')).toHaveCount(0)
})
