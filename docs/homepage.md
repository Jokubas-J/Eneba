# Homepage — backend-driven blocks

The brief asks for a homepage whose content comes from the backend (V1/V2 in
`docs/requirements.md`). Here both the **layout** and **each block's data** come from the API;
the frontend knows only how to draw a few block *types*.

## How it works

1. `GET /api/home` returns the active rows of `home_blocks`, in `sort_order`:

   ```json
   {"items": [
     {"id": 1, "type": "banner_mosaic", "title": null, "source": "/api/home/banners", "showAllUrl": null},
     {"id": 2, "type": "product_carousel", "title": "Best sellers", "source": "/api/home/best-sellers", "showAllUrl": "/store?sort=popular"},
     …
   ]}
   ```

2. The frontend has one map: block type → React component. For each block it fetches
   `source` **separately**, with its own loading skeleton and its own error state. A type it
   does not know is skipped, so a new type can be added to the table before the frontend
   supports it.
3. Each `source` is one endpoint with one query (`backend/internal/home`):

| Block | Endpoint | Data | Rule |
|---|---|---|---|
| Banner mosaic | `/api/home/banners` | `banners` table (curated) | active rows by `sort_order`; the first is the large tile |
| Best sellers | `/api/home/best-sellers` | `order_items` of paid orders + `products.popularity` | keys sold in the last 30 days, then Steam recommendation count; in stock only |
| Biggest discounts | `/api/home/deals` | `product_listing` (cheapest in-stock offer) | largest % below the reference price |
| New releases | `/api/home/new-releases` | `products.release_date` | newest first; future dates (pre-orders) left out |
| Categories | `/api/home/categories` | `genres` + live counts + `taxonomy_icons` | genres with something in stock, largest first, each with its icon key |

All carousels read the same `product_listing` view as the store, so a card's "From €X" is the
same on every page. Every endpoint answers `{"items": [...]}` (never `null`) with
`Cache-Control: public, max-age=60`.

## Decisions

- **Best sellers uses real sales only.** The seed creates no order history; inventing one would
  be fake data. Products without sales in the window follow by popularity, so the row is full
  from the start and a real purchase visibly moves a game up. (Author's decision, 2026-11-17.)
- **Banner art is Steam's "library hero" image** (3840×1240), hot-linked like the covers. Which
  games have one was checked once when migration 00011 was written; the app never calls Steam.
  Banners carry no prices, because a price written into a banner would go stale.
  (Author's decision, 2026-11-17.)
- **One package, one server, separate endpoints.** "Independent blocks" is about failure and
  loading: one slow or broken query costs only its own block. Splitting the five queries into
  separately deployed services would add network hops and deployments and change nothing the
  visitor sees.
- **Content lives in migrations, not the seed.** `seed load -reset` empties the catalog and
  orders but not `home_blocks` or `banners`, so a re-seed keeps the homepage. Changing the
  homepage is an `UPDATE` on those tables, not a deploy.

Icons: a genre's icon is a key in `taxonomy_icons` (by slug, so it survives a re-seed); the
frontend draws it with `ContentIcon.tsx` and falls back to a default icon for an unknown key.

## Failure cases

| Case | Handling | Test |
|---|---|---|
| One block's query fails | that endpoint returns 500 (no internal details); the other blocks still load; the frontend shows a retry in that block only | `TestHomeEndpoints`, frontend `HomePage.test.tsx` |
| A block type the frontend does not know | skipped, the rest of the page renders | frontend `HomePage.test.tsx` |
| A block has no data | `{"items": []}`; the frontend hides the block | `TestHomeEndpoints` |
| A product sells out | it leaves the carousels within a minute (cache) | `TestBestSellers`, `TestDeals` |
| Unpaid, expired or old orders | do not count as sales | `TestBestSellers` |
| A banner image fails to load | placeholder instead of a broken image (`RemoteImage`) | — |
| Unknown or missing icon key | default icon | `ContentIcon.test.tsx` |
