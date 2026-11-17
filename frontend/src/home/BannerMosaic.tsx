import { Link } from 'react-router'
import type { Banner } from '../api.ts'
import { RemoteImage } from '../components/RemoteImage.tsx'

// Eneba's banner mosaic (screenshot 01): edge to edge, ~310px high, one large tile
// followed by narrow ones. Which banners, their order and links come from the
// banners table; this component only lays them out. On phones the large tile is
// shown alone, as the narrow ones would be slivers.
export function BannerMosaic({ items }: { items: Banner[] }) {
  const [first, ...rest] = items
  if (!first) return null
  return (
    <div className="flex h-[180px] gap-1 md:h-[310px]">
      <BannerTile banner={first} large className="min-w-0 flex-[7]" />
      {rest.map((banner) => (
        <BannerTile key={banner.id} banner={banner} className="hidden min-w-0 flex-1 md:block" />
      ))}
    </div>
  )
}

function BannerTile({ banner, large = false, className }: { banner: Banner; large?: boolean; className: string }) {
  const content = (
    <>
      <RemoteImage
        src={banner.imageUrl}
        // The large tile is the first thing on the page: load it at once.
        loading={large ? 'eager' : 'lazy'}
        className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
      />
      {/* Steam's hero art has no text on it, so the title goes on a dark gradient
          at the bottom, where it stays readable on any picture. */}
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-4 pt-10">
        <p className={`font-extrabold ${large ? 'text-heading' : 'text-sm'}`}>{banner.title}</p>
        {large && banner.subtitle && <p className="mt-1 text-sm text-text-nav">{banner.subtitle}</p>}
      </div>
    </>
  )
  const classes = `group relative block overflow-hidden bg-purple-dark ${className}`
  // Internal links stay inside the single-page app; anything else is a normal link.
  return banner.linkUrl.startsWith('/') ? (
    <Link to={banner.linkUrl} className={classes} aria-label={banner.title}>
      {content}
    </Link>
  ) : (
    <a href={banner.linkUrl} className={classes} aria-label={banner.title}>
      {content}
    </a>
  )
}
