import { expect, test, type Page } from '@playwright/test'

// Increment 22 browser acceptance: the complete People facet is reachable in
// the left panel and in local Person-name suggestions without new requests.

const personName = 'Phyo Min Tun'

function suggestionOptions(page: Page) {
  return page.getByRole('listbox', { name: 'Filter suggestions' }).getByRole('option')
}

async function searchRequestCount(page: Page): Promise<number> {
  return page.evaluate(() => performance.getEntriesByType('resource').filter((entry) => entry.name.includes('/api/v1/search')).length)
}

test('the left People filter finds every participating Person', async ({ page }) => {
  await page.goto('./search')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
  await page.locator('summary', { hasText: 'People' }).click()
  await expect(page.getByLabel('Find a person')).toBeFocused()
  // Hundreds of options stay responsive inside the bounded scroll region.
  const visible = await page.getByRole('group', { name: 'People options' }).getByRole('button').evaluateAll((elements) => elements.length)
  expect(visible).toBeGreaterThan(100)

  await page.getByLabel('Find a person').fill('Phyo')
  const matches = page.getByRole('group', { name: 'People options' }).getByRole('button')
  await expect(matches).toHaveCount(1)
  await expect(matches.first()).toContainText(personName)
  await expect(matches.first()).toContainText('11')
})

test('both Person suggestion dimensions appear locally without requests', async ({ page }) => {
  await page.goto('./search')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
  const input = page.getByPlaceholder('Search title, student, advisor, or code')
  await input.click()
  const before = await searchRequestCount(page)

  await input.pressSequentially('Phyo Min')
  const options = suggestionOptions(page)
  await expect(options).toHaveCount(2)
  await expect(options.nth(0)).toContainText(personName)
  await expect(options.nth(0)).toContainText('People')
  await expect(options.nth(1)).toContainText('Advisor')
  expect(await searchRequestCount(page)).toBe(before)

  await input.press('Tab')
  await expect(page).toHaveURL(/person_id=/)
  await expect(page.getByText(`People: ${personName}`)).toBeVisible()
  expect(await searchRequestCount(page)).toBe(before + 1)
})

test('the People panel stays usable at a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 740 })
  await page.goto('./search')
  await page.locator('summary', { hasText: 'People' }).click()
  await page.getByLabel('Find a person').fill('Phyo')
  await expect(page.getByRole('group', { name: 'People options' }).getByRole('button').first()).toContainText(personName)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  expect(overflow).toBe(false)
})
