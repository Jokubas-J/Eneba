import { useQuery } from '@tanstack/react-query'
import { fetchHomeBlocks } from '../api.ts'
import { Button } from '../components/Button.tsx'
import { Page } from '../components/Page.tsx'
import { BlockSkeleton } from '../home/BlockSection.tsx'
import { blockRenderers } from '../home/blockRenderers.tsx'
import { HomeBlockView } from '../home/HomeBlockView.tsx'

// The homepage is whatever GET /api/home says: an ordered list of blocks, each
// drawn by HomeBlockView, which loads that block's data by itself.
// Contract and failure cases: docs/homepage.md.
export function HomePage() {
  const layout = useQuery({ queryKey: ['home'], queryFn: fetchHomeBlocks })

  if (layout.isError) {
    return (
      <Page>
        <div role="alert" className="bg-purple-dark p-8 text-center">
          <p>The homepage could not be loaded.</p>
          <Button className="mt-3" onClick={() => layout.refetch()}>
            Try again
          </Button>
        </div>
      </Page>
    )
  }
  if (!layout.data) {
    return <BlockSkeleton height={310} />
  }

  // Titled bands alternate between the two purples, starting with the lighter one,
  // counting only blocks that this frontend can draw.
  const framedBlocks = layout.data.filter((block) => blockRenderers[block.type]?.framed)
  return (
    // -my-6 cancels the main area's padding: the banners start right under the
    // header and the last band meets the footer, as on eneba.com. The h1 is for
    // screen readers; like Eneba's, the page has no visible title.
    <div className="-my-6">
      <h1 className="sr-only">Eneba assignment: game keys from many sellers</h1>
      {layout.data.map((block) => (
        <HomeBlockView key={block.id} block={block} dark={framedBlocks.indexOf(block) % 2 === 1} />
      ))}
    </div>
  )
}
