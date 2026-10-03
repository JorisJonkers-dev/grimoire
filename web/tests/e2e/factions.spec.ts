import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

test('a DM adds a Faction from the catalogue and decides its Standing Changes; a Player sees tiers and shared reasons', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Factions ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-faction-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')

  await page.getByTestId('factions-link').click()
  await expect(page.getByTestId('no-factions')).toBeVisible()
  const add = page.getByTestId('faction-add')
  await add.getByTestId('faction-archetype').selectOption({ label: 'City watch' })
  await expect(add.getByTestId('faction-name')).toHaveValue('City watch')
  await add.getByTestId('faction-name').fill('The Lantern Watch')
  await add.getByTestId('faction-notes').fill('The captain takes bribes.')
  await add.getByRole('button', { name: 'Add Faction' }).click()
  const card = page.locator('[data-testid^="faction-0"], [data-testid^="faction-1"], section.faction').filter({ hasText: 'The Lantern Watch' }).first()
  await expect(card.getByTestId('tier')).toHaveText('Neutral')
  await expect(card.getByTestId('faction-secrets')).toContainText('Score 0')

  // A suggestion waits; nothing moves until the DM confirms it.
  const suggest = async (delta: string, reason: string, share: boolean) => {
    await card.getByTestId('suggest-delta').fill(delta)
    await card.getByTestId('suggest-reason').fill(reason)
    await card.getByTestId('suggest-share').setChecked(share)
    await card.getByRole('button', { name: 'Suggest' }).click()
    await expect(card.getByText(reason)).toBeVisible()
  }
  await suggest('30', 'Returned the stolen seal.', true)
  await expect(card.getByTestId('tier')).toHaveText('Neutral')
  await player.getByTestId('factions-link').click()
  const seen = player.locator('section.faction').filter({ hasText: 'The Lantern Watch' })
  await expect(seen.getByTestId('tier')).toHaveText('Neutral')
  await expect(seen.locator('[data-testid^="change-"]')).toHaveCount(0)

  await card.getByTestId('decide-confirm').click()
  await expect(card.getByTestId('tier')).toHaveText('Friendly')
  await expect(card.getByTestId('faction-secrets')).toContainText('Score 30')
  // The second is confirmed at less than was suggested, and its reason kept from the Players.
  await suggest('-70', 'Killed a watchman.', false)
  await card.getByTestId('decide-delta').fill('-50')
  await card.getByTestId('decide-confirm').click()
  await expect(card.getByTestId('tier')).toHaveText('Unfriendly')
  await expect(card.getByTestId('faction-secrets')).toContainText('Score -20')
  // A third is dismissed and moves nothing.
  await suggest('60', 'A rumour of their bravery.', true)
  await card.getByTestId('decide-dismiss').click()
  await expect(card.getByText('Dismissed: rose by 60. A rumour of their bravery.')).toBeVisible()
  await expect(card.getByTestId('tier')).toHaveText('Unfriendly')
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])

  // The Player: the tier, which way each confirmed change went, the one shared reason, and no number.
  await player.reload()
  await expect(seen.getByTestId('tier')).toHaveText('Unfriendly')
  await expect(seen.locator('[data-testid^="change-"]')).toHaveText(['Standing fell.', 'Standing rose: Returned the stolen seal.'])
  const text = await seen.innerText()
  expect(text).not.toMatch(/\d/)
  for (const secret of ['watchman', 'bribes', 'rumour', 'Score']) expect(text).not.toContain(secret)
  await expect(player.getByTestId('faction-add')).toHaveCount(0)
  await expect(player.getByTestId('suggest')).toHaveCount(0)
  expect((await new AxeBuilder({ page: player }).analyze()).violations).toEqual([])
})
