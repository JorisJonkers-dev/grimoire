import AxeBuilder from '@axe-core/playwright'
import { expect, type Page, test } from '@playwright/test'

async function place(dm: Page, label: string, kind: 'party' | 'enemy', hex: string) {
  await dm.getByTestId('token-monster').fill('goblin-warrior')
  await dm.getByTestId('token-kind').selectOption(kind)
  await dm.getByTestId('token-label').fill(label)
  await dm.locator(`[data-hex="${hex}"]`).click()
  await expect(dm.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
}

test('the DM keeps a checkpoint and rewinds the Session to it on every screen', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Rewind ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-rewind-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  await player.goto(new URL(page.url()).pathname)
  await expect(player.getByTestId('connection')).toHaveText('Live')
  // The Checkpoints are the DM's alone.
  await expect(player.getByTestId('checkpoints')).toHaveCount(0)

  await place(page, 'Scout', 'party', '0,0')
  await expect(page.getByTestId('checkpoints-none')).toBeVisible()
  await page.getByTestId('checkpoint-name').fill('Before the ambush')
  await page.getByTestId('checkpoint-keep').click()
  const kept = page.getByTestId('checkpoints').getByRole('listitem').filter({ hasText: 'Before the ambush' })
  await expect(kept).toBeVisible()

  await place(page, 'Grik', 'enemy', '1,0')
  await expect(player.locator('[data-hex="1,0"]')).toHaveAttribute('aria-label', /Grik/)

  // A rewind asks first, then takes back everything since on both screens.
  await kept.getByRole('button', { name: 'Rewind to Before the ambush' }).click()
  await expect(page.getByTestId('rewind-confirm')).toContainText('Everything done since is taken back')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await page.getByTestId('rewind-yes').click()
  for (const screen of [page, player]) {
    await expect(screen.locator('[data-hex="1,0"]')).not.toHaveAttribute('aria-label', /Grik/)
    await expect(screen.locator('[data-hex="0,0"]')).toHaveAttribute('aria-label', /Scout/)
    await expect(screen.getByTestId('connection')).toHaveText('Live')
  }
  // The log keeps what happened, and offers nothing the rewind took back for undo.
  const log = page.getByTestId('action-log')
  await expect(log).toContainText('session rewound')
  await expect(log.getByRole('listitem').filter({ hasText: 'token placed: Grik' }).getByRole('button')).toHaveCount(0)
  await expect(log.getByRole('listitem').filter({ hasText: 'token placed: Scout' }).getByRole('button', { name: /Undo/ })).toHaveCount(1)
  await player.context().close()
})
