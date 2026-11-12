import { expect, test, type TestInfo } from '@playwright/test'
import { createAccountThroughEstate, estateLogin, estateNames, signInThroughEstate } from './estate'

// These journeys go through the fake jorisjonkers.dev the suite runs beside the server (cmd/fakeoidc);
// nothing reaches the real provider.
function onPhoneAndDesktop(info: TestInfo) {
  test.skip(!['phone', 'desktop'].includes(info.project.name), 'the sign-in page is checked on phone and desktop')
}

test('someone holding the Grimoire grant signs in through jorisjonkers.dev for the first time and lands signed in', async ({ page }, info) => {
  onPhoneAndDesktop(info)
  const login = estateLogin('player', info)
  await signInThroughEstate(page, login)
  await expect(page.getByTestId('oidc-choose')).toContainText(`Welcome, ${estateNames.player}.`)
  await expect(page.getByTestId('oidc-username')).toHaveValue(login.replace('+', '-'))
  await page.getByRole('button', { name: 'Create the Account' }).click()
  await expect(page.getByTestId('account-link')).toHaveText(estateNames.player)
  await expect(page).not.toHaveURL(/\/(oidc\/callback|sign-in)/)
  await expect(page.getByTestId('admin-link')).toHaveCount(0)
})

test('someone whose login already has an Account signs straight in through jorisjonkers.dev', async ({ page }, info) => {
  onPhoneAndDesktop(info)
  const login = estateLogin('player', info)
  await createAccountThroughEstate(page, login)
  await expect(page.getByTestId('account-link')).toHaveText(estateNames.player)
  await page.getByTestId('account-menu-toggle').click()
  await page.getByTestId('sign-out').click()
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible()

  await signInThroughEstate(page, login)
  await expect(page.getByTestId('account-link')).toHaveText(estateNames.player)
  await expect(page.getByTestId('oidc-choose')).toHaveCount(0)
})

test('someone without the Grimoire grant is refused at jorisjonkers.dev sign-in', async ({ page }, info) => {
  onPhoneAndDesktop(info)
  await signInThroughEstate(page, estateLogin('outsider', info))
  await expect(page.getByTestId('oidc-failed')).toContainText('does not have access to Grimoire')
  await expect(page.getByTestId('oidc-choose')).toHaveCount(0)
  await expect(page.getByTestId('account-link')).toHaveCount(0)
})

test('an estate admin who signs in through jorisjonkers.dev gets an Admin Account', async ({ page }, info) => {
  onPhoneAndDesktop(info)
  await createAccountThroughEstate(page, estateLogin('admin', info))
  await expect(page.getByTestId('account-link')).toHaveText(estateNames.admin)
  await expect(page.getByTestId('admin-link')).toHaveCount(1)
})
