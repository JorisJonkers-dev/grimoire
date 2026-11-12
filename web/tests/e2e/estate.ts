import { expect, type Page, type TestInfo } from '@playwright/test'

/**
 * The kinds of jorisjonkers.dev identity the fake provider knows (`oidctest.People` in
 * api/internal/identity/oidc/oidctest/people.go): with the Grimoire grant, without it, and an estate
 * admin. A new kind is one more Login there and one more name here.
 */
export type EstateKind = 'player' | 'outsider' | 'admin'

/** The display name of each kind, as the fake provider signs it into the ID token. */
export const estateNames: Record<EstateKind, string> = { player: 'Rowan Player', outsider: 'Ash Outsider', admin: 'Sage Admin' }

/**
 * A fresh jorisjonkers.dev identity of a kind for one test: `<kind>+<tag>`, which the fake provider
 * turns into its own subject, email and preferred username, so tests in parallel, on other devices or
 * in a rerun against the same database never meet each other's Accounts.
 */
export function estateLogin(kind: EstateKind, info: TestInfo) {
  return `${kind}+${info.project.name}-${String(info.workerIndex)}-${Date.now().toString(36)}`
}

/** Signs in from Grimoire's sign-in page through the fake jorisjonkers.dev as login, and comes back. */
export async function signInThroughEstate(page: Page, login: string) {
  await page.goto('/sign-in')
  await page.getByTestId('sign-in-oidc').click()
  // Grimoire's own sign-in page has a Username field and a Sign in button too, so wait to be at the provider.
  await page.waitForURL((url) => url.pathname === '/authorize')
  await page.getByLabel('Username').fill(login)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await page.waitForURL((url) => url.pathname !== '/authorize')
}

/** Signs a new jorisjonkers.dev login in for the first time and creates its Account, with the defaults offered. */
export async function createAccountThroughEstate(page: Page, login: string) {
  await signInThroughEstate(page, login)
  await expect(page.getByTestId('oidc-choose')).toBeVisible()
  await page.getByRole('button', { name: 'Create the Account' }).click()
}
