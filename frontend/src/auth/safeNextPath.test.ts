import { expect, it } from 'vitest'
import { safeNextPath } from './useAuth.ts'

it.each([
  ['/cart', '/cart'],
  ['/store?text=gta', '/store?text=gta'],
  [null, '/'],
  ['', '/'],
  ['https://evil.example', '/'],
  ['//evil.example/path', '/'],
  ['/\\evil.example', '/'],
  ['javascript:alert(1)', '/'],
  ['cart', '/'],
])('next=%j -> %s', (next, expected) => {
  expect(safeNextPath(next)).toBe(expected)
})
