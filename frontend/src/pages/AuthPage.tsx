import { type FormEvent, useState } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '../api.ts'
import { safeNextPath, useCurrentUser, useLogin, useRegister } from '../auth/useAuth.ts'
import { Button } from '../components/Button.tsx'

// Log in and Register, laid out like eneba.com's sign-in page (measured with
// Playwright): a large greeting on the left and a 380px form card on the right,
// stacked on a phone. One component for both forms: they differ only in texts and in
// which request they send. (Eneba's social buttons and e-mailed "magic link" are left
// out on purpose; see docs/auth.md.)
export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const [searchParams] = useSearchParams()
  const next = safeNextPath(searchParams.get('next'))
  const navigate = useNavigate()
  const me = useCurrentUser()
  const login = useLogin()
  const register = useRegister()
  const action = mode === 'login' ? login : register

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  // Already signed in: this page has nothing to do.
  if (me.data) return <Navigate to={next} replace />

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    action.mutate({ email, password }, { onSuccess: () => navigate(next, { replace: true }) })
  }

  const isLogin = mode === 'login'
  // The other form keeps ?next=, so switching between them still returns the visitor
  // to where they started.
  const otherLink = `${isLogin ? '/register' : '/login'}${next !== '/' ? `?next=${encodeURIComponent(next)}` : ''}`
  // The backend's message is written for people ("Wrong e-mail address or
  // password."); anything unexpected gets a general one.
  const error =
    action.error instanceof ApiError && action.error.status < 500
      ? action.error.message
      : action.error
        ? 'Something went wrong. Please try again.'
        : null

  return (
    // Measured at 1568px: greeting from x=125, card ending 125px before the right edge.
    <div className="flex w-full flex-col gap-8 px-6 lg:flex-row lg:items-center lg:justify-between lg:px-[125px]">
      {/* The line height is set at both sizes: a bare leading-[46px] would also apply
          on wide screens and make the 80px lines overlap. */}
      <p className="text-[40px] leading-[46px] font-extrabold lg:text-display lg:leading-[88px]">
        Hi!
        <br />
        It’s nice to see you!
      </p>

      {/* The card: 380px wide with 40px padding (300px of content), as measured. */}
      <section aria-labelledby="auth-title" className="w-full lg:w-[380px] lg:shrink-0 lg:bg-purple-base lg:p-8">
        <h1 id="auth-title" className="text-xl font-extrabold">
          {isLogin ? 'Log in' : 'Create account'}
        </h1>
        <p className="mt-1 text-xs font-extrabold text-text-muted">
          {isLogin ? 'New user? ' : 'Already have an account? '}
          <Link to={otherLink} className="text-yellow hover:underline">
            {isLogin ? 'Create account' : 'Log in'}
          </Link>
        </p>

        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4" noValidate>
          <FloatingField
            id="auth-email"
            label="Email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={setEmail}
          />
          <FloatingField
            id="auth-password"
            label="Password"
            type="password"
            // Tells password managers whether to fill in a saved password or offer
            // to generate a new one.
            autoComplete={isLogin ? 'current-password' : 'new-password'}
            value={password}
            onChange={setPassword}
            hint={isLogin ? undefined : 'At least 8 characters.'}
          />

          {/* role="alert": a screen reader reads the error out as soon as it appears. */}
          {error && (
            <p role="alert" className="border border-pink-region p-2 text-sm text-pink-region">
              {error}
            </p>
          )}

          <Button type="submit" disabled={action.isPending} className="w-full">
            {action.isPending ? 'Please wait…' : isLogin ? 'Log in' : 'Create account'}
          </Button>
        </form>
      </section>
    </div>
  )
}

type FieldProps = {
  id: string
  label: string
  type: 'email' | 'password'
  autoComplete: string
  value: string
  onChange: (value: string) => void
  hint?: string
}

// An input whose label sits inside it like a placeholder and moves up when the field
// is focused or filled (Eneba's style, 55px high).
//
// Unlike a real placeholder, it stays a proper <label for=…>: screen readers announce
// it, clicking it focuses the input, and it does not disappear once the visitor types.
// The trick: the input has a placeholder of one space, so the CSS state
// :placeholder-shown means "empty"; Tailwind's peer-* classes let the label (the
// input's next sibling) react to the input's state without any JavaScript.
function FloatingField({ id, label, type, autoComplete, value, onChange, hint }: FieldProps) {
  const hintId = hint ? `${id}-hint` : undefined
  return (
    <div>
      <div className="relative">
        <input
          id={id}
          type={type}
          autoComplete={autoComplete}
          required
          placeholder=" "
          value={value}
          onChange={(event) => onChange(event.target.value)}
          aria-describedby={hintId}
          className="peer h-[55px] w-full bg-overlay-input px-4 pt-4 text-base focus:outline-2 focus:outline-yellow"
        />
        <label
          htmlFor={id}
          className="pointer-events-none absolute top-2 left-4 text-xs text-text-muted transition-all peer-placeholder-shown:top-1/2 peer-placeholder-shown:-translate-y-1/2 peer-placeholder-shown:text-base peer-focus:top-2 peer-focus:translate-y-0 peer-focus:text-xs"
        >
          {label}
        </label>
      </div>
      {hint && (
        <p id={hintId} className="mt-1 text-xs text-text-muted">
          {hint}
        </p>
      )}
    </div>
  )
}
