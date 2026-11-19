import { Component, type ErrorInfo, type ReactNode } from 'react'

// Catches an error thrown while React renders the components inside it, and shows a
// short message with a way out instead of a blank page.
//
// It has to be a class: getDerivedStateFromError and componentDidCatch exist only for
// class components (React has no hook for this). Think of it as a try/catch around
// rendering. It does not catch errors in event handlers or in requests; those are
// handled where they happen (TanStack Query error states, mutation errors).
//
// `resetKey`: when it changes (Layout passes the current path), a caught error is
// forgotten, so following a link away from a broken page works again.
type Props = { children: ReactNode; resetKey?: string }
type State = { failed: boolean; key?: string }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { failed: false, key: this.props.resetKey }

  static getDerivedStateFromError(): Partial<State> {
    return { failed: true }
  }

  static getDerivedStateFromProps(props: Props, state: State): Partial<State> | null {
    return props.resetKey !== state.key ? { failed: false, key: props.resetKey } : null
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    // The details go to the browser console for whoever debugs it.
    console.error('Page crashed:', error, info.componentStack)
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <div role="alert" className="mx-auto my-10 max-w-[480px] bg-purple-dark p-8 text-center">
        <p className="text-xl font-extrabold">Something went wrong on this page.</p>
        <p className="mt-2 text-sm text-text-muted">Reloading usually helps. Your cart is kept.</p>
        <div className="mt-6 flex justify-center gap-3">
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="h-7 bg-yellow px-4 text-button font-extrabold text-black"
          >
            Reload
          </button>
          <a href="/" className="flex h-7 items-center border border-white px-4 text-button font-extrabold">
            Homepage
          </a>
        </div>
      </div>
    )
  }
}
