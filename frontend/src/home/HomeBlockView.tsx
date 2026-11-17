import { useQuery } from '@tanstack/react-query'
import { fetchBlockItems, type HomeBlock } from '../api.ts'
import { blockRenderers } from './blockRenderers.tsx'
import { BlockError, BlockSection } from './BlockSection.tsx'

// One block: fetches its own data (own loading and error state) and draws it with
// the renderer of its type. Returns nothing for a type this frontend does not know,
// or when the block has no items, so neither leaves an empty gap or breaks the page.
export function HomeBlockView({ block, dark }: { block: HomeBlock; dark: boolean }) {
  const renderer = blockRenderers[block.type]
  const items = useQuery({
    queryKey: ['home-block', block.source],
    queryFn: () => fetchBlockItems<unknown>(block.source),
    // An unknown type needs no data.
    enabled: renderer !== undefined,
  })
  if (!renderer) return null
  if (items.data && items.data.length === 0) return null

  const body = items.isError ? (
    <BlockError onRetry={() => items.refetch()} />
  ) : !items.data ? (
    renderer.skeleton()
  ) : (
    // The real content fades in where the skeleton was.
    <div className="animate-fade-in">{renderer.render(items.data)}</div>
  )

  if (!renderer.framed) return <div>{body}</div>
  return (
    <BlockSection title={block.title} showAllUrl={items.data ? block.showAllUrl : null} dark={dark}>
      {body}
    </BlockSection>
  )
}
