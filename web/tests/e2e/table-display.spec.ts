import AxeBuilder from '@axe-core/playwright'
import { devices, expect, type Locator, type Page, test } from '@playwright/test'

async function enter(card: Locator, face: string) {
  await card.getByTestId('face-0').fill(face)
  await card.getByTestId('face-0').press('Enter')
}

async function place(page: Page, hex: string, label: string, kind: string, controller = '', hidden = false) {
  await page.getByTestId('token-monster').fill('goblin-warrior')
  await page.getByTestId('token-kind').selectOption(kind)
  await page.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
  await page.getByTestId('token-label').fill(label)
  await page.getByTestId('token-hidden').setChecked(hidden)
  await page.locator(`[data-hex="${hex}"]`).click()
  await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
}

test('the TV shows the party\'s view: roster, caption, the last roll, the reveal and whose turn it is', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'tv', 'the Table Display is for the TV')
  const stamp = String(Date.now())
  const desk = { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 900 } }
  const dm = await (await browser.newContext({ ...desk, extraHTTPHeaders: { 'X-User-Id': `e2e-tv-dm-${stamp}` } })).newPage()
  await dm.goto('/campaigns')
  await dm.getByTestId('campaign-name').fill(`Table ${stamp}`)
  await dm.getByTestId('campaign-display-name').fill('DM')
  await dm.getByRole('button', { name: 'Start as DM' }).click()
  await dm.getByTestId('create-invite').click()
  const link = new URL(await dm.getByTestId('invite-link').inputValue())
  // The TV is signed in as a player; the same player also holds a device of their own.
  await page.goto(link.pathname + link.hash)
  await page.getByTestId('join-display-name').fill('Aria')
  await page.getByRole('button', { name: 'Join as Player' }).click()
  await expect(page.getByTestId('member-list')).toContainText('Aria (you)')
  await dm.getByTestId('start-session').click()
  await expect(dm.getByTestId('connection')).toHaveText('Live')
  const sessionUrl = new URL(dm.url()).pathname
  await page.goto(`${sessionUrl}/table`)
  const player = await (await browser.newContext(desk)).newPage()
  await player.goto(sessionUrl)

  await place(dm, '0,0', 'Scout', 'party', 'Aria')
  await place(dm, '1,0', 'Grik', 'enemy')
  await place(dm, '2,-1', 'Lurker', 'enemy', '', true)
  const tv = page.getByTestId('table-display')
  await expect(tv.getByTestId('roster-strip').getByRole('listitem')).toHaveCount(2)
  await expect(tv.getByTestId('health-Grik')).toHaveAttribute('aria-label', 'unhurt')

  await dm.getByTestId('table-caption-text').fill('The gate creaks open.')
  await dm.getByTestId('show-caption').click()
  await expect(tv.getByTestId('table-caption')).toHaveText('The gate creaks open.')

  await dm.getByTestId('choose-combatants').click()
  await dm.getByTestId('begin-combat').click()
  await enter(dm.getByTestId('roll-card').filter({ hasText: 'Initiative for Grik' }), '20')
  await enter(dm.getByTestId('roll-card').filter({ hasText: 'Initiative for Lurker' }), '15')
  // The DM's rolls never show on the TV; the player's does, with its die.
  await expect(tv.getByTestId('table-roll')).toHaveCount(0)
  await enter(player.getByTestId('roll-card'), '5')
  await Promise.all([
    expect(tv.getByTestId('table-roll')).toContainText('Aria · Initiative for Scout'),
    expect(tv.getByTestId('table-roll').locator('.die')).toHaveText('5'),
    expect(tv.getByTestId('rolled-Scout')).toHaveText('5'),
    expect(tv.getByTestId('rolled-Grik')).toHaveText('20'),
    expect(tv.getByTestId('table-turn')).toHaveText("Grik's turn"),
  ])
  await expect(tv.getByTestId('rolled-Lurker')).toHaveCount(0)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await expect(tv.getByTestId('table-turn')).toHaveCount(0)

  // A hidden creature's turn passes without a word on the TV.
  await dm.getByTestId('control-Grik').click()
  await dm.getByTestId('turn-Grik').getByTestId('end-turn').click()
  await expect(dm.getByTestId('turn-Lurker')).toBeVisible()
  await expect(tv.getByTestId('rail-Grik')).not.toHaveAttribute('aria-current', 'step')
  await expect(tv.getByTestId('table-turn')).toHaveCount(0)
  await dm.getByTestId('turn-Lurker').getByTestId('end-turn').click()
  await expect(tv.getByTestId('table-turn')).toHaveText("Scout's turn")

  // Nothing of the DM's reaches the TV.
  await expect(tv).not.toContainText('Lurker')
  await expect(tv.locator('[data-testid^="caption-"]')).toHaveCount(0)
  await expect(tv.locator('[data-hex="1,0"]')).not.toHaveAttribute('aria-label', /suggested|HP/)
  await dm.getByTestId('clear-caption').click()
  await expect(tv.getByTestId('table-caption')).toHaveCount(0)
})
