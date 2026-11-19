// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from './api.ts'
import { ErrorBoundary } from './components/ErrorBoundary.tsx'
import { WakeUpNotice } from './components/WakeUpNotice.tsx'
import { maxRetries, shouldRetry } from './queryClient.ts'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('shouldRetry', () => {
  it('retries network errors and 5xx (a sleeping server) up to maxRetries times', () => {
    expect(shouldRetry(0, new TypeError('Failed to fetch'))).toBe(true)
    expect(shouldRetry(0, new ApiError('/api/x', 502, 'bad_gateway'))).toBe(true)
    expect(shouldRetry(maxRetries - 1, new ApiError('/api/x', 504, 'timeout'))).toBe(true)
    expect(shouldRetry(maxRetries, new ApiError('/api/x', 504, 'timeout'))).toBe(false)
  })

  it('does not retry 4xx answers: asking again gives the same answer', () => {
    expect(shouldRetry(0, new ApiError('/api/x', 404, 'not_found'))).toBe(false)
    expect(shouldRetry(0, new ApiError('/api/x', 401, 'not_signed_in'))).toBe(false)
  })
})

function Crash(): never {
  throw new Error('boom')
}

describe('ErrorBoundary', () => {
  it('shows a message instead of a blank page when a component crashes', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {}) // React logs the caught error
    render(
      <ErrorBoundary>
        <Crash />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('alert').textContent).toContain('Something went wrong')
    expect(screen.getByRole('button', { name: 'Reload' })).toBeTruthy()
  })

  it('forgets the error when the reset key (the page) changes', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const { rerender } = render(
      <ErrorBoundary resetKey="/broken">
        <Crash />
      </ErrorBoundary>,
    )
    expect(screen.queryByRole('alert')).toBeTruthy()
    rerender(
      <ErrorBoundary resetKey="/store">
        <p>Store</p>
      </ErrorBoundary>,
    )
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.getByText('Store')).toBeTruthy()
  })
})

describe('WakeUpNotice', () => {
  function SlowQuery() {
    useQuery({ queryKey: ['slow'], queryFn: () => new Promise(() => {}) }) // never answers
    return null
  }

  it('appears only when data takes longer than 4 seconds', () => {
    vi.useFakeTimers()
    const client = new QueryClient()
    render(
      <QueryClientProvider client={client}>
        <SlowQuery />
        <WakeUpNotice />
      </QueryClientProvider>,
    )
    act(() => vi.advanceTimersByTime(3000))
    expect(screen.queryByText(/Waking up the server/)).toBeNull()
    act(() => vi.advanceTimersByTime(1500))
    expect(screen.getByText(/Waking up the server/)).toBeTruthy()
  })
})
