// Package seed builds the demo catalog: it downloads public game data from the
// Steam store ONCE into a JSON snapshot, generates the marketplace data Steam does
// not have (sellers, offers, keys), and loads everything into PostgreSQL.
//
// Only the seed command uses this package. The server never imports it, so the
// running application cannot call Steam.
package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Game is one entry of the committed snapshot (seed/steam_games.json): the subset
// of Steam's data that the application stores.
type Game struct {
	SteamAppID      int      `json:"steamAppId"`
	Type            string   `json:"type"` // "game" or "dlc"
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Developer       string   `json:"developer"`
	Publisher       string   `json:"publisher"`
	PriceCents      int      `json:"priceCents"` // Steam's regular (undiscounted) price in EUR cents
	Genres          []string `json:"genres"`
	ReleaseDate     string   `json:"releaseDate"`     // YYYY-MM-DD, or "" when unknown
	CoverURL        string   `json:"coverUrl"`        // portrait cover, or "" when Steam has none
	HeaderURL       string   `json:"headerUrl"`       // landscape image, always present
	Recommendations int      `json:"recommendations"` // number of user recommendations; used as popularity
	// Screenshots from the store page (at most maxScreenshots), in Steam's order.
	Screenshots []Screenshot `json:"screenshots"`
}

// Screenshot is one image of the store page's gallery: a small version for the
// thumbnail strip and a large one for the full-size view. Only the URLs are kept;
// the images stay on Steam's CDN (hot-linked, like the covers).
type Screenshot struct {
	ThumbnailURL string `json:"thumbnailUrl"` // 600x338
	FullURL      string `json:"fullUrl"`      // 1920x1080
}

// The product page shows one row of thumbnails; more than this is never displayed.
const maxScreenshots = 8

// ErrSkip marks an app that exists but is of no use to the catalog (free to play,
// unreleased, not a game...). The fetch command reports it and carries on.
var ErrSkip = errors.New("skipped")

// errRateLimited is returned when Steam answers 429; the caller waits and retries.
var errRateLimited = errors.New("rate limited by Steam")

const (
	// Public, unauthenticated endpoint that the Steam store pages themselves use.
	// It is undocumented and limited to roughly 200 requests per 5 minutes, which
	// is one more reason the data is fetched once and committed.
	// cc=lt -> prices in EUR; l=english -> English texts and date format.
	storeDetailsURL = "https://store.steampowered.com/api/appdetails?cc=lt&l=english&appids="

	// Portrait "library" cover (600x900), the shape Eneba's product cards use.
	coverURLPattern = "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/%d/library_600x900.jpg"
)

// appDetails mirrors the parts of Steam's JSON that we read. Fields that are not
// listed here are ignored by the decoder.
type appDetails struct {
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	IsFree           bool     `json:"is_free"`
	ShortDescription string   `json:"short_description"`
	HeaderImage      string   `json:"header_image"`
	Developers       []string `json:"developers"`
	Publishers       []string `json:"publishers"`
	// A pointer so that "no price at all" (nil) can be told apart from a price of 0.
	PriceOverview *struct {
		Currency string `json:"currency"`
		Initial  int    `json:"initial"`
	} `json:"price_overview"`
	Genres []struct {
		Description string `json:"description"`
	} `json:"genres"`
	ReleaseDate struct {
		ComingSoon bool   `json:"coming_soon"`
		Date       string `json:"date"`
	} `json:"release_date"`
	Recommendations struct {
		Total int `json:"total"`
	} `json:"recommendations"`
	Screenshots []struct {
		PathThumbnail string `json:"path_thumbnail"`
		PathFull      string `json:"path_full"`
	} `json:"screenshots"`
}

