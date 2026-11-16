package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// sampleGames builds a snapshot large enough for every random branch to occur.
func sampleGames(n int) []Game {
	games := make([]Game, n)
	for i := range games {
		games[i] = Game{
			SteamAppID:      1000 + i,
			Type:            "game",
			Name:            fmt.Sprintf("Sample Game %d", i),
			Description:     "A game.",
			PriceCents:      999 + i*100,
			Genres:          []string{"Action", "RPG"},
			ReleaseDate:     "2020-01-02",
			Recommendations: 1000 * (i + 1),
		}
	}
	return games
}

func TestGenerateIsDeterministic(t *testing.T) {
	games := sampleGames(50)

	first := Generate(games, 7)
	second := Generate(games, 7)
	// DeepEqual compares the whole nested structure, value by value.
	if !reflect.DeepEqual(first, second) {
		t.Error("the same snapshot and seed produced two different datasets")
	}

	// The input order must not matter either: Generate sorts by app id itself.
	reversed := make([]Game, len(games))
	for i, game := range games {
		reversed[len(games)-1-i] = game
	}
	if !reflect.DeepEqual(first, Generate(reversed, 7)) {
		t.Error("the order of the snapshot changed the result")
	}

	if reflect.DeepEqual(first, Generate(games, 8)) {
		t.Error("a different seed produced an identical dataset")
	}
}

func TestGenerateInvariants(t *testing.T) {
	games := sampleGames(200)
	dataset := Generate(games, 2027)

	sellerNames := map[string]bool{}
	for _, s := range dataset.Sellers {
		sellerNames[s.Name] = true
	}

	slugs := map[string]bool{}
	keys := map[string]bool{}
	steamGlobal := map[int]bool{}
	stock := map[string]int{} // how many offers are sold out / low / well stocked

	for _, p := range dataset.Products {
		if slugs[p.Slug] {
			t.Errorf("duplicate product slug %q", p.Slug)
		}
		slugs[p.Slug] = true

		if p.PlatformSlug == "steam" && p.RegionSlug == "global" {
			steamGlobal[p.SteamAppID] = true
		}
		if len(p.Offers) < 1 || len(p.Offers) > 6 {
			t.Errorf("%s has %d offers, want 1-6", p.Slug, len(p.Offers))
		}

		offerSellers := map[string]bool{}
		for _, o := range p.Offers {
			if !sellerNames[o.SellerName] {
				t.Errorf("%s: offer from unknown seller %q", p.Slug, o.SellerName)
			}
			if offerSellers[o.SellerName] {
				t.Errorf("%s: seller %q has two offers", p.Slug, o.SellerName)
			}
			offerSellers[o.SellerName] = true

			// The same rules the database enforces with CHECK constraints.
			if o.PriceCents <= 0 || o.PriceCents > o.OriginalPriceCents {
				t.Errorf("%s: price %d with reference price %d", p.Slug, o.PriceCents, o.OriginalPriceCents)
			}

			switch n := len(o.Keys); {
			case n == 0:
				stock["sold out"]++
			case n <= 2:
				stock["low"]++
			default:
				stock["in stock"]++
			}
			for _, key := range o.Keys {
				if keys[key] {
					t.Errorf("duplicate key %q", key)
				}
				keys[key] = true
				if !strings.HasPrefix(key, "DEMO-") || len(key) != len("DEMO-XXXXX-XXXXX-XXXXX") {
					t.Errorf("key %q does not look like DEMO-XXXXX-XXXXX-XXXXX", key)
				}
			}
		}
	}

	for _, game := range games {
		if !steamGlobal[game.SteamAppID] {
			t.Errorf("game %d has no Steam Global product", game.SteamAppID)
		}
	}
	// The offer list UI has three stock states; the demo data must contain them all.
	for _, state := range []string{"sold out", "low", "in stock"} {
		if stock[state] == 0 {
			t.Errorf("no offer is %s", state)
		}
	}
	if len(dataset.Products) <= len(games) {
		t.Errorf("%d products for %d games: no extra platform/region variants were generated", len(dataset.Products), len(games))
	}
	if len(dataset.Genres) != 2 {
		t.Errorf("%d genres, want 2 (each genre listed once)", len(dataset.Genres))
	}
}

func TestGenerateMakesSlugsUniqueForGamesWithTheSameName(t *testing.T) {
	games := []Game{
		{SteamAppID: 1, Type: "game", Name: "Same Name", PriceCents: 1000},
		{SteamAppID: 2, Type: "game", Name: "Same Name", PriceCents: 1000},
	}
	slugs := map[string]bool{}
	for _, p := range Generate(games, 1).Products {
		if slugs[p.Slug] {
			t.Fatalf("duplicate slug %q", p.Slug)
		}
		slugs[p.Slug] = true
	}
}

func TestProductName(t *testing.T) {
	tests := []struct {
		v    variant
		want string
	}{
		{variant{platform: platforms[0], region: regions[0]}, "Hades (PC) Steam Key GLOBAL"},
		{variant{platform: platforms[1], region: regions[1]}, "Hades (PC) GOG.com Key EUROPE"},
		{variant{platform: platforms[3], region: regions[2]}, "Hades (Xbox Series X|S) Xbox Live Key UNITED STATES"},
		{variant{platform: platforms[4], region: regions[0]}, "Hades (PS5) PSN Key GLOBAL"},
	}
	for _, tt := range tests {
		if got := productName("Hades", tt.v); got != tt.want {
			t.Errorf("productName() = %q, want %q", got, tt.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Baldur's Gate 3 (PC) Steam Key GLOBAL": "baldur-s-gate-3-pc-steam-key-global",
		"  --Hello,   World!--  ":               "hello-world",
		"NieR:Automata":                         "nier-automata",
		"Pokémon":                               "pok-mon",
		"!!!":                                   "",
	}
	for input, want := range tests {
		if got := slugify(input); got != want {
			t.Errorf("slugify(%q) = %q, want %q", input, got, want)
		}
	}
}

// The committed snapshot is input to every seeded database, so a broken or
// accidentally emptied file should fail here rather than at deploy time.
func TestGenerateFromCommittedSnapshot(t *testing.T) {
	data, err := os.ReadFile("../../seed/steam_games.json")
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var games []Game
	if err := json.Unmarshal(data, &games); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(games) < 150 {
		t.Fatalf("snapshot has %d games, want at least 150", len(games))
	}

	dataset := Generate(games, 2027)

	offers, keys := 0, 0
	for _, p := range dataset.Products {
		if p.Name == "" || p.Slug == "" || p.CoverURL == "" {
			t.Errorf("product of app %d is missing a name, slug or cover", p.SteamAppID)
		}
		offers += len(p.Offers)
		for _, o := range p.Offers {
			keys += len(o.Keys)
		}
	}
	for _, g := range dataset.Genres {
		if g.Slug == "early-access" {
			t.Error("Early Access must not become a genre")
		}
	}
	t.Logf("%d games -> %d products, %d offers, %d keys, %d genres",
		len(games), len(dataset.Products), offers, keys, len(dataset.Genres))
}
