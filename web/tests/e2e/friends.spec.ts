import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'
import { account } from './accounts'

test('two people become Friends from the Friends page', async ({ browser, request }, info) => {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the Friends page is checked on phone and desktop')
  const stamp = `${info.project.name}-${String(Date.now())}`
  const asker = await account(browser, request, `ask-${stamp}`, 'Asker')
  const friend = await account(browser, request, `fri-${stamp}`, 'Friendly')

  await asker.page.getByTestId('friends-link').click()
  await asker.page.getByTestId('friend-username').fill(`fri-${stamp}`)
  await asker.page.getByRole('button', { name: 'Send a request' }).click()
  await expect(asker.page.getByTestId('friend-request-sent')).toBeVisible()
  await expect(asker.page.getByTestId('friend-outgoing')).toContainText('Friendly')
  expect((await new AxeBuilder({ page: asker.page }).analyze()).violations).toEqual([])
  expect(await asker.page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)

  await friend.page.goto('/friends')
  await friend.page.getByTestId(`accept-ask-${stamp}`).click()
  await expect(friend.page.getByTestId(`friend-ask-${stamp}`)).toContainText('Asker')
  await asker.page.reload()
  await expect(asker.page.getByTestId(`friend-fri-${stamp}`)).toContainText('Friendly')
  await asker.context.close()
  await friend.context.close()
})
