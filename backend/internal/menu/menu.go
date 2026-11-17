// Package menu serves the site menu (GET /api/menu): the sections of the
// "Categories" drawer and the links of the nav strip, from the menu_sections and
// menu_links tables. Design: docs/gap-analysis.md §2 (option B, chosen by the author).
package menu

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Section is one entry of the menu. Kind "link" is itself a link (URL); every other
// kind has Links: hand-written ones (kind "links") or generated from a lookup table.
type Section struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Kind      string  `json:"kind"`
	Placement string  `json:"placement"` // "drawer" or "strip"
	URL       *string `json:"url"`
	Icon      *string `json:"icon"`
	Links     []Link  `json:"links"`
}

type Link struct {
	Label string  `json:"label"`
	URL   string  `json:"url"`
	Icon  *string `json:"icon"`
	Count *int    `json:"count"` // products in stock; only for generated links
}

// Menu returns the active sections in order, each with its links.
func (s *Service) Menu(ctx context.Context) ([]Section, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, kind, placement, url, icon
		FROM menu_sections
		WHERE is_active
		ORDER BY placement, sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("load menu sections: %w", err)
	}
	sections, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Section, error) {
		var sec Section
		err := row.Scan(&sec.ID, &sec.Title, &sec.Kind, &sec.Placement, &sec.URL, &sec.Icon)
		sec.Links = []Link{}
		return sec, err
	})
	if err != nil {
		return nil, fmt.Errorf("read menu sections: %w", err)
	}

	written, err := s.writtenLinks(ctx)
	if err != nil {
		return nil, err
	}
	// Generated lists are built at most once per kind, and only if a section asks.
	generated := map[string][]Link{}
	for i := range sections {
		sec := &sections[i]
		switch sec.Kind {
		case "links":
			if links, ok := written[sec.ID]; ok {
				sec.Links = links
			}
		case "platforms", "genres", "regions":
			if _, done := generated[sec.Kind]; !done {
				if generated[sec.Kind], err = s.generatedLinks(ctx, sec.Kind); err != nil {
					return nil, err
				}
			}
			sec.Links = generated[sec.Kind]
		}
	}
	return sections, nil
}

// writtenLinks returns the hand-written links of every section, keyed by section id.
func (s *Service) writtenLinks(ctx context.Context) (map[int64][]Link, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT section_id, label, url, icon
		FROM menu_links
		ORDER BY section_id, sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("load menu links: %w", err)
	}
	defer rows.Close()
	links := map[int64][]Link{}
	for rows.Next() {
		var sectionID int64
		var l Link
		if err := rows.Scan(&sectionID, &l.Label, &l.URL, &l.Icon); err != nil {
			return nil, fmt.Errorf("read menu link: %w", err)
		}
		links[sectionID] = append(links[sectionID], l)
	}
	return links, rows.Err()
}

// generatedQueries list each lookup table's entries that have something in stock,
// with their counts and icons, largest first. The kind is one of three fixed keys,
// never user input, so each query is a fixed string.
var generatedQueries = map[string]string{
	"platforms": `
		SELECT l.platform_slug, l.platform_name, ti.icon, count(*)::int
		FROM product_listing l
		LEFT JOIN taxonomy_icons ti ON ti.kind = 'platform' AND ti.slug = l.platform_slug
		WHERE l.offer_count > 0
		GROUP BY l.platform_slug, l.platform_name, ti.icon
		ORDER BY 4 DESC, 2`,
	"regions": `
		SELECT l.region_slug, l.region_name, ti.icon, count(*)::int
		FROM product_listing l
		LEFT JOIN taxonomy_icons ti ON ti.kind = 'region' AND ti.slug = l.region_slug
		WHERE l.offer_count > 0
		GROUP BY l.region_slug, l.region_name, ti.icon
		ORDER BY 4 DESC, 2`,
	"genres": `
		SELECT g.slug, g.name, ti.icon, count(*)::int
		FROM genres g
		JOIN product_genres pg ON pg.genre_id = g.id
		JOIN product_listing l ON l.id = pg.product_id
		LEFT JOIN taxonomy_icons ti ON ti.kind = 'genre' AND ti.slug = g.slug
		WHERE l.offer_count > 0
		GROUP BY g.slug, g.name, ti.icon
		ORDER BY 4 DESC, 2`,
}

// storeParam is the store's URL filter for each generated kind.
var storeParam = map[string]string{"platforms": "platform", "regions": "region", "genres": "genre"}

func (s *Service) generatedLinks(ctx context.Context, kind string) ([]Link, error) {
	rows, err := s.pool.Query(ctx, generatedQueries[kind])
	if err != nil {
		return nil, fmt.Errorf("load %s for the menu: %w", kind, err)
	}
	links, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Link, error) {
		var slug string
		var count int
		var l Link
		err := row.Scan(&slug, &l.Label, &l.Icon, &count)
		// Slugs are lowercase letters, digits and dashes, so they need no escaping.
		l.URL = "/store?" + storeParam[kind] + "=" + slug
		l.Count = &count
		return l, err
	})
	if err != nil {
		return nil, fmt.Errorf("read %s for the menu: %w", kind, err)
	}
	if links == nil {
		links = []Link{} // [] in JSON, not null
	}
	return links, nil
}
