import { Link, Outlet } from 'react-router'
import { PageSuspense } from './PageFallback.tsx'

const steps = ['Cart', 'Payment', 'Get your product']

// The frame of the checkout pages (design-reference.md §5, screenshot 14): instead
// of the shop header, the logo and a three-step progress bar on a darker strip,
// with no search and no navigation, so nothing distracts from finishing the order.
export function CheckoutLayout({ step }: { step: number }) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="bg-purple-dark">
        <div className="mx-auto flex max-w-page flex-wrap items-center gap-6 px-4 py-3">
          <Link to="/" className="text-[32px] leading-none font-extrabold tracking-tight">
            eneba
            <span className="ml-1 align-top text-tiny font-extrabold text-yellow uppercase">assignment</span>
          </Link>
          <ol className="flex flex-1 items-center gap-2 text-xs font-semibold" aria-label="Checkout progress">
            {steps.map((label, index) => {
              const state = index + 1 === step ? 'current' : index + 1 < step ? 'done' : 'todo'
              return (
                <li
                  key={label}
                  aria-current={state === 'current' ? 'step' : undefined}
                  className="flex flex-1 items-center gap-2 last:flex-none"
                >
                  <span
                    className={`flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-extrabold ${
                      state === 'todo' ? 'bg-text-muted text-purple-dark' : 'bg-teal text-purple-deep'
                    }`}
                  >
                    {index + 1}
                  </span>
                  <span className={state === 'todo' ? 'text-text-muted' : 'text-teal'}>{label}</span>
                  {index < steps.length - 1 && <span className="mx-2 hidden h-px flex-1 bg-text-muted sm:block" />}
                </li>
              )
            })}
          </ol>
        </div>
      </header>
      <main className="flex-1 py-8">
        <PageSuspense>
          <Outlet />
        </PageSuspense>
      </main>
    </div>
  )
}
