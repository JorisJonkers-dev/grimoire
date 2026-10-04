import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

const KEY = 'grimoire.accessibility'
const sizeOf = (page: Page, selector: string) => page.locator(selector).first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize))
const rootColour = (page: Page, name: string) => page.evaluate((n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim().toUpperCase(), name)

test('each accessibility setting changes the whole app, and is kept on the device', async ({ page }) => {
  await page.goto('/')
  await page.getByTestId('accessibility-link').click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Accessibility')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // Text size: headings, body text and controls all grow by the same step.
  const texts = ['h1', 'footer', '[data-testid="reset-accessibility"]', '[data-testid="header-search-input"]', 'legend']
  const before = await Promise.all(texts.map((s) => sizeOf(page, s)))
  await page.getByTestId('text-largest').check()
  const after = await Promise.all(texts.map((s) => sizeOf(page, s)))
  after.forEach((size, i) => { expect(size / (before[i] ?? 1), texts[i]).toBeCloseTo(1.3, 2) })
  await page.getByTestId('text-large').check()
  expect((await sizeOf(page, 'h1')) / (before[0] ?? 1)).toBeCloseTo(1.15, 2)

  // Colours: the palette changes what tells allies from enemies.
  expect(await rootColour(page, '--color-enemy')).toBe('#D0692A')
  await page.getByTestId('palette-red-green').check()
  expect(await rootColour(page, '--color-enemy')).toBe('#E69F00')
  await expect(page.locator('.chip--enemy')).toHaveCSS('color', 'rgb(230, 159, 0)')
  await expect(page.locator('.chip--party')).toHaveCSS('color', 'rgb(86, 180, 233)')
  await page.getByTestId('palette-blue-yellow').check()
  await expect(page.locator('.chip--success')).toHaveCSS('color', 'rgb(61, 220, 151)')

  // The font is loaded and every text is set in it.
  await page.getByTestId('dyslexia-font').check()
  for (const s of ['h1', 'footer', 'legend']) await expect(page.locator(s).first()).toHaveCSS('font-family', /^OpenDyslexic/)
  await expect.poll(() => page.evaluate(() => document.fonts.check('16px OpenDyslexic'))).toBe(true)

  // Motion: asking the app stills what the device's own setting would.
  await expect(page.locator('footer')).not.toHaveCSS('transition-duration', '0.001s')
  await page.getByTestId('reduce-motion').check()
  await expect(page.locator('footer')).toHaveCSS('transition-duration', '0.001s')
  await expect(page.locator('footer')).toHaveCSS('animation-duration', '0.001s')

  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  // Kept on the device: a fresh load of another page starts as this one was left.
  await page.goto('/compendium/spells')
  await expect(page.locator('html')).toHaveAttribute('data-palette', 'blue-yellow')
  await expect(page.locator('html')).toHaveAttribute('data-text', 'large')
  await expect(page.locator('html')).toHaveAttribute('data-font', 'dyslexia')
  await expect(page.locator('html')).toHaveAttribute('data-motion', 'reduced')
  await expect(page.locator('h1')).toHaveCSS('font-family', /^OpenDyslexic/)
})

for (const palette of ['red-green', 'blue-yellow'] as const) {
  test(`every surface outside a Campaign passes axe in the ${palette} palette at the largest text, and nothing runs off the side`, async ({ page }) => {
    test.slow()
    await page.addInitScript(([key, value]) => { localStorage.setItem(key ?? '', value ?? '') }, [KEY, JSON.stringify({ palette, textSize: 'largest', dyslexiaFont: palette === 'blue-yellow' })])
    for (const path of ['/', '/accessibility', '/compendium/spells', '/compendium/spells/fireball', '/compendium/monster', '/compendium/monster/goblin', '/compendium/guides', '/about/attribution', '/about/automation', '/sign-in', '/campaigns', '/search?q=fire', '/library', '/shared-library', '/characters', '/friends', '/dice-sets', '/conversations']) {
      await page.goto(path)
      await expect(page.locator('html')).toHaveAttribute('data-palette', palette)
      await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible()
      expect((await new AxeBuilder({ page }).analyze()).violations, path).toEqual([])
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), `${path} overflows`).toBeLessThanOrEqual(0)
    }
  })
}

async function enter(page: Page, face: string) {
  const card = page.getByTestId('roll-card')
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

test('a screen reader is told each turn and roll in live play, and the app\'s reduced motion skips the reveal', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Heard ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const user = { 'X-User-Id': `e2e-heard-${stamp}` }
  const player = await (await browser.newContext({ extraHTTPHeaders: user })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(page.url()).pathname
  await player.goto(sessionUrl)
  // The same player on a second device, set in the app to reduce motion and to hear no rolls.
  const calmContext = await browser.newContext({ extraHTTPHeaders: user })
  await calmContext.addInitScript(([key, value]) => { localStorage.setItem(key ?? '', value ?? '') }, [KEY, JSON.stringify({ reduceMotion: true, announceRolls: false })])
  const calm = await calmContext.newPage()
  await calm.goto(sessionUrl)

  await page.getByTestId('token-label').fill('Aria')
  await page.getByTestId('token-kind').selectOption('party')
  await page.getByTestId('token-controller').selectOption({ label: 'Aria' })
  await page.locator('[data-hex="0,0"]').click()
  await page.getByTestId('token-label').fill('Goblin')
  await page.getByTestId('token-kind').selectOption('enemy')
  await page.getByTestId('token-controller').selectOption('')
  await page.locator('[data-hex="3,0"]').click()
  await expect(page.locator('[data-hex="3,0"]')).toHaveAttribute('aria-label', /Goblin/)

  const heard = player.getByTestId('announcer')
  await expect(heard).toHaveAttribute('role', 'log')
  await expect(heard).toHaveAttribute('aria-live', 'polite')
  await expect(heard.locator('p')).toHaveCount(0)

  await page.getByTestId('choose-combatants').click()
  await page.getByTestId('begin-combat').click()
  await enter(page, '5')
  await expect(player.getByTestId('roll-card')).toContainText('Initiative for Aria')
  await enter(player, '15')

  // The player's own roll and the turn it won are read out; the log is in the page for a screen reader and seen by nobody.
  await expect(heard.locator('p')).toHaveText(['Aria rolled 15 for Initiative for Aria.', 'Round 1. Your turn: Aria.'])
  const box = await heard.boundingBox()
  expect(box?.width).toBeLessThanOrEqual(1)
  expect(box?.height).toBeLessThanOrEqual(1)
  expect(await player.getByRole('log', { name: 'What is happening' }).count()).toBe(1)
  // The device set to reduce motion goes straight to the order, and hears the turn alone.
  await expect(calm.getByTestId('turn-banner')).toContainText('Aria')
  await expect(calm.getByTestId('rolled-Aria')).toHaveCount(0)
  await expect(calm.getByTestId('turn-banner')).toHaveCSS('animation-name', 'none')
  await expect(calm.getByTestId('announcer').locator('p')).toHaveText(['Round 1. Your turn: Aria.'])
  // The DM hears who acts by name.
  await expect(page.getByTestId('announcer').locator('p').last()).toHaveText('Round 1. Aria acts.')
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])

  await player.getByTestId('turn-Aria').getByTestId('end-turn').click()
  await expect(heard.locator('p').last()).toHaveText('Round 1. Goblin acts.')
  await page.getByTestId('turn-Goblin').getByTestId('end-turn').click()
  await expect(heard.locator('p').last()).toHaveText('Round 2. Your turn: Aria.')
  await expect(page.getByTestId('announcer').locator('p').last()).toHaveText('Round 2. Aria acts.')
})
