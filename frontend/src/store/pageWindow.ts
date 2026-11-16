// pageWindow lists the page numbers to show: always the first and last page and up
// to two pages on each side of the current one; 0 stands for a gap ("…").
//   page 6 of 20  ->  1 … 4 5 [6] 7 8 … 20
export function pageWindow(page: number, pageCount: number): number[] {
  const pages: number[] = []
  for (let p = 1; p <= pageCount; p++) {
    const near = Math.abs(p - page) <= 2
    if (p === 1 || p === pageCount || near) {
      pages.push(p)
    } else if (pages[pages.length - 1] !== 0) {
      pages.push(0)
    }
  }
  return pages
}
