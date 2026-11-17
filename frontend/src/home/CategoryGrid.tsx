import { useState } from 'react'
import { Link } from 'react-router'
import type { Category } from '../api.ts'
import { Button } from '../components/Button.tsx'
import { ContentIcon } from '../components/ContentIcon.tsx'

// Rows of 7 tiles, as on eneba.com (screenshot 06); "Load more" shows the rest.
const firstRow = 7

// Square genre tiles (screenshot 06): live count on top, icon, name. Each tile opens
// the store filtered by that genre. The icon key comes with the genre from the API
// (taxonomy_icons); a genre without one gets the default icon.
export function CategoryGrid({ items }: { items: Category[] }) {
  const [showAll, setShowAll] = useState(false)
  const visible = showAll ? items : items.slice(0, firstRow)
  return (
    <>
      <ul className="grid grid-cols-2 gap-4 sm:grid-cols-4 lg:grid-cols-7">
        {visible.map((category) => (
          <li key={category.slug}>
            <Link
              to={`/store?genre=${encodeURIComponent(category.slug)}`}
              className="flex h-[135px] flex-col items-center justify-between bg-purple-light p-4 shadow-tile hover:brightness-110"
            >
              <span className="text-sm font-extrabold">{category.count}</span>
              <ContentIcon name={category.icon} className="size-9" />
              <span className="text-sm font-extrabold">{category.name}</span>
            </Link>
          </li>
        ))}
      </ul>
      {!showAll && items.length > firstRow && (
        <div className="mt-6 flex justify-center">
          <Button variant="outline" className="w-[190px]" onClick={() => setShowAll(true)}>
            Load more
          </Button>
        </div>
      )}
    </>
  )
}
