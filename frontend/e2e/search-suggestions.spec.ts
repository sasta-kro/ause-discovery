import { expect, test, type Locator, type Page } from '@playwright/test'

// Browser keyboard verification for Increment 18 Search Filter Suggestions:
// Tab acceptance, free-text Enter, and the two-stage Escape contract against
// the local-test stack with imported Project metadata.

function inputLocator(page: Page): Locator {
  return page.getByPlaceholder('Search title, student, advisor, or code')
}

// The page also exposes sort-select and left-panel filter listboxes, so
// suggestion options are always scoped to the Filter suggestions popup.
function suggestionOptions(page: Page): Locator {
  return page.getByRole('listbox', { name: 'Filter suggestions' }).getByRole('option')
}

async function searchRequestCount(page: Page): Promise<number> {
  return page.evaluate(() => performance.getEntriesByType('resource').filter((entry) => entry.name.includes('/api/v1/search')).length)
}

test.beforeEach(async ({ page }) => {
  await page.goto('./search')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
})

test('typing a year offers one suggestion and Tab converts exactly it', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  const before = await searchRequestCount(page)

  await input.pressSequentially('2025')
  const options = suggestionOptions(page)
  await expect(options).toHaveCount(1)
  await expect(options.first()).toContainText('2025')
  await expect(options.first()).toContainText('Academic year')
  await expect(page.locator('kbd')).toHaveText('Tab')

  await input.press('Tab')
  await expect(input).toBeFocused()
  await expect(input).toHaveValue('')
  await expect(page).toHaveURL(/academic_year=2025/)
  await expect(page.getByText('Academic year: 2025')).toBeVisible()

  const after = await searchRequestCount(page)
  expect(after - before).toBe(1)
})

test('mixed-case classification prefix converts case-insensitively', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('COMP')
  await expect(suggestionOptions(page).first()).toContainText('Computer Science')
  await input.press('Tab')
  await expect(page).toHaveURL(/program_key=/)
  await expect(page.getByText('Program: Computer Science')).toBeVisible()
})

test('best gam keeps the unmatched free text and searches with it', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('best gam')
  await expect(suggestionOptions(page).first()).toContainText('Game')
  await input.press('Tab')
  await expect(input).toHaveValue('best')
  await expect(page).toHaveURL(/category_key=game/)
  await expect(page).toHaveURL(new RegExp('q=best'))
  await expect(page.getByText('Category: Game')).toBeVisible()
})

test('an unrelated trailing term closes suggestions and removing it restores them', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('gam zzzz')
  await expect(page.getByRole('listbox', { name: 'Filter suggestions' })).toHaveCount(0)
  for (let index = 0; index < 5; index += 1) await input.press('Backspace')
  await expect(input).toHaveValue('gam')
  await expect(suggestionOptions(page).first()).toContainText('Game')
})

test('collision navigation converts the highlighted dimension', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('other')
  const options = suggestionOptions(page)
  await expect(options).toHaveCount(2)
  await expect(options.nth(0)).toContainText('Platform')
  await expect(options.nth(1)).toContainText('Domain')
  await input.press('ArrowDown')
  await expect(input).toHaveAttribute('aria-activedescendant', /.+option-domain_key_other/)
  await input.press('Tab')
  await expect(page).toHaveURL(/domain_key=other/)
  await expect(page.getByText('Domain: Other')).toBeVisible()
})

test('Enter submits the draft as free text without accepting a suggestion', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('pyt')
  await expect(suggestionOptions(page).first()).toContainText('Python')
  await input.press('Enter')
  await expect(page).toHaveURL(/q=pyt/)
  await expect(page).not.toHaveURL(/technology_key=/)
  await expect(page.getByRole('listbox', { name: 'Filter suggestions' })).toHaveCount(0)
})

test('Escape closes without blur, text change reopens, second Escape blurs', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('2019')
  await expect(suggestionOptions(page).first()).toContainText('2019')

  await input.press('Escape')
  await expect(input).toBeFocused()
  await expect(page.getByRole('listbox', { name: 'Filter suggestions' })).toHaveCount(0)

  await input.pressSequentially('9')
  await expect(page.getByRole('listbox', { name: 'Filter suggestions' })).toHaveCount(0)
  for (let index = 0; index < 2; index += 1) await input.press('Backspace')
  await input.pressSequentially('8')
  await expect(suggestionOptions(page).first()).toContainText('2018')

  await input.press('Escape')
  await input.press('Escape')
  await expect(input).not.toBeFocused()
  await input.press('Tab')
  await expect(input).not.toBeFocused()
})

test('pointer selection applies exactly one filter without a blur race', async ({ page }) => {
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('mobile')
  await suggestionOptions(page).first().click()
  await expect(page).toHaveURL(/platform_key=mobile/)
  await expect(page.getByText('Platform: Mobile')).toBeVisible()
  await expect(page.getByRole('listbox', { name: 'Filter suggestions' })).toHaveCount(0)
})

test('suggestions and controls remain usable at 200 percent zoom and narrow width', async ({ page }) => {
  // style.zoom does not re-evaluate media queries the way real browser zoom
  // does, so the responsive reflow claim is proven on a genuinely narrow
  // viewport and zoom is used only to prove the surface stays visible.
  await page.setViewportSize({ width: 360, height: 740 })
  const input = inputLocator(page)
  await input.click()
  await input.pressSequentially('2020')
  await expect(suggestionOptions(page).first()).toContainText('2020')
  await expect(page.locator('kbd')).toBeVisible()
  await input.press('Tab')
  await expect(page).toHaveURL(/academic_year=2020/)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  expect(overflow).toBe(false)

  await page.evaluate(() => { document.documentElement.style.zoom = '2' })
  // Year choices mirror the filtered facet response, exactly like the left
  // panel, so a different year is not suggested while 2020 is applied. A
  // catalog dimension keeps the surface live at this zoom.
  const zoomInput = inputLocator(page)
  await zoomInput.click()
  await zoomInput.pressSequentially('mob')
  await expect(suggestionOptions(page).first()).toContainText('Mobile')
  await expect(page.locator('kbd')).toBeVisible()

  // Removing the year chip restores the unfiltered year facets, which makes
  // other years eligible again.
  await zoomInput.press('Escape')
  for (let index = 0; index < 3; index += 1) await zoomInput.press('Backspace')
  await page.getByRole('button', { name: 'Remove Academic year: 2020' }).click()
  await zoomInput.click()
  await zoomInput.pressSequentially('2019')
  await expect(suggestionOptions(page).first()).toContainText('2019')
})
