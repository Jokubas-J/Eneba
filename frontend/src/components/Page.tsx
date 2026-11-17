import type { ReactNode } from 'react'

// The centred content column of a page. Most pages use Eneba's standard width
// (1200px, the same as the header); the product page uses the wide one (1600px).
export function Page({ wide = false, children }: { wide?: boolean; children: ReactNode }) {
  return <div className={`mx-auto w-full px-4 ${wide ? 'max-w-wide' : 'max-w-page'}`}>{children}</div>
}
