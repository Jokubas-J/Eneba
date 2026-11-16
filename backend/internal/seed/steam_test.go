package seed

import (
	"errors"
	"testing"
)

// A shortened copy of a real appdetails response (only the fields we read, plus one
// we ignore to show that unknown fields do no harm).
const witcherResponse = `{"292030": {"success": true, "data": {
	"type": "game",
	"name": "The Witcher® 3: Wild Hunt",
	"steam_appid": 292030,
	"is_free": false,
	"short_description": "Geralt &amp; Ciri.  You are a monster slayer.",
	"header_image": "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/292030/header.jpg?t=1793900434",
	"developers": ["CD PROJEKT RED"],
	"publishers": ["CD PROJEKT RED"],
	"price_overview": {"currency": "EUR", "initial": 2999, "final": 599, "discount_percent": 80},
	"genres": [{"id": "3", "description": "RPG"}, {"id": "1", "description": "Action"}],
	"release_date": {"coming_soon": false, "date": "18 May, 2015"},
	"recommendations": {"total": 853406}
}}}`

func TestParseAppDetails(t *testing.T) {
	game, err := parseAppDetails([]byte(witcherResponse), 292030)
	if err != nil {
		t.Fatalf("parseAppDetails() error = %v", err)
	}

	checks := []struct {
		field     string
		got, want any
	}{
		{"SteamAppID", game.SteamAppID, 292030},
		{"Type", game.Type, "game"},
		{"Name (trademark sign removed)", game.Name, "The Witcher 3: Wild Hunt"},
		{"Description (entity decoded, spaces collapsed)", game.Description, "Geralt & Ciri. You are a monster slayer."},
		{"Developer", game.Developer, "CD PROJEKT RED"},
		{"Publisher", game.Publisher, "CD PROJEKT RED"},
		{"PriceCents (regular price, not the sale price)", game.PriceCents, 2999},
		{"ReleaseDate", game.ReleaseDate, "2015-05-18"},
		{"HeaderURL (query removed)", game.HeaderURL, "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/292030/header.jpg"},
		{"Recommendations", game.Recommendations, 853406},
		{"number of genres", len(game.Genres), 2},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.field, c.got, c.want)
		}
	}
}

func TestParseAppDetailsSkips(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown app", `{"1": {"success": false}}`},
		{"free to play", `{"1": {"success": true, "data": {"type": "game", "is_free": true}}}`},
		{"no price", `{"1": {"success": true, "data": {"type": "game"}}}`},
		{"not a game", `{"1": {"success": true, "data": {"type": "music", "price_overview": {"currency": "EUR", "initial": 999}}}}`},
		{"not released", `{"1": {"success": true, "data": {"type": "game", "price_overview": {"currency": "EUR", "initial": 999}, "release_date": {"coming_soon": true}}}}`},
		{"wrong currency", `{"1": {"success": true, "data": {"type": "game", "price_overview": {"currency": "USD", "initial": 999}}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAppDetails([]byte(tt.body), 1)
			if !errors.Is(err, ErrSkip) {
				t.Errorf("error = %v, want ErrSkip", err)
			}
		})
	}
}

func TestParseAppDetailsRejectsInvalidJSON(t *testing.T) {
	_, err := parseAppDetails([]byte(`<html>Access denied</html>`), 1)
	if err == nil || errors.Is(err, ErrSkip) {
		t.Errorf("error = %v, want a decoding error that is not ErrSkip", err)
	}
}

func TestParseSteamDate(t *testing.T) {
	tests := map[string]string{
		"18 May, 2015":  "2015-05-18",
		"May 18, 2015":  "2015-05-18",
		"9 Dec, 2020":   "2020-12-09",
		"Coming soon":   "",
		"Q4 2026":       "",
		"":              "",
		" 1 Jan, 2024 ": "2024-01-01",
	}
	for input, want := range tests {
		if got := parseSteamDate(input); got != want {
			t.Errorf("parseSteamDate(%q) = %q, want %q", input, got, want)
		}
	}
}
