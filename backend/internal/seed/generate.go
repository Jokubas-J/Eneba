package seed

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
)

// Dataset is everything the loader writes to the database. It is built in memory
// first, by a function with no side effects, so that the rules below can be unit
// tested without a database.
type Dataset struct {
	Platforms []Named
	Regions   []Named
	Genres    []Named
	Sellers   []Seller
	Products  []Product
}

// Named is a row of one of the small lookup tables (platforms, regions, genres).
type Named struct {
	Slug string
	Name string
}

type Seller struct {
	Name        string
	Rating      float64 // 0-10 with one decimal
	RatingCount int
	IsVerified  bool
}

// Product is one sellable variant: a game on one platform for one region.
type Product struct {
	Slug         string
	Name         string
	Description  string
	Type         string
	PlatformSlug string
	RegionSlug   string
	CoverURL     string
	ReleaseDate  string // YYYY-MM-DD or ""
	Developer    string
	Publisher    string
	Popularity   int
	SteamAppID   int
	GenreSlugs   []string
	Offers       []Offer
}

type Offer struct {
	SellerName         string
	PriceCents         int
	OriginalPriceCents int
	Keys               []string // one entry per key in stock; empty = sold out
}

// The lookup data is fixed, not random. Real data: none of this comes from Steam.
var (
	platforms = []Named{
		{"steam", "Steam"},
		{"gog", "GOG.com"},
		{"epic-games", "Epic Games"},
		{"xbox-live", "Xbox Live"},
		{"psn", "PSN"},
	}
	regions = []Named{
		{"global", "Global"},
		{"europe", "Europe"},
		{"united-states", "United States"},
	}
	// Invented seller names; any resemblance to real merchants is accidental.
	sellers = []Seller{
		{"Keyhaven", 9.8, 48210, true},
		{"Pixel Harbor", 9.7, 31544, true},
		{"NordKeys", 9.6, 27803, true},
		{"Lootbay", 9.5, 19377, true},
		{"GameVault 24", 9.4, 15120, true},
		{"Amber Codes", 9.3, 9842, false},
		{"Quickdrop Games", 9.1, 7410, true},
		{"Baltic Keys", 9.0, 5236, false},
		{"Respawn Market", 8.8, 3187, false},
		{"Cartridge Club", 8.6, 1904, false},
		{"Byte Bazaar", 8.3, 951, false},
		{"Second Player", 7.9, 312, false},
	}
)

// variant describes one platform/region combination a game can be sold as.
type variant struct {
	platform Named
	region   Named
	// How this variant's reference price and popularity relate to Steam's own
	// figures (regional keys are a little cheaper and less sought after).
	priceFactor      float64
	popularityFactor float64
}

// Generate turns the Steam snapshot into the full demo catalog.
//
// All randomness comes from one generator started from seedValue, and the games are
// processed in a fixed order, so the same snapshot and seed always give exactly the
// same products, prices and keys. That makes bugs reproducible and lets tests
// assert on the output.
func Generate(games []Game, seedValue uint64) Dataset {
	// math/rand/v2 with an explicit source: no hidden global state. PCG is one of
	// its two built-in algorithms; cryptographic quality is not needed for demo data.
	rng := rand.New(rand.NewPCG(seedValue, seedValue))

	// Copy before sorting so the caller's slice is not reordered behind its back.
	sorted := make([]Game, len(games))
	copy(sorted, games)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].SteamAppID < sorted[b].SteamAppID })

	dataset := Dataset{Platforms: platforms, Regions: regions, Sellers: sellers}

	genreSeen := map[string]bool{}
	slugSeen := map[string]bool{}
	keySeen := map[string]bool{}

	for _, game := range sorted {
		var genreSlugs []string
		for _, name := range game.Genres {
			slug := slugify(name)
			// Steam files "Early Access" under genres, but it is a release status,
			// not a kind of game, so it would be a misleading store filter.
			if slug == "" || slug == "early-access" {
				continue
			}
			genreSlugs = append(genreSlugs, slug)
			if !genreSeen[slug] {
				genreSeen[slug] = true
				dataset.Genres = append(dataset.Genres, Named{Slug: slug, Name: name})
			}
		}

		// A few games have no portrait cover at the predictable address; the
		// landscape header image is used instead (the card crops it to fit).
		coverURL := game.CoverURL
		if coverURL == "" {
			coverURL = game.HeaderURL
		}

		for _, v := range pickVariants(rng) {
			name := productName(game.Name, v)
			slug := slugify(name)
			if slugSeen[slug] {
				// Two different Steam apps can share a name (a remaster and the
				// original, for example); the app id makes the slug unique again.
				slug = fmt.Sprintf("%s-%d", slug, game.SteamAppID)
			}
			slugSeen[slug] = true

			referencePrice := int(math.Round(float64(game.PriceCents) * v.priceFactor))

			dataset.Products = append(dataset.Products, Product{
				Slug:         slug,
				Name:         name,
				Description:  game.Description,
				Type:         game.Type,
				PlatformSlug: v.platform.Slug,
				RegionSlug:   v.region.Slug,
				CoverURL:     coverURL,
				ReleaseDate:  game.ReleaseDate,
				Developer:    game.Developer,
				Publisher:    game.Publisher,
				Popularity:   int(float64(game.Recommendations) * v.popularityFactor),
				SteamAppID:   game.SteamAppID,
				GenreSlugs:   genreSlugs,
				Offers:       makeOffers(rng, referencePrice, keySeen),
			})
		}
	}
	return dataset
}