// parseAppDetails converts the body of one appdetails response into a Game.
// It is a pure function (no network), which is what makes it testable.
func parseAppDetails(body []byte, appID int) (Game, error) {
	// The response is an object keyed by the app id:
	//   {"292030": {"success": true, "data": {...}}}
	var envelope map[string]struct {
		Success bool       `json:"success"`
		Data    appDetails `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Game{}, fmt.Errorf("decode response: %w", err)
	}
	entry, ok := envelope[strconv.Itoa(appID)]
	if !ok || !entry.Success {
		return Game{}, fmt.Errorf("%w: Steam has no store page for this id", ErrSkip)
	}
	d := entry.Data

	switch {
	case d.Type != "game" && d.Type != "dlc":
		return Game{}, fmt.Errorf("%w: type is %q", ErrSkip, d.Type)
	case d.ReleaseDate.ComingSoon:
		return Game{}, fmt.Errorf("%w: not released yet", ErrSkip)
	case d.IsFree || d.PriceOverview == nil || d.PriceOverview.Initial <= 0:
		return Game{}, fmt.Errorf("%w: free or without a price", ErrSkip)
	case d.PriceOverview.Currency != "EUR":
		return Game{}, fmt.Errorf("%w: price is in %s, not EUR", ErrSkip, d.PriceOverview.Currency)
	}

	game := Game{
		SteamAppID: appID,
		Type:       d.Type,
		Name:       cleanText(d.Name),
		// Only the SHORT description is kept. It is plain text; the long one is HTML
		// written by the publisher, and showing third-party HTML in our pages would
		// be a cross-site-scripting risk.
		Description:     cleanText(d.ShortDescription),
		PriceCents:      d.PriceOverview.Initial,
		ReleaseDate:     parseSteamDate(d.ReleaseDate.Date),
		HeaderURL:       stripQuery(d.HeaderImage),
		Recommendations: d.Recommendations.Total,
		Genres:          []string{},
		Screenshots:     []Screenshot{},
	}
	if len(d.Developers) > 0 {
		game.Developer = cleanText(d.Developers[0])
	}
	if len(d.Publishers) > 0 {
		game.Publisher = cleanText(d.Publishers[0])
	}
	for _, genre := range d.Genres {
		game.Genres = append(game.Genres, cleanText(genre.Description))
	}
	for _, shot := range d.Screenshots {
		if len(game.Screenshots) == maxScreenshots {
			break
		}
		if shot.PathThumbnail == "" || shot.PathFull == "" {
			continue
		}
		game.Screenshots = append(game.Screenshots, Screenshot{
			ThumbnailURL: stripQuery(shot.PathThumbnail),
			FullURL:      stripQuery(shot.PathFull),
		})
	}
	return game, nil
}

// cleanText decodes HTML entities (Steam sends "&amp;" in plain-text fields) and
// drops trademark signs, which only get in the way of searching and of URL slugs.
func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.NewReplacer("™", "", "®", "", "©", "").Replace(s)
	// Fields splits on any run of white space, so this also collapses double spaces.
	return strings.Join(strings.Fields(s), " ")
}

// parseSteamDate turns "18 May, 2015" into "2015-05-18". Steam's format varies a
// little between games, so a few layouts are tried; an unknown one becomes "".
func parseSteamDate(s string) string {
	// Go describes a date layout by writing the reference date 2 Jan 2006 in the
	// wanted shape (where C# would use "d MMM, yyyy").
	for _, layout := range []string{"2 Jan, 2006", "Jan 2, 2006", "2 January, 2006", "January 2, 2006"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return ""
}

// stripQuery removes the "?t=123" cache-buster Steam appends to image URLs.
func stripQuery(url string) string {
	base, _, _ := strings.Cut(url, "?")
	return base
}

// FetchGame downloads one game's store details and checks whether its portrait
// cover exists.
func FetchGame(ctx context.Context, client *http.Client, appID int) (Game, error) {
	body, err := get(ctx, client, storeDetailsURL+strconv.Itoa(appID))
	if err != nil {
		return Game{}, err
	}
	game, err := parseAppDetails(body, appID)
	if err != nil {
		return Game{}, err
	}

	// Not every game has a portrait cover at the predictable address, so it is
	// checked now with a HEAD request (headers only, no image download) instead of
	// showing broken images later.
	coverURL := fmt.Sprintf(coverURLPattern, appID)
	if exists(ctx, client, coverURL) {
		game.CoverURL = coverURL
	}
	return game, nil
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func exists(ctx context.Context, client *http.Client, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// IsRateLimited reports whether err means "Steam asked us to slow down".
func IsRateLimited(err error) bool {
	return errors.Is(err, errRateLimited)
}
