package catalog_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
	"github.com/Jokubas-J/Eneba/backend/internal/testdb"
)

// searchCases is the table the ranking was designed against. It runs on the REAL
// catalog (the committed Steam snapshot), because search quality is a property of
// the algorithm and the data together. docs/search.md shows the same table.
//
// wantFirst is text that the name of the first result must contain (compared
// without case); "" means the query must return nothing.
var searchCases = []struct {
	kind      string
	query     string
	wantFirst string
}{
	// One or two wrong, missing or swapped letters.
	{"typo", "witchr 3", "the witcher 3"},
	{"typo", "wicher", "the witcher 3"},
	{"typo", "cyberpnk", "cyberpunk 2077"},
	{"typo", "skyrm", "skyrim"},
	{"typo", "eldn ring", "elden ring"},
	{"typo", "hogwarst", "hogwarts legacy"},
	{"typo", "stardw valley", "stardew valley"},
	{"typo", "red dead redemtion", "red dead redemption 2"},
	{"typo", "battlefeild", "battlefield"},
	{"typo", "teraria", "terraria"},
	{"typo", "civilzation", "civilization"},
	{"typo", "fallot 4", "fallout 4"},
	{"typo", "resident evl", "resident evil"}, // short word: lower bar in a multi-word query

	// The visitor is still typing, or types only part of the title.
	{"partial", "witc", "the witcher 3"},
	{"partial", "cyber", "cyberpunk 2077"},
	{"partial", "hollow kn", "hollow knight"},
	{"partial", "red dead 2", "red dead redemption 2"},
	{"partial", "resident evil 4", "resident evil 4"},
	{"partial", "half life 2", "half-life 2"},
	{"partial", "dark souls", "dark souls"},
	{"partial", "god of war", "god of war"},
	{"partial", "souls", "dark souls"},

	// Abbreviations: matched against the first letters of the words of the name.
	{"abbreviation", "gta", "grand theft auto v"},
	{"abbreviation", "rdr2", "red dead redemption 2"},
	{"abbreviation", "bg3", "baldur's gate 3"},
	{"abbreviation", "re4", "resident evil 4"},

	// Synonyms (search_synonyms table): numerals, abbreviations, other spellings.
	{"synonym", "gta 5", "grand theft auto v"},
	{"synonym", "gta v", "grand theft auto v"},
	{"synonym", "grand theft auto 5", "grand theft auto v"},
	{"synonym", "dark souls 3", "dark souls iii"},
	{"synonym", "civilization 6", "civilization vi"},
	{"synonym", "civ 6", "civilization vi"},
	{"synonym", "ff 16", "final fantasy xvi"},
	{"synonym", "re 4", "resident evil 4"},
	{"synonym", "spiderman", "spider-man"},
	{"synonym", "halflife 2", "half-life 2"},
	{"synonym", "repo", "r.e.p.o"},

	// Capital letters and punctuation must not matter.
	{"spelling", "THE WITCHER", "the witcher 3"},
	{"spelling", "baldurs gate", "baldur's gate 3"},
	{"spelling", "baldur's gate 3", "baldur's gate 3"},
	{"spelling", "half-life", "half-life"},
	{"spelling", "  elden   ring  ", "elden ring"},

	// Platform and region are part of the product name, so they are searchable.
	{"platform", "skyrim psn", "psn key"},
	{"platform", "terraria xbox", "xbox live key"},

	// Nothing in the catalog should match these. Minecraft is not sold on Steam, so
	// it is not in the snapshot: the right answer for "mincraft" is "no results",
	// not the nearest wrong game.
	{"no match", "zzxqv", ""},
	{"no match", "qwerty", ""},
	{"no match", "mincraft", ""},
	{"no match", "asdf ghjk", ""},
}

// loadRealCatalog fills the test database with the same data production has.
func loadRealCatalog(t *testing.T) (*catalog.Service, context.Context) {
	t.Helper()
	data, err := os.ReadFile("../../seed/steam_games.json")
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var games []seed.Game
	if err := json.Unmarshal(data, &games); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	return newService(t, seed.Generate(games, 2027))
}

func TestSearchRanking(t *testing.T) {
	service, ctx := loadRealCatalog(t)

	// The log doubles as the results table of docs/search.md (run with -v).
	t.Log("| Kind | Query | Results | First result |")
	t.Log("|---|---|---|---|")

	for _, tc := range searchCases {
		result, err := service.List(ctx, catalog.ListFilter{Text: tc.query})
		if err != nil {
			t.Fatalf("List(%q) error = %v", tc.query, err)
		}

		first := "(none)"
		if len(result.Items) > 0 {
			first = result.Items[0].Name
		}
		t.Logf("| %s | `%s` | %d | %s |", tc.kind, strings.TrimSpace(tc.query), result.Total, first)

		if tc.wantFirst == "" {
			if result.Total != 0 {
				t.Errorf("%q: got %d results (first: %s), want none", tc.query, result.Total, first)
			}
			continue
		}
		if len(result.Items) == 0 {
			t.Errorf("%q: no results, want first to contain %q", tc.query, tc.wantFirst)
			continue
		}
		if !strings.Contains(strings.ToLower(first), tc.wantFirst) {
			t.Errorf("%q: first result is %q, want it to contain %q", tc.query, first, tc.wantFirst)
		}
	}
}

