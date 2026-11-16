import { useQuery } from '@tanstack/react-query'
import { fetchHealth } from './api.ts'

// Temporary page for the deployment skeleton: it proves that the browser can reach
// the Go server through the /api proxy. It is replaced by the real shell and routes.
function App() {
  const health = useQuery({ queryKey: ['health'], queryFn: fetchHealth })

  return (
    <main className="mx-auto max-w-xl p-8">
      <h1 className="text-2xl font-bold">Eneba assignment</h1>
      <p className="mt-4">
        Backend status:{' '}
        <strong>
          {health.isPending && 'checking…'}
          {health.isError && 'unreachable'}
          {health.isSuccess && health.data.status}
        </strong>
      </p>
    </main>
  )
}

export default App
