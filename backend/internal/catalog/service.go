package catalog

import (
	"context"
	"errors"
	"fmt"
)

// ErrInvalidFilter is returned for a filter that cannot be right whatever is in the
// database. The HTTP layer turns it into a 400 response.
var ErrInvalidFilter = errors.New("invalid filter")

// maxPage bounds OFFSET. Far beyond any real catalog, it only stops a request such
// as ?page=999999999 from making the database skip billions of rows.
const maxPage = 10_000

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
	if f.Sort == "" {
		f.Sort = SortPopular
	}
	if err := validate(f); err != nil {
		return ListResult{}, err
	}

	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}

	// The discount is derived here rather than stored: it can never disagree with
	// the two prices it comes from.
	for i := range items {
		// &items[i] takes the element itself; "for _, p := range" would hand out a
		// copy and the assignment below would be lost.
		p := &items[i]
		if p.PriceCents != nil && p.OriginalPriceCents != nil {
			discount := DiscountPercent(*p.PriceCents, *p.OriginalPriceCents)
			p.DiscountPercent = &discount
		}
	}

	return ListResult{Items: items, Total: total, Page: f.Page, PageSize: PageSize}, nil
}

func validate(f ListFilter) error {
	if _, ok := orderBy[f.Sort]; !ok {
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
