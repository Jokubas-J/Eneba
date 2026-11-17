// Package catalog reads the product catalog for the store: listing, filtering and
// sorting. It contains the service (rules) and the repository (SQL); the HTTP
// handlers that call it live in package httpapi.
package catalog

// Sort is the order of a product list. It is a closed set of named values (the
// nearest Go equivalent of a C# enum); the repository maps each one to a fixed
// ORDER BY clause, so text from the URL never becomes part of the SQL.
type Sort string

const (
	// SortDefault means "the visitor did not choose": best match first when there
	// is a search text, most popular first otherwise. The service resolves it.
	SortDefault   Sort = ""
	SortRelevance Sort = "relevance"
	SortPopular   Sort = "popular"
	SortPriceAsc  Sort = "price_asc"
	SortPriceDesc Sort = "price_desc"
	SortNewest    Sort = "newest"
	SortOldest    Sort = "oldest"
	SortName      Sort = "name"
	SortDiscount  Sort = "discount" // biggest discount first (homepage "Biggest discounts" → Show all)
)

// ParseSort converts a URL value into a Sort.
func ParseSort(s string) (Sort, bool) {
	switch Sort(s) {
	case SortDefault, SortRelevance, SortPopular, SortPriceAsc, SortPriceDesc, SortNewest, SortOldest, SortName, SortDiscount:
		return Sort(s), true
	}
	return "", false
}

// PageSize is fixed: the store grid shows 4 cards per row, so 24 is 6 full rows.
const PageSize = 24

// ListFilter describes which products the store asks for.
//
// Several values in one group mean OR ("Steam or GOG"); different groups are
// combined with AND ("(Steam or GOG) and Europe"), which is how the checkbox
// filters on eneba.com behave. An empty group does not filter.
type ListFilter struct {
	// Text is the search query as typed. Empty means "no search".
	Text      string
	Platforms []string // platform slugs
	Regions   []string // region slugs
	Genres    []string // genre slugs
	Types     []string // product types: "game", "dlc"
	// Price bounds in cents, inclusive. A nil pointer means "no bound"; a pointer
	// is used because 0 is itself a meaningful bound.
	PriceMin *int
	PriceMax *int
	Sort     Sort
	Page     int // 1-based
}

// Named is a slug + display name pair (platform, region).
type Named struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ProductCard is one product as shown on a card in a list.
type ProductCard struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	CoverURL string `json:"coverUrl"`
	Platform Named  `json:"platform"`
	Region   Named  `json:"region"`
	// The three price fields are null in JSON when no offer is in stock.
	PriceCents         *int `json:"priceCents"`         // cheapest in-stock offer
	OriginalPriceCents *int `json:"originalPriceCents"` // that offer's reference price
	DiscountPercent    *int `json:"discountPercent"`
	OfferCount         int  `json:"offerCount"` // offers that can be bought right now
}

// ListResult is one page of products plus the size of the whole result and the
// options of the filter sidebar.
type ListResult struct {
	Items    []ProductCard `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Facets   Facets        `json:"facets"`
}

// FacetOption is one checkbox of a filter group: its value, its label and how many
// products the store would list with it ticked.
type FacetOption struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Facets holds the options of every filter group.
type Facets struct {
	Platforms []FacetOption `json:"platforms"`
	Regions   []FacetOption `json:"regions"`
	Genres    []FacetOption `json:"genres"`
	Types     []FacetOption `json:"types"`
}

// typeNames are the display names of the product types (the database stores only
// the short code).
var typeNames = map[string]string{
	"game": "Game",
	"dlc":  "DLC",
}

// DiscountPercent returns how far price is below original, as a whole percentage
// (62 for "-62%"). Integer maths only; the result is rounded to the nearest percent.
func DiscountPercent(priceCents, originalCents int) int {
	if originalCents <= 0 || priceCents >= originalCents {
		return 0
	}
	saved := originalCents - priceCents
	// Adding half of the divisor before an integer division rounds to nearest
	// instead of always rounding down.
	return (saved*100 + originalCents/2) / originalCents
}
