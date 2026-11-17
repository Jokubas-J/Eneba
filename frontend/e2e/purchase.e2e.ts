import { expect, test } from '@playwright/test'

// The happy path from PLAN.md §8, as a visitor would do it: search with a typo, open
// the product, add it to the cart, sign up on the way to checkout, pay (simulated
// provider), see the key, find the order in the history.
test('search with a typo, buy a key, see it in the order history', async ({ page }) => {
  // 1. Search with a typo: "witchr 3" must still find The Witcher 3.
  await page.goto('/')
  const search = page.getByRole('combobox', { name: /search/i })
  // Typed key by key, as a person does (the dropdown reacts to each keystroke).
  await search.pressSequentially('witchr 3', { delay: 50 })
  await expect(page.getByRole('listbox')).toBeVisible()
  await search.press('Enter')
  await expect(page).toHaveURL(/\/store\?text=witchr/)
  const result = page.getByRole('link', { name: /The Witcher 3/ }).first()
  await expect(result).toBeVisible()

  // 2. Open the product and add the best offer to the cart.
  await result.click()
  await expect(page.getByRole('heading', { level: 1, name: /The Witcher 3/ })).toBeVisible()
  const productName = (await page.getByRole('heading', { level: 1 }).textContent())?.trim() ?? ''
  await page.getByRole('button', { name: /add to cart/i }).first().click()
  const added = page.getByRole('dialog', { name: 'Added to cart' })
  await expect(added).toBeVisible()
  await added.getByRole('button', { name: 'View cart' }).click()

  // 3. The cart asks a visitor to log in; register instead, and come back to the cart.
  await expect(page).toHaveURL(/\/cart$/)
  await expect(page.getByText(productName).first()).toBeVisible()
  await page.getByRole('link', { name: 'Log in to check out' }).click()
  await expect(page).toHaveURL(/\/login\?next=%2Fcart/)
  await page.getByRole('main').getByRole('link', { name: 'Create account' }).click()
  await expect(page).toHaveURL(/\/register\?next=%2Fcart/)
  const email = `e2e-${Date.now()}@example.com`
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill('correct horse battery staple')
  await page.locator('main form button[type=submit]').click()
  await expect(page).toHaveURL(/\/cart$/)

  // 4. Checkout reserves the key and opens the (simulated) payment page.
  await page.getByRole('button', { name: 'Proceed to checkout' }).click()
  await expect(page).toHaveURL(/\/checkout\/simulated\/\d+$/)
  const orderId = page.url().split('/').pop()
  await page.getByRole('button', { name: /^Pay / }).click()

  // 5. The order page shows the delivered key.
  await expect(page).toHaveURL(new RegExp(`/orders/${orderId}$`))
  await expect(page.getByText('Payment received. Your keys are below.')).toBeVisible()
  const key = page.locator('ul[aria-label^="Keys for"] code').first()
  await expect(key).toHaveText(/^[A-Z0-9]+(-[A-Z0-9]+)+$/)

  // 6. The cart is empty again, and the order is in the history as paid.
  await page.goto('/orders')
  const row = page.getByRole('link', { name: new RegExp(`Order #${orderId}\\b`) })
  await expect(row).toBeVisible()
  await expect(row).toContainText('Paid')
  // The header's cart link has no item count in its name any more: the cart is empty.
  await expect(page.getByRole('banner').getByRole('link', { name: 'Cart', exact: true })).toBeVisible()
})
