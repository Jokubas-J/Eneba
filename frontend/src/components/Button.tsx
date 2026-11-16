import type { ComponentProps } from 'react'
import { Link } from 'react-router'

// The two button looks of the design (design-reference.md §5): yellow with black
// text for the main action, a white outline for secondary actions. Both are 35px
// high with 15px/800 labels and square corners.
type Variant = 'primary' | 'outline'

const base =
  'inline-flex h-7 items-center justify-center px-2 text-button font-extrabold whitespace-nowrap ' +
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-yellow ' +
  'disabled:cursor-not-allowed disabled:opacity-50'

const variants: Record<Variant, string> = {
  primary: 'bg-yellow text-black hover:brightness-95',
  outline: 'border border-white text-white hover:bg-white hover:text-purple-base',
}

function classes(variant: Variant, extra?: string) {
  return `${base} ${variants[variant]} ${extra ?? ''}`
}

// ComponentProps<'button'> is "every attribute a <button> accepts" (onClick, type,
// disabled...), so the component does not have to list them one by one.
type ButtonProps = ComponentProps<'button'> & { variant?: Variant }

export function Button({ variant = 'primary', className, type = 'button', ...rest }: ButtonProps) {
  return <button type={type} className={classes(variant, className)} {...rest} />
}

// The same look for navigation. A link and a button are different things to the
// browser (a link has a URL: it can be opened in a new tab, copied, crawled), so
// anything that goes to another page is a real <a>, not a button with a click handler.
type ButtonLinkProps = ComponentProps<typeof Link> & { variant?: Variant }

export function ButtonLink({ variant = 'primary', className, ...rest }: ButtonLinkProps) {
  return <Link className={classes(variant, className)} {...rest} />
}
