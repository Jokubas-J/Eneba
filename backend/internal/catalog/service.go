package catalog

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

// ErrInvalidFilter is returned for a filter that cannot be right whatever is in the
// database. The HTTP layer turns it into a 400 response.
var ErrInvalidFilter = errors.New("invalid filter")

const (
	// maxPage bounds OFFSET. Far beyond any real catalog, it only stops a request
	// such as ?page=999999999 from making the database skip billions of rows.
	maxPage = 10_000

	// SuggestLimit is how many product hits the header search dropdown shows, and
	// QuerySuggestLimit how many query suggestions are listed above them.
	SuggestLimit      = 6
	QuerySuggestLimit = 5

	// Rules that keep visitor searches from being abused as a way to show text to
	// everyone (see docs/search.md, "Query suggestions"):
	//   - only searches that returned products are recorded;
	//   - only short queries are recorded (a suggestion is a few words, not a message);
	//   - a recorded query is shown to others only after it was searched on this many
	//     different days.
	maxRecordedLength = 40
	maxRecordedWords  = 5
	minVisitorDays    = 2

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

	words := NormalizeQuery(f.Text)
	synonyms, err := s.synonymsFor(ctx, words)
	if err != nil {
		return ListResult{}, err
	}

	items, total, err := s.repo.List(ctx, f, synonyms, PageSize, (f.Page-1)*PageSize)
	if err != nil {
		return ListResult{}, err
	}
	addDiscounts(items)

	facets, err := s.repo.Facets(ctx, f, synonyms)
	if err != nil {
		return ListResult{}, err
	}
	for i := range facets.Types {
		if name, ok := typeNames[facets.Types[i].Slug]; ok {
			facets.Types[i].Name = name
		}
	}

	// A search that found something feeds the suggestions. Only page 1 counts, so
	// paging through one search is one search. A failure here is logged and not
	// returned: the visitor asked for products, and got them.
	if total > 0 && f.Page == 1 && isRecordable(words) {
		if err := s.repo.RecordSearch(ctx, words, total); err != nil {
			log.Printf("record search %q: %v", f.Text, err)
		}
	}

	return ListResult{Items: items, Total: total, Page: f.Page, PageSize: PageSize, Facets: facets}, nil
}

// synonymsFor loads the synonym table, but only when there is a search text: plain
// browsing does not need it and saves the database round trip.
func (s *Service) synonymsFor(ctx context.Context, words []string) ([]Synonym, error) {
	if len(words) == 0 {
		return nil, nil
	}
	return s.repo.Synonyms(ctx)
}

// SuggestResult is the content of the search dropdown.
type SuggestResult struct {
	Suggestions []string      `json:"suggestions"` // queries, shown on top
	Items       []ProductCard `json:"items"`       // best product matches
	Total       int           `json:"total"`       // all matches, for "Show all N results"
}

// Suggest returns the content of the search dropdown for a partly typed query: query
// suggestions and the best few products.
//
// Unlike List it records nothing: it runs on every pause in typing, so "wit",
// "witc" and "witch" would all be counted as searches.
func (s *Service) Suggest(ctx context.Context, text string) (SuggestResult, error) {
	empty := SuggestResult{Suggestions: []string{}, Items: []ProductCard{}}

	// Decided on the simplified query, so "  !! " or "a" never reaches the database.
	words := NormalizeQuery(text)
	if len(words) == 0 || len(strings.Join(words, " ")) < minSuggestLength {
		return empty, nil
	}

	suggestions, err := s.repo.Suggestions(ctx, words, QuerySuggestLimit)
	if err != nil {
		return empty, err
	}
	synonyms, err := s.repo.Synonyms(ctx)
	if err != nil {
		return empty, err
	}
	items, total, err := s.repo.List(ctx, ListFilter{Text: text, Sort: SortRelevance}, synonyms, SuggestLimit, 0)
	if err != nil {
		return empty, err
	}
	addDiscounts(items)
	return SuggestResult{Suggestions: suggestions, Items: items, Total: total}, nil
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

// CuratedSuggestions lists the curated search queries (used by the test that checks
// each of them finds products).
func (s *Service) CuratedSuggestions(ctx context.Context) ([]string, error) {
	return s.repo.CuratedSuggestions(ctx)
}

// isRecordable reports whether a search query may be recorded as a suggestion.
func isRecordable(words []string) bool {
	return len(words) > 0 && len(words) <= maxRecordedWords && len(strings.Join(words, " ")) <= maxRecordedLength
}
