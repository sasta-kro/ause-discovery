import { expect, test, type Locator, type Page } from '@playwright/test'

// Increment 20 browser smoke: left-filter default ordering, per-group sort
// toggles, and guarded expansion focus against the local-test stack.

function group(page: Page, name: string): Locator {
  return page.getByRole('group', { name: `${name} options` })
}

function choiceButtons(page: Page, name: string): Locator {
  return group(page, name).getByRole('button')
}

type RenderedChoice = { label: string; count: number }

async function choices(page: Page, name: string): Promise<RenderedChoice[]> {
  return choiceButtons(page, name).evaluateAll((elements) => elements.map((element) => {
    const spans = element.querySelectorAll('span')
    return { label: spans[0]?.textContent ?? '', count: Number(spans[1]?.textContent ?? 0) }
  }))
}

function expectDescending(actual: RenderedChoice[]): number[] {
  const counts = actual.map((choice) => choice.count)
  expect([...counts].sort((a, b) => b - a)).toEqual(counts)
  return counts
}

test.beforeEach(async ({ page }) => {
  await page.goto('./search')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
})

test('academic years are newest first and expansion focuses the local input', async ({ page }) => {
  await page.getByText('Academic year', { exact: true }).click()
  const years = await choices(page, 'Academic year')
  expect(years.length).toBeGreaterThan(1)
  const numbers = years.map((choice) => Number(choice.label))
  expect([...numbers].sort((a, b) => b - a)).toEqual(numbers)
  await expect(page.getByLabel('Find a year')).toBeFocused()
  expect(await page.getByRole('button', { name: /Academic year order:/ }).count()).toBe(0)
})

test('semester keeps its academic order without a search input or toggle', async ({ page }) => {
  await page.getByText('Semester', { exact: true }).click()
  const labels = (await choices(page, 'Semester')).map((choice) => choice.label)
  expect(labels).toEqual(['First semester', 'Second semester', 'Summer semester'])
  expect(await page.getByRole('searchbox', { name: /semester/i }).count()).toBe(0)
  expect(await page.getByRole('button', { name: /Semester order:/ }).count()).toBe(0)
  // The clicked summary keeps natural focus; nothing else received it.
  expect(await page.evaluate(() => document.activeElement?.tagName)).toBe('SUMMARY')
})

test('people defaults to most common first and the toggle is local per group', async ({ page }) => {
  const before = await page.evaluate(() => performance.getEntriesByType('resource').filter((entry) => entry.name.includes('/api/v1/search')).length)
  await page.getByText('People', { exact: true }).click()
  await expect(page.getByLabel('Find a person')).toBeFocused()
  expectDescending(await choices(page, 'People'))

  await page.getByRole('button', { name: 'People order: Most common first. Change to alphabetical.' }).click()
  const alphabetical = (await choices(page, 'People')).map((choice) => choice.label)
  expect([...alphabetical].sort((a, b) => a.localeCompare(b))).toEqual(alphabetical)
  expect(await page.evaluate(() => performance.getEntriesByType('resource').filter((entry) => entry.name.includes('/api/v1/search')).length)).toBe(before)

  // Closing and reopening preserves the mode while the page stays mounted.
  await page.getByText('People', { exact: true }).click()
  await page.getByText('People', { exact: true }).click()
  const reopened = (await choices(page, 'People')).map((choice) => choice.label)
  expect([...reopened].sort((a, b) => a.localeCompare(b))).toEqual(reopened)

  // Technology stays independent and keeps its own default mode.
  await page.getByText('Technology', { exact: true }).click()
  expectDescending(await choices(page, 'Technology'))
  expect(page.getByRole('button', { name: 'People order: Alphabetical. Change to most common first.' })).toBeTruthy()
})

test('the default-open technology group does not steal initial focus', async ({ page }) => {
  await expect(choiceButtons(page, 'Technology').first()).toBeVisible()
  expect(await page.evaluate(() => document.activeElement === document.body)).toBe(true)
})

test('selection after reordering stays bound to its value and chip', async ({ page }) => {
  await page.getByText('People', { exact: true }).click()
  await page.getByRole('button', { name: 'People order: Most common first. Change to alphabetical.' }).click()
  const first = choiceButtons(page, 'People').first()
  const firstName = (await choices(page, 'People'))[0].label
  await first.click()
  await expect(page).toHaveURL(/person_id=/)
  await expect(page.getByRole('button', { name: `Remove People: ${firstName}` })).toBeVisible()
  await expect(first).toHaveAttribute('aria-pressed', 'true')

  await page.getByRole('button', { name: `Remove People: ${firstName}` }).click()
  await expect(page).not.toHaveURL(/person_id=/)
  await expect(first).toHaveAttribute('aria-pressed', 'false')
})

test('keyboard expansion focuses the local input and zoom stays overflow free', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 740 })
  const summary = page.locator('summary', { hasText: 'Domain' })
  await summary.focus()
  await summary.press('Enter')
  await expect(page.getByLabel('Find domain')).toBeFocused()
  await page.getByRole('button', { name: 'Domain order: Most common first. Change to alphabetical.' }).click()
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  expect(overflow).toBe(false)
})
