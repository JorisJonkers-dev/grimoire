import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { crypt } from './maps'

test('the party splits across two maps, each group on its own screen, and comes back together', async ({ page, browser }, info) => {
  test.skip(info.project.name !== 'desktop', 'one multi-client run is enough')
  test.slow()
  const stamp = String(Date.now())
  await page.goto('/campaigns')
  await page.getByTestId('campaign-name').fill(`Split ${stamp}`)
  await page.getByTestId('campaign-display-name').fill('DM')
  await page.getByRole('button', { name: 'Start as DM' }).click()
  await page.getByTestId('create-invite').click()
  const link = new URL(await page.getByTestId('invite-link').inputValue())
  const campaign = new URL(page.url()).pathname
  for (const name of ['Crypt', 'Tower']) {
    const made = await page.request.post(`/api/v1${campaign}/maps?name=${name}`, { data: crypt, headers: { 'Content-Type': 'application/octet-stream' } })
    expect(made.status()).toBe(201)
  }
  const player = await (await browser.newContext({ extraHTTPHeaders: { 'X-User-Id': `e2e-split-${stamp}` } })).newPage()
  await player.goto(link.pathname + link.hash)
  await player.getByTestId('join-display-name').fill('Aria')
  await player.getByRole('button', { name: 'Join as Player' }).click()
  await expect(player.getByTestId('member-list')).toContainText('Aria (you)')
  await page.getByTestId('start-session').click()
  await expect(page.getByTestId('connection')).toHaveText('Live')
  const home = new URL(page.url()).pathname
  await player.goto(home)
  await expect(player.getByTestId('connection')).toHaveText('Live')

  // Scout is Aria's; Brom is the DM's own; a goblin lurks in the crypt.
  for (const [label, kind, controller, hex] of [['Scout', 'party', 'Aria', '0,0'], ['Brom', 'party', '', '1,0'], ['Grik', 'enemy', '', '2,0']] as const) {
    await page.getByTestId('token-monster').fill('goblin-warrior')
    await page.getByTestId('token-kind').selectOption(kind)
    await page.getByTestId('token-controller').selectOption(controller ? { label: controller } : '')
    await page.getByTestId('token-label').fill(label)
    await page.locator(`[data-hex="${hex}"]`).click()
    await expect(page.locator(`[data-hex="${hex}"]`)).toHaveAttribute('aria-label', new RegExp(label))
  }
  await expect(player.locator('[data-hex="2,0"]')).toHaveAttribute('aria-label', /Grik/)

  // Scout goes up the tower: Aria's screen follows, and sees nothing more of the crypt.
  const groups = page.getByTestId('groups')
  await groups.getByTestId('group-name').fill('The tower')
  await groups.getByTestId('goes-Scout').check()
  await groups.getByTestId('group-map').selectOption({ label: 'Tower' })
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
  await groups.getByTestId('group-split').click()
  const tower = groups.getByRole('listitem').filter({ hasText: 'The tower' })
  await expect(tower).toContainText('Scout')
  await expect(page.locator('[data-hex="0,0"]')).not.toHaveAttribute('aria-label', /Scout/)
  await expect(player).not.toHaveURL(new RegExp(`${home}$`))
  await expect(player.getByTestId('connection')).toHaveText('Live')
  await expect(player.getByTestId('tokens')).toContainText('Scout')
  await expect(player.getByTestId('tokens')).not.toContainText('Grik')
  await expect(player.getByTestId('tokens')).not.toContainText('Brom')
  const away = new URL(player.url()).pathname
  // Going back to the crypt by hand brings Aria's screen to the tower again.
  await player.goto(home)
  await expect(player).toHaveURL(new RegExp(`${away}$`))
  await expect(player.getByTestId('tokens')).toContainText('Scout')

  // The DM goes from group to group.
  await tower.getByRole('link', { name: 'Go to this group' }).click()
  await expect(page).toHaveURL(new RegExp(`${away}$`))
  await expect(page.getByTestId('tokens')).toContainText('Scout')
  await expect(page.getByTestId('tokens')).not.toContainText('Grik')
  await expect(page.getByTestId('groups-away')).toBeVisible()
  await page.getByTestId('groups').getByRole('listitem').filter({ hasText: 'The party' }).getByRole('link', { name: 'Go to this group' }).click()
  await expect(page).toHaveURL(new RegExp(`${home}$`))

  // The Table Display follows the tower, then the party again.
  const tv = await page.context().newPage()
  await tv.goto(`${home}/table`)
  await expect(tv.getByTestId('table-display')).toBeVisible()
  await page.getByTestId('groups').getByRole('listitem').filter({ hasText: 'The tower' }).getByLabel('Table Display follows').check()
  await expect(tv).toHaveURL(new RegExp(`${away}/table$`))
  await page.getByTestId('groups').getByRole('listitem').filter({ hasText: 'The party' }).getByLabel('Table Display follows').check()
  await expect(tv).toHaveURL(new RegExp(`${home}/table$`))

  // The group comes back: one party, one Session, and Aria's screen with it.
  await page.getByTestId('groups').getByRole('button', { name: 'Bring The tower back' }).click()
  await expect(page.getByTestId('tokens')).toContainText('Scout')
  await expect(page.getByTestId('groups').getByRole('listitem')).toHaveCount(0)
  await expect(player).toHaveURL(new RegExp(`${home}$`))
  await expect(player.getByTestId('tokens')).toContainText('Brom')
  await player.context().close()
})
