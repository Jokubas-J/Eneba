import { useQuery } from '@tanstack/react-query'
import { fetchHealth } from '../api.ts'
import { Page } from '../components/Page.tsx'

// Temporary homepage for the deployment skeleton: it proves that the browser can
// reach the Go server through the /api proxy. The real, backend-driven homepage
// blocks replace it later.
export function HomePage() {
  const health = useQuery({ queryKey: ['health'], queryFn: fetchHealth })

  return (
    <Page>
      <h1 className="text-heading font-bold">Eneba assignment</h1>
      <p className="mt-4">
        Backend status:{' '}
        <strong className="font-extrabold">
          {health.isPending && 'checking…'}
          {health.isError && 'unreachable'}
          {health.isSuccess && health.data.status}
        </strong>
      </p>
    </Page>
  )
}
