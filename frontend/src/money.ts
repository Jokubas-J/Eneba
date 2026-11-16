// Money is integer cents everywhere in the data; it becomes text only here.

// Intl.NumberFormat is the browser's built-in formatter. "en-IE" is English as used
// in a euro country, which gives the form Eneba shows: €15.57.
const euroFormat = new Intl.NumberFormat('en-IE', { style: 'currency', currency: 'EUR' })

export function formatPrice(cents: number): string {
  return euroFormat.format(cents / 100)
}

// Parses what a visitor typed into a price box ("10", "9.99", "9,99") into cents.
// Returns null for anything that is not a non-negative amount.
export function eurosToCents(text: string): number | null {
  const trimmed = text.trim().replace(',', '.')
  // Only digits with at most two decimals. parseFloat alone would accept "12abc".
  if (!/^\d+(\.\d{1,2})?$/.test(trimmed)) return null
  // Math.round because 9.99 * 100 is 998.9999999999999 in floating point.
  return Math.round(parseFloat(trimmed) * 100)
}

// The reverse, for showing a bound in the URL and in the price box: 1000 -> "10",
// 999 -> "9.99".
export function centsToEuros(cents: number): string {
  return cents % 100 === 0 ? String(cents / 100) : (cents / 100).toFixed(2)
}
