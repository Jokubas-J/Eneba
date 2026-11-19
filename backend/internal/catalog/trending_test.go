package catalog_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/Jokubas-J/Eneba/backend/internal/catalog"
	"github.com/Jokubas-J/Eneba/backend/internal/seed"
)

func TestTrendingSearches(t *testing.T) {
	data, err := os.ReadFile("../../seed/steam_games.json")
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var games []seed.Game
	if err := json.Unmarshal(data, &games); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	service, ctx, pool := newServiceWithPool(t, seed.Generate(games, 2027))

	trending := func() []string {
		t.Helper()
		got, err := service.Trending(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("with no searches yet: curated queries for the most popular games", func(t *testing.T) {
		got := trending()
		t.Logf("trending: %v", got)
		if len(got) != catalog.TrendingSize {
			t.Fatalf("got %d queries, want %d", len(got), catalog.TrendingSize)
		}
		// Terraria has the most Steam recommendations in the snapshot.
		if got[0] != "terraria" {
			t.Errorf("first = %q, want terraria", got[0])
		}
		for _, want := range []string{"cyberpunk", "elden ring"} {
			if !slices.Contains(got, want) {
				t.Errorf("%q missing", want)
			}
		}
		// One query per game: "witcher", "witcher 3" and "the witcher 3" all find The
		// Witcher 3, so at most one of them, the shortest.
		witchers := 0
		for _, q := range got {
			if q == "witcher" || q == "witcher 3" || q == "the witcher 3" || q == "witcher 3 remastered" {
				witchers++
			}
		}
		if witchers > 1 {
			t.Errorf("%d Witcher queries in %v, want at most 1", witchers, got)
		}
		seen := map[string]bool{}
		for _, q := range got {
			if seen[q] {
				t.Errorf("%q appears twice", q)
			}
			seen[q] = true
		}
	})

	t.Run("real searches come first, with the usual anti-spam rule", func(t *testing.T) {
		search := func(text string) {
			t.Helper()
			if _, err := service.List(ctx, catalog.ListFilter{Text: text, Page: 1}); err != nil {
				t.Fatal(err)
			}
		}
		aDayPasses := func() {
			t.Helper()
			if _, err := pool.Exec(ctx, `UPDATE search_suggestions SET last_searched_on = last_searched_on - 1`); err != nil {
				t.Fatal(err)
			}
		}
		// "stardew" (a visitor query) on two days; "hades" (curated) once;
		// "ruuust" once (a visitor query on one day only: not public yet);
		// "zzxqv" finds nothing (never recorded).
		search("stardew")
		aDayPasses()
		search("stardew")
		search("stardew")
		search("hades")
		search("ruuust")
		search("zzxqv")

		got := trending()
		t.Logf("trending: %v", got)
		if len(got) < 2 || got[0] != "stardew" || got[1] != "hades" {
			t.Errorf("trending starts %v, want [stardew hades ...] (most searched first)", got)
		}
		for _, banned := range []string{"ruuust", "zzxqv"} {
			if slices.Contains(got, banned) {
				t.Errorf("%q must not be trending", banned)
			}
		}
		if len(got) != catalog.TrendingSize {
			t.Errorf("got %d queries, want %d (filled with curated ones)", len(got), catalog.TrendingSize)
		}
	})

	t.Run("searches older than two weeks no longer count", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE search_suggestions SET last_searched_at = now() - interval '15 days'`); err != nil {
			t.Fatal(err)
		}
		if got := trending(); slices.Contains(got, "stardew") {
			t.Errorf("trending %v still contains an old search", got)
		}
	})
}
