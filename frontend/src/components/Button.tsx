import type { ComponentProps } from 'react'
import { Link } from 'react-router'
import { Spinner } from './Spinner.tsx'

// The two button looks of the design (design-reference.md §5): yellow with black
// text for the main action, a white outline for secondary actions. Both are 35px
// high with 15px/800 labels and square corners.
type Variant = 'primary' | 'outline'

const base =
  'inline-flex h-7 items-center justify-center gap-2 px-2 text-button font-extrabold whitespace-nowrap ' +
  'transition-[filter,background-color,color,transform] duration-150 active:not-disabled:scale-[0.98] ' +
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-yellow ' +
  'disabled:cursor-not-allowed'

// Disabled styles per variant. Lowering the opacity of the whole button would mix the
// yellow with the purple page behind it into brown, so a disabled primary button
// keeps its colours (and only loses the hover effect; "not-disabled" rather than
// "enabled" because a link (ButtonLink) is never "enabled" in CSS terms); a disabled outline button
// fades its white border and text.
const variants: Record<Variant, string> = {
  primary: 'bg-yellow text-black hover:not-disabled:brightness-95',
  outline:
    'border border-white text-white hover:not-disabled:bg-white hover:not-disabled:text-purple-base ' +
    'disabled:border-white/40 disabled:text-white/40',
}

function classes(variant: Variant, extra?: string) {
  return `${base} ${variants[variant]} ${extra ?? ''}`
}

// ComponentProps<'button'> is "every attribute a <button> accepts" (onClick, type,
// disabled...), so the component does not have to list them one by one.
// loading: the action is running. The button shows a spinner and cannot be pressed
// again (so a slow request is not sent twice); aria-busy tells screen readers.
type ButtonProps = ComponentProps<'button'> & { variant?: Variant; loading?: boolean }

export function Button({ variant = 'primary', className, type = 'button', loading = false, disabled, children, ...rest }: ButtonProps) {
  return (
    <button type={type} className={classes(variant, className)} disabled={disabled || loading} aria-busy={loading || undefined} {...rest}>
      {loading && <Spinner />}
      {children}
    </button>
  )
}

// The same look for navigation. A link and a button are different things to the
// browser (a link has a URL: it can be opened in a new tab, copied, crawled), so
// anything that goes to another page is a real <a>, not a button with a click handler.
type ButtonLinkProps = ComponentProps<typeof Link> & { variant?: Variant }

export function ButtonLink({ variant = 'primary', className, ...rest }: ButtonLinkProps) {
  return <Link className={classes(variant, className)} {...rest} />
}