// suggestionCases is the table for the query suggestions of the dropdown (top part).
// wantFirst is the first suggestion expected; "" means none.
var suggestionCases = []struct {
	typed     string
	wantFirst string
	wantAlso  []string
}{
	{"resid", "resident evil", []string{"resident evil 2", "resident evil 4"}},
	{"gta", "gta", []string{"gta v", "gta 5"}},
	{"witcher", "witcher", []string{"witcher 3", "the witcher 3"}},
	{"witchr", "witcher", nil}, // typo: found by similarity
	{"elden", "elden ring", nil},
	{"stard", "stardew valley", nil},
	{"souls", "dark souls", nil}, // a later word of the suggestion
	{"zzxqv", "", nil},
}

func TestQuerySuggestions(t *testing.T) {
	service, ctx := loadRealCatalog(t)

	t.Log("| Typed | Suggestions |")
	t.Log("|---|---|")
	for _, tc := range suggestionCases {
		result, err := service.Suggest(ctx, tc.typed)
		if err != nil {
			t.Fatalf("Suggest(%q) error = %v", tc.typed, err)
		}
		t.Logf("| %q | %s |", tc.typed, strings.Join(result.Suggestions, ", "))

		if len(result.Suggestions) > catalog.QuerySuggestLimit {
			t.Errorf("%q: %d suggestions, want at most %d", tc.typed, len(result.Suggestions), catalog.QuerySuggestLimit)
		}
		if tc.wantFirst == "" {
			if len(result.Suggestions) != 0 {
				t.Errorf("%q: suggestions %v, want none", tc.typed, result.Suggestions)
			}
			continue
		}
		if len(result.Suggestions) == 0 || result.Suggestions[0] != tc.wantFirst {
			t.Errorf("%q: suggestions %v, want %q first", tc.typed, result.Suggestions, tc.wantFirst)
		}
		for _, want := range tc.wantAlso {
			if !slices.Contains(result.Suggestions, want) {
				t.Errorf("%q: suggestions %v, want them to include %q", tc.typed, result.Suggestions, want)
			}
		}
	}
}

// A suggestion that leads to "No products found" would be worse than none.
func TestEveryCuratedSuggestionFindsProducts(t *testing.T) {
	service, ctx := loadRealCatalog(t)

	curated, err := service.CuratedSuggestions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(curated) < 100 {
		t.Fatalf("only %d curated suggestions", len(curated))
	}
	for _, query := range curated {
		result, err := service.List(ctx, catalog.ListFilter{Text: query})
		if err != nil {
			t.Fatalf("List(%q) error = %v", query, err)
		}
		if result.Total == 0 {
			t.Errorf("curated suggestion %q finds no products", query)
		}
	}
}

func TestVisitorSearchesBecomeSuggestions(t *testing.T) {
	service, ctx := newService(t, fixture())

	suggestions := func(typed string) []string {
		t.Helper()
		result, err := service.Suggest(ctx, typed)
		if err != nil {
			t.Fatal(err)
		}
		return result.Suggestions
	}
	search := func(text string, page int) {
		t.Helper()
		if _, err := service.List(ctx, catalog.ListFilter{Text: text, Page: page}); err != nil {
			t.Fatal(err)
		}
	}

	// The fixture has products named Alpha, bravo, Charlie, Delta.
	search("charlie", 1)
	if got := suggestions("charl"); len(got) != 0 {
		t.Errorf("after one search: suggestions %v, want none yet (needs 2 searches)", got)
	}

	search("CHARLIE!", 1) // simplified to the same query
	if got := suggestions("charl"); !slices.Equal(got, []string{"charlie"}) {
		t.Errorf("after two searches: suggestions %v, want [charlie]", got)
	}

	// No results, a later page, or typing in the dropdown are not counted.
	search("zzxqv", 1)
	search("zzxqv", 1)
	search("delta", 2)
	search("delta", 2)
	for range 3 {
		suggestions("bravo")
	}
	// (Curated suggestions such as "deltarune" may still appear for these.)
	for _, query := range []string{"zzxqv", "delta", "bravo"} {
		if got := suggestions(query); slices.Contains(got, query) {
			t.Errorf("suggestions for %q = %v, want %q not to be among them", query, got, query)
		}
	}
}

