package catalog_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
)

// counts turns a facet group into "slug:count" pairs in the returned order.
func counts(options []catalog.FacetOption) []string {
	out := make([]string, len(options))
	for i, option := range options {
		out[i] = option.Slug + ":" + strconv.Itoa(option.Count)
	}
	return out
}

// The fixture (see list_test.go):
//
//	alpha    steam/global  action       1000
//	bravo    steam/europe  rpg          500
//	charlie  gog/global    action, rpg  3000
//	delta    gog/europe    rpg          sold out
func TestFacets(t *testing.T) {
	service, ctx := newService(t, fixture())

	tests := []struct {
		name      string
		filter    catalog.ListFilter
		platforms []string
		regions   []string
		genres    []string
	}{
		{
			name:      "no filter: every option with its full count",
			filter:    catalog.ListFilter{},
			platforms: []string{"gog:2", "steam:2"},
			regions:   []string{"europe:2", "global:2"},
			genres:    []string{"rpg:3", "action:2"}, // charlie is counted under both genres
		},
		{
			name:      "a group ignores its own selection but narrows the others",
			filter:    catalog.ListFilter{Platforms: []string{"steam"}},
			platforms: []string{"gog:2", "steam:2"}, // unchanged: GOG can still be added
			regions:   []string{"europe:1", "global:1"},
			genres:    []string{"action:1", "rpg:1"},
		},
		{
			name:      "two groups: each is counted under the other one's selection",
			filter:    catalog.ListFilter{Platforms: []string{"steam"}, Regions: []string{"europe"}},
			platforms: []string{"gog:1", "steam:1"},     // products in Europe
			regions:   []string{"europe:1", "global:1"}, // products on Steam
			genres:    []string{"rpg:1", "action:0"},    // only bravo; an empty option is still listed
		},
		{
			name:      "genre selection narrows platforms, not the genre list",
			filter:    catalog.ListFilter{Genres: []string{"action"}},
			platforms: []string{"gog:1", "steam:1"},
			regions:   []string{"global:2", "europe:0"},
			genres:    []string{"rpg:3", "action:2"},
		},
		{
			name:      "price bounds apply to every group",
			filter:    catalog.ListFilter{PriceMax: intPtr(1000)},
			platforms: []string{"steam:2", "gog:0"},
			regions:   []string{"europe:1", "global:1"},
			genres:    []string{"action:1", "rpg:1"},
		},
		{
			name:      "the search text applies to every group",
			filter:    catalog.ListFilter{Text: "charlie"},
			platforms: []string{"gog:1", "steam:0"},
			regions:   []string{"global:1", "europe:0"},
			genres:    []string{"action:1", "rpg:1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			facets := result.Facets
			if got := counts(facets.Platforms); !reflect.DeepEqual(got, tt.platforms) {
				t.Errorf("platforms = %v, want %v", got, tt.platforms)
			}
			if got := counts(facets.Regions); !reflect.DeepEqual(got, tt.regions) {
				t.Errorf("regions = %v, want %v", got, tt.regions)
			}
			if got := counts(facets.Genres); !reflect.DeepEqual(got, tt.genres) {
				t.Errorf("genres = %v, want %v", got, tt.genres)
			}
			if got := counts(facets.Types); !reflect.DeepEqual(got, []string{"game:" + strconv.Itoa(result.Total)}) {
				t.Errorf("types = %v, want game:%d", got, result.Total)
			}
		})
	}

	t.Run("options carry display names", func(t *testing.T) {
		result, err := service.List(ctx, catalog.ListFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Facets.Platforms[0]; got != (catalog.FacetOption{Slug: "gog", Name: "GOG.com", Count: 2}) {
			t.Errorf("first platform option = %+v", got)
		}
		if got := result.Facets.Types[0]; got.Name != "Game" {
			t.Errorf("type option name = %q, want %q", got.Name, "Game")
		}
	})

	t.Run("the ticked option's count equals the result total", func(t *testing.T) {
		// What the sidebar promises next to a checkbox must be what ticking it gives.
		all, err := service.List(ctx, catalog.ListFilter{Regions: []string{"global"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, option := range all.Facets.Platforms {
			ticked, err := service.List(ctx, catalog.ListFilter{Regions: []string{"global"}, Platforms: []string{option.Slug}})
			if err != nil {
				t.Fatal(err)
			}
			if ticked.Total != option.Count {
				t.Errorf("platform %s: sidebar says %d, ticking it lists %d", option.Slug, option.Count, ticked.Total)
			}
		}
	})
}
