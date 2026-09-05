import AxeBuilder from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const acceptanceProjectTitle = process.env.AUSE_ACCEPTANCE_PROJECT_TITLE
const acceptanceProjectID = process.env.AUSE_ACCEPTANCE_PROJECT_ID

test('public discovery, nested routing, and accessibility remain operational', async ({ page }) => {
  await page.goto('./')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  await expect(page.locator('main#main-content')).toHaveCount(1)
  await expect(page.getByRole('link', { name: 'Skip to main content' })).toHaveAttribute('href', '#main-content')

  await page.goto('./search?q=computer%20vision')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
  await expect(page).toHaveTitle('Search projects | AUSE Discovery')

  const accessibility = await new AxeBuilder({ page }).analyze()
  expect(accessibility.violations).toEqual([])
})

test('a configured acceptance Project opens with safe Artifact actions', async ({ page }) => {
  test.skip(!acceptanceProjectTitle || !acceptanceProjectID, 'Acceptance Project fixture is not configured')
  await page.goto(`./projects/${acceptanceProjectID}`)
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(acceptanceProjectTitle!)
  await expect(page.getByRole('link', { name: 'View' })).toHaveAttribute('target', '_blank')
  await expect(page.getByRole('link', { name: 'View' })).toHaveAttribute('rel', /noopener/)
})

test('public shell remains usable at a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 740 })
  await page.goto('./about')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('About AUSE Discovery')
  const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  expect(horizontalOverflow).toBe(false)
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])
})