// pickVariants decides which platform/region versions of a game exist. Every game
// has the Steam Global key (the data really is from Steam); the others are synthetic
// and appear with fixed probabilities so that the platform and region filters and
// the "other versions" comparison have something to show.
func pickVariants(rng *rand.Rand) []variant {
	steam, global, europe, unitedStates := platforms[0], regions[0], regions[1], regions[2]

	variants := []variant{{steam, global, 1.0, 1.0}}
	if rng.Float64() < 0.55 {
		variants = append(variants, variant{steam, europe, 0.92, 0.6})
	}
	if rng.Float64() < 0.20 {
		variants = append(variants, variant{steam, unitedStates, 0.95, 0.3})
	}
	if rng.Float64() < 0.30 {
		// One version on another platform (index 1..4 skips Steam itself).
		other := platforms[1+rng.IntN(len(platforms)-1)]
		region := global
		if rng.Float64() < 0.5 {
			region = europe
		}
		variants = append(variants, variant{other, region, 0.97, 0.4})
	}
	return variants
}

// productName follows Eneba's naming, where platform and region are part of the
// title: "The Witcher 3 (PC) Steam Key GLOBAL".
func productName(game string, v variant) string {
	device := " (PC)"
	switch v.platform.Slug {
	case "xbox-live":
		device = " (Xbox Series X|S)"
	case "psn":
		device = " (PS5)"
	}
	return fmt.Sprintf("%s%s %s Key %s", game, device, v.platform.Name, strings.ToUpper(v.region.Name))
}

// makeOffers creates one to six offers from different sellers for one product.
func makeOffers(rng *rand.Rand, referencePrice int, keySeen map[string]bool) []Offer {
	count := 1 + rng.IntN(6)
	// Perm returns the numbers 0..n-1 in random order; taking the first "count"
	// picks that many DIFFERENT sellers (the schema allows one offer per seller).
	order := rng.Perm(len(sellers))[:count]

	offers := make([]Offer, 0, count)
	for _, sellerIndex := range order {
		// Marketplace prices sit 5-60% below the reference (Steam) price.
		price := int(float64(referencePrice) * (0.40 + 0.55*rng.Float64()))
		// Shop-style ending: 1234 -> 1239, unless that would pass the reference.
		if rounded := price - price%10 + 9; rounded <= referencePrice {
			price = rounded
		}
		if price < 1 {
			price = 1
		}

		offers = append(offers, Offer{
			SellerName:         sellers[sellerIndex].Name,
			PriceCents:         price,
			OriginalPriceCents: referencePrice,
			Keys:               makeKeys(rng, stockSize(rng), keySeen),
		})
	}
	return offers
}

// stockSize makes the three stock states of the offer list all appear in the demo:
// sold out (10%), "only N left" (15%) and comfortably in stock.
func stockSize(rng *rand.Rand) int {
	roll := rng.Float64()
	switch {
	case roll < 0.10:
		return 0
	case roll < 0.25:
		return 1 + rng.IntN(2)
	default:
		return 3 + rng.IntN(13)
	}
}

// No 0/O or 1/I: characters that are easy to confuse when a key is typed in.
const keyAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// makeKeys creates n unique fake license keys such as DEMO-7KQ2M-XW9RT-B4HNC.
// The DEMO prefix makes it obvious that they cannot be redeemed anywhere.
func makeKeys(rng *rand.Rand, n int, seen map[string]bool) []string {
	keys := make([]string, 0, n)
	for len(keys) < n {
		var b strings.Builder
		b.WriteString("DEMO")
		for group := 0; group < 3; group++ {
			b.WriteByte('-')
			for i := 0; i < 5; i++ {
				b.WriteByte(keyAlphabet[rng.IntN(len(keyAlphabet))])
			}
		}
		key := b.String()
		// key_value is UNIQUE in the database; a repeat is astronomically unlikely
		// (32^15 combinations) but checking costs nothing.
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

// slugify makes a URL-safe identifier: lower case, a-z and 0-9 kept, every other
// run of characters replaced by a single "-".
// "Baldur's Gate 3 (PC) Steam Key GLOBAL" -> "baldur-s-gate-3-pc-steam-key-global"
func slugify(s string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range strings.ToLower(s) {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if !isAlnum {
			pendingDash = true
			continue
		}
		// The dash is written only when another letter follows, so slugs never
		// start or end with one.
		if pendingDash && b.Len() > 0 {
			b.WriteByte('-')
		}
		pendingDash = false
		b.WriteRune(r)
	}
	return b.String()
}
