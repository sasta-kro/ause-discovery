import { expect, test, type Page } from '@playwright/test'

// Increment 21 browser verification: internal pathname navigation begins at
// document position zero with the complete header visible, main keeps route
// focus, and same-path Search state changes preserve scroll.

async function state(page: Page): Promise<{ scrollY: number; activeId: string | null; headerVisible: boolean }> {
  return page.evaluate(() => {
    const header = document.querySelector('header')
    const headerBottom = header ? header.getBoundingClientRect().bottom : -1
    return {
      scrollY: window.scrollY,
      activeId: document.activeElement?.id ?? null,
      headerVisible: headerBottom >= 0 && headerBottom <= window.innerHeight,
    }
  })
}

test.beforeEach(async ({ page }) => {
  await page.goto('./search')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
})

test('brand navigation from a scrolled Search page returns Home to the top', async ({ page }) => {
  await page.evaluate(() => window.scrollTo(0, 1200))
  expect((await state(page)).scrollY).toBeGreaterThan(0)
  await page.getByRole('link', { name: 'AUSE Discovery' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  const result = await state(page)
  expect(result.scrollY).toBe(0)
  expect(result.headerVisible).toBe(true)
  expect(result.activeId).toBe('main-content')
})

test('browse and landing search navigation open Search at the top', async ({ page }) => {
  await page.getByRole('link', { name: 'AUSE Discovery' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  await page.evaluate(() => window.scrollTo(0, 900))
  await page.getByRole('link', { name: 'Browse the archive' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
  let result = await state(page)
  expect(result.scrollY).toBe(0)
  expect(result.headerVisible).toBe(true)
  expect(result.activeId).toBe('main-content')

  await page.getByRole('link', { name: 'AUSE Discovery' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  await page.evaluate(() => window.scrollTo(0, 600))
  await page.getByLabel('Search the project archive').fill('archive')
  await page.getByRole('button', { name: 'Search projects' }).click()
  await expect(page).toHaveURL(/q=archive/)
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
  result = await state(page)
  expect(result.headerVisible).toBe(true)
  expect(result.activeId).toBe('main-content')
})

test('brand activation while already on Home returns the page to the top', async ({ page }) => {
  await page.getByRole('link', { name: 'AUSE Discovery' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  await page.evaluate(() => window.scrollTo(0, 800))
  await page.getByRole('link', { name: 'AUSE Discovery' }).click()
  await page.waitForTimeout(200)
  const result = await state(page)
  expect(result.scrollY).toBe(0)
  expect(result.headerVisible).toBe(true)
})

test('same-path Search filter, sort, and paging changes preserve the scroll position', async ({ page }) => {
  await page.locator('summary', { hasText: 'Category' }).click()
  await expect(page.getByRole('group', { name: 'Category options' }).getByRole('button').first()).toBeVisible()
  // Scroll below the header; the still-loading document may clamp the exact
  // request, so the preserved anchor is whatever position actually settled.
  await page.evaluate(() => window.scrollTo(0, 700))
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(300)
  const anchor = await page.evaluate(() => window.scrollY)

  // Native scroll anchoring may compensate a few pixels when the result list
  // reflows; the contract forbids a jump back to the top, not reflow itself.
  const preserved = async () => {
    const position = await page.evaluate(() => window.scrollY)
    expect(Math.abs(position - anchor)).toBeLessThan(80)
    expect(position).toBeGreaterThan(300)
  }

  await page.getByRole('group', { name: 'Category options' }).getByRole('button').first().click()
  await page.waitForTimeout(700)
  await preserved()

  await page.getByLabel('Sort results').selectOption('title')
  await page.waitForTimeout(700)
  await preserved()

  // Cursor pagination replaces the entire result list, so the browser's own
  // scroll anchoring may relocate; the contract forbids a reset to the top.
  const next = page.getByRole('button', { name: 'Next page' })
  if (await next.isEnabled()) {
    await next.click()
    await page.waitForTimeout(700)
    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(300)
  }
})

test('history back and forward keep focus on main without forcing the top', async ({ page }) => {
  await page.getByRole('link', { name: 'AUSE Discovery' }).click()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  await page.goBack()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
  await page.waitForTimeout(300)
  const result = await state(page)
  expect(result.activeId).toBe('main-content')
  expect(result.scrollY).toBe(0)
  await page.goForward()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  await page.waitForTimeout(300)
  expect((await state(page)).activeId).toBe('main-content')
})

test('keyboard activation follows the same policy at a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 740 })
  await page.evaluate(() => window.scrollTo(0, 1000))
  const brand = page.getByRole('link', { name: 'AUSE Discovery' })
  await brand.focus()
  await brand.press('Enter')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Discover the work that shaped a generation.')
  const home = await state(page)
  expect(home.scrollY).toBe(0)
  expect(home.headerVisible).toBe(true)

  const browse = page.getByRole('link', { name: 'Browse the archive' })
  await page.evaluate(() => window.scrollTo(0, 600))
  await browse.focus()
  await browse.press('Enter')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Search projects')
  const search = await state(page)
  expect(search.scrollY).toBe(0)
  expect(search.headerVisible).toBe(true)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  expect(overflow).toBe(false)
})
