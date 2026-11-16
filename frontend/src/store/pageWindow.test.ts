import { expect, it } from 'vitest'
import { pageWindow } from './pageWindow.ts'

// 0 stands for a gap.
it.each([
  [1, 1, [1]],
  [1, 5, [1, 2, 3, 0, 5]],
  [3, 5, [1, 2, 3, 4, 5]],
  [6, 20, [1, 0, 4, 5, 6, 7, 8, 0, 20]],
  [20, 20, [1, 0, 18, 19, 20]],
  [2, 20, [1, 2, 3, 4, 0, 20]],
])('page %i of %i', (page, pageCount, expected) => {
  expect(pageWindow(page, pageCount)).toEqual(expected)
})
