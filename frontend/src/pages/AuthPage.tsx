import { type FormEvent, useState } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '../api.ts'
import { safeNextPath, useCurrentUser, useLogin, useRegister } from '../auth/useAuth.ts'
import { Button } from '../components/Button.tsx'
import { Page } from '../components/Page.tsx'

// One component for both forms: they differ only in texts and in which request
// they send.
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
    <Page>
      <div className="mx-auto max-w-[420px] bg-purple-light p-6 shadow-panel">
        <h1 className="text-heading font-bold">{isLogin ? 'Log in' : 'Create an account'}</h1>

        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4" noValidate>
          <label className="flex flex-col gap-1 text-sm font-semibold">
            E-mail
            <input
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="h-8 bg-overlay-input px-2 font-normal inset-shadow-input focus:outline-2 focus:outline-yellow"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm font-semibold">
            Password
            <input
              type="password"
              // Tells password managers whether to fill in a saved password or offer
              // to generate a new one.
              autoComplete={isLogin ? 'current-password' : 'new-password'}
              required
              minLength={isLogin ? undefined : 8}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              aria-describedby={isLogin ? undefined : 'password-hint'}
              className="h-8 bg-overlay-input px-2 font-normal inset-shadow-input focus:outline-2 focus:outline-yellow"
            />
            {!isLogin && (
              <span id="password-hint" className="text-xs font-normal text-text-muted">
                At least 8 characters.
              </span>
            )}
          </label>

          {/* role="alert": a screen reader reads the error out as soon as it appears. */}
          {error && (
            <p role="alert" className="border border-pink-region p-2 text-sm text-pink-region">
              {error}
            </p>
          )}

          <Button type="submit" disabled={action.isPending}>
            {action.isPending ? 'Please wait…' : isLogin ? 'Log in' : 'Register'}
          </Button>
        </form>

        <p className="mt-6 text-center text-sm text-text-muted">
          {isLogin ? 'No account yet? ' : 'Already have an account? '}
          <Link to={otherLink} className="font-semibold text-yellow hover:underline">
            {isLogin ? 'Register' : 'Log in'}
          </Link>
        </p>
      </div>
    </Page>
  )
}
