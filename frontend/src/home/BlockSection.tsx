import { useId, type ReactNode } from 'react'
import { ButtonLink } from '../components/Button.tsx'
import { Page } from '../components/Page.tsx'

// The frame of a homepage block with a heading: a full-width band (Eneba alternates
// two purples, design-reference.md §5 "Homepage blocks"), the h2, the content and an
// optional centred "Show all" button.
export function BlockSection({
  title,
  showAllUrl,
  dark,
  children,
}: {
  title: string | null
  showAllUrl: string | null
  dark: boolean
  children: ReactNode
}) {
  const headingId = useId()
  return (
    <section aria-labelledby={title ? headingId : undefined} className={`py-10 ${dark ? 'bg-purple-dark' : ''}`}>
      <Page>
        {title && (
          <h2 id={headingId} className="mb-6 text-heading font-bold">
            {title}
          </h2>
        )}
        {children}
        {showAllUrl && (
          <div className="mt-6 flex justify-center">
            <ButtonLink to={showAllUrl} variant="outline" className="w-[190px]">
              Show all
            </ButtonLink>
          </div>
        )}
      </Page>
    </section>
  )
}

// What a block shows while its data loads or after it failed. Each block has its own,
// so a slow or broken block never holds up or breaks the others.
export function BlockSkeleton({ height }: { height: number }) {
  return <div className="animate-pulse bg-overlay-item" style={{ height }} aria-hidden="true" />
}

export function BlockError({ onRetry }: { onRetry: () => void }) {
  return (
    <div role="alert" className="flex flex-col items-center gap-3 bg-overlay-item py-8 text-center">
      <p>This section could not be loaded.</p>
      <button type="button" onClick={onRetry} className="font-semibold text-yellow hover:underline">
        Try again
      </button>
    </div>
  )
}
