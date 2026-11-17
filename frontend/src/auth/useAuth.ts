import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchMe, login, logout, register, type User } from '../api.ts'

// The signed-in user is server state like any other: one query, cached under
// ['me'], shared by every component that asks. Sign-in, registration and sign-out
// write the new answer straight into that cache, so the header updates at once
// without another request.
export function useCurrentUser() {
  return useQuery({
    queryKey: ['me'],
    // No abort signal here (unlike the search dropdown): in development, StrictMode
    // mounts the page twice, the first unmount cancelled the request, and the second
    // mount stayed attached to the cancelled request, leaving the page on its
    // skeleton. Cancelling brings nothing for a whole-page load anyway.
    queryFn: () => fetchMe(),
    staleTime: 5 * 60_000,
  })
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) => login(email, password),
    onSuccess: (user: User) => queryClient.setQueryData(['me'], user),
  })
}

export function useRegister() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) => register(email, password),
    onSuccess: (user: User) => queryClient.setQueryData(['me'], user),
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: logout,
    onSuccess: () => queryClient.setQueryData(['me'], null),
  })
}

// safeNextPath decides where to go after signing in. The target comes from the URL
// (?next=/cart), which anyone can write, so only a path on this site is accepted:
// "https://evil.example" or "//evil.example" (a protocol-relative URL, which the
// browser treats as another site) would turn the login page into a redirector to
// a phishing page.
export function safeNextPath(next: string | null): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return '/'
  return next
}
