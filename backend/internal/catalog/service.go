package catalog

import (
	"context"
	"errors"
	"fmt"
)

// ErrInvalidFilter is returned for a filter that cannot be right whatever is in the
// database. The HTTP layer turns it into a 400 response.
var ErrInvalidFilter = errors.New("invalid filter")

const (
	// maxPage bounds OFFSET. Far beyond any real catalog, it only stops a request
	// such as ?page=999999999 from making the database skip billions of rows.
	maxPage = 10_000

	// SuggestLimit is how many product hits the header search dropdown shows.
	SuggestLimit = 6

	// A one-letter query matches a large part of the catalog and helps nobody, so
	// suggestions start at two characters.
	minSuggestLength = 2
)

// Service holds the rules of the catalog that are not SQL and not HTTP.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List validates the filter, loads one page and fills in the derived fields.
func (s *Service) List(ctx context.Context, f ListFilter) (ListResult, error) {
	if f.Page == 0 {
		f.Page = 1
	}
	f.Sort = resolveSort(f)
	if err := validate(f); err != nil {
		return ListResult{}, err
	}

	items, total, err := s.repo.List(ctx, f, PageSize, (f.Page-1)*PageSize)
	if err != nil {
		return ListResult{}, err
	}
	addDiscounts(items)

	return ListResult{Items: items, Total: total, Page: f.Page, PageSize: PageSize}, nil
}

// Suggest returns the few best product matches for a partly typed query: the data
// of the search dropdown in the header.
func (s *Service) Suggest(ctx context.Context, text string) ([]ProductCard, error) {
	// Decided on the simplified query, so "  !! " or "a" never reaches the database.
	words := NormalizeQuery(text)
	if len(words) == 0 || (len(words) == 1 && len(words[0]) < minSuggestLength) {
		return []ProductCard{}, nil
	}

	items, _, err := s.repo.List(ctx, ListFilter{Text: text, Sort: SortRelevance}, SuggestLimit, 0)
	if err != nil {
		return nil, err
	}
	addDiscounts(items)
	return items, nil
}

// resolveSort picks the order when the visitor did not choose one: a search is
// shown best match first, plain browsing most popular first.
func resolveSort(f ListFilter) Sort {
	if f.Sort != SortDefault {
		return f.Sort
	}
	if len(NormalizeQuery(f.Text)) > 0 {
		return SortRelevance
	}
	return SortPopular
}

// addDiscounts fills in DiscountPercent. The discount is derived here rather than
// stored: it can never disagree with the two prices it comes from.
func addDiscounts(items []ProductCard) {
	for i := range items {
		// &items[i] takes the element itself; "for _, p := range" would hand out a
		// copy and the assignment below would be lost.
		p := &items[i]
		if p.PriceCents != nil && p.OriginalPriceCents != nil {
			discount := DiscountPercent(*p.PriceCents, *p.OriginalPriceCents)
			p.DiscountPercent = &discount
		}
	}
}

func validate(f ListFilter) error {
	if _, ok := orderBy[f.Sort]; !ok && f.Sort != SortRelevance {
		return fmtInvalid("unknown sort")
	}
	if f.Page < 1 || f.Page > maxPage {
		return fmtInvalid("page out of range")
	}
	if f.PriceMin != nil && *f.PriceMin < 0 {
		return fmtInvalid("price_min must not be negative")
	}
	if f.PriceMax != nil && *f.PriceMax < 0 {
		return fmtInvalid("price_max must not be negative")
	}
	if f.PriceMin != nil && f.PriceMax != nil && *f.PriceMin > *f.PriceMax {
		return fmtInvalid("price_min is greater than price_max")
	}
	return nil
}

// fmtInvalid wraps ErrInvalidFilter with a reason (%w), so callers can test for the
// kind of error with errors.Is and still show the reason to the client.
func fmtInvalid(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidFilter, reason)
}