func TestSearchBehaviour(t *testing.T) {
	service, ctx := loadRealCatalog(t)

	t.Run("every word must match, so results stay on topic", func(t *testing.T) {
		result, err := service.List(ctx, catalog.ListFilter{Text: "witchr 3"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Total == 0 {
			t.Fatal("no results")
		}
		for _, item := range result.Items {
			if !strings.Contains(strings.ToLower(item.Name), "witcher 3") {
				t.Errorf("unrelated product in the results: %s", item.Name)
			}
		}
	})

	t.Run("an explicit sort overrides best-match order but keeps the matches", func(t *testing.T) {
		byRelevance, err := service.List(ctx, catalog.ListFilter{Text: "resident evil"})
		if err != nil {
			t.Fatal(err)
		}
		byPrice, err := service.List(ctx, catalog.ListFilter{Text: "resident evil", Sort: catalog.SortPriceAsc})
		if err != nil {
			t.Fatal(err)
		}
		if byPrice.Total != byRelevance.Total || byPrice.Total < 2 {
			t.Fatalf("totals: %d by price, %d by relevance; want equal and at least 2", byPrice.Total, byRelevance.Total)
		}
		last := 0
		for _, item := range byPrice.Items {
			if item.PriceCents == nil {
				continue // sold-out products come last
			}
			if *item.PriceCents < last {
				t.Errorf("prices are not ascending: %d after %d", *item.PriceCents, last)
			}
			last = *item.PriceCents
		}
	})

	t.Run("search combines with the other filters", func(t *testing.T) {
		result, err := service.List(ctx, catalog.ListFilter{Text: "witcher", Regions: []string{"global"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.Total == 0 {
			t.Fatal("no results")
		}
		for _, item := range result.Items {
			if item.Region.Slug != "global" {
				t.Errorf("%s is %s, want only global", item.Name, item.Region.Slug)
			}
		}
	})

	t.Run("a query of punctuation only is no search at all", func(t *testing.T) {
		all, err := service.List(ctx, catalog.ListFilter{})
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.List(ctx, catalog.ListFilter{Text: " !!! -- "})
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != all.Total {
			t.Errorf("total = %d, want the whole catalog (%d)", result.Total, all.Total)
		}
	})

	t.Run("LIKE wildcards in the query are not wildcards", func(t *testing.T) {
		result, err := service.List(ctx, catalog.ListFilter{Text: "%%%"})
		if err != nil {
			t.Fatal(err)
		}
		all, _ := service.List(ctx, catalog.ListFilter{})
		// "%%%" simplifies to nothing, so it is "no search" rather than a pattern.
		// (That no % or _ survives simplification is checked in TestNormalizeQuery.)
		if result.Total != all.Total {
			t.Errorf("total for %%%%%% = %d, want %d", result.Total, all.Total)
		}
	})

	t.Run("suggestions are the best few matches", func(t *testing.T) {
		result, err := service.Suggest(ctx, "witchr")
		if err != nil {
			t.Fatal(err)
		}
		items := result.Items
		if result.Total < len(items) {
			t.Errorf("total = %d, fewer than the %d items shown", result.Total, len(items))
		}
		if len(items) == 0 || len(items) > catalog.SuggestLimit {
			t.Fatalf("got %d suggestions, want 1-%d", len(items), catalog.SuggestLimit)
		}
		if !strings.Contains(strings.ToLower(items[0].Name), "witcher") {
			t.Errorf("first suggestion is %s", items[0].Name)
		}
		if items[0].PriceCents != nil && items[0].DiscountPercent == nil {
			t.Error("suggestion has a price but no discount")
		}
	})

	t.Run("too short or empty queries give no suggestions", func(t *testing.T) {
		for _, query := range []string{"", " ", "w", "!?", "a "} {
			result, err := service.Suggest(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			if result.Items == nil || len(result.Items) != 0 || result.Suggestions == nil || len(result.Suggestions) != 0 {
				t.Errorf("Suggest(%q) = %+v, want empty lists", query, result)
			}
		}
	})
}

// The Go and SQL simplification rules must agree, or queries and names would be
// compared in different forms.
func TestNormalizeQueryMatchesTheDatabase(t *testing.T) {
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for _, input := range []string{
		"Baldur's Gate 3 (PC) Steam Key GLOBAL",
		"NieR:Automata™",
		"  Half-Life 2:   Episode One ",
		"Marvel’s Spider-Man Remastered",
		"Ratchet & Clank: Rift Apart (Xbox Series X|S)",
		"100% Orange_Juice",
		"Pokémon",
		"!!!",
	} {
		var fromSQL string
		if err := pool.QueryRow(ctx, `SELECT normalize_search($1)`, input).Scan(&fromSQL); err != nil {
			t.Fatalf("normalize_search(%q): %v", input, err)
		}
		fromGo := strings.Join(catalog.NormalizeQuery(input), " ")
		if fromGo != fromSQL {
			t.Errorf("%q: Go gives %q, SQL gives %q", input, fromGo, fromSQL)
		}
	}
}

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Witchr 3", []string{"witchr", "3"}},
		{"  baldur's   GATE ", []string{"baldurs", "gate"}},
		{"half-life", []string{"half", "life"}},
		{"100%_off", []string{"100", "off"}},
		{"", []string{}},
		{"?!", []string{}},
		{"a b c d e f g h i j", []string{"a", "b", "c", "d", "e", "f", "g", "h"}}, // capped at 8 words
		{strings.Repeat("x", 300), []string{strings.Repeat("x", 100)}},            // capped at 100 characters
	}
	for _, tt := range tests {
		got := catalog.NormalizeQuery(tt.input)
		if len(got) == 0 && len(tt.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("NormalizeQuery(%q) = %v, want %v", fmt.Sprintf("%.20s", tt.input), got, tt.want)
		}
	}
}
