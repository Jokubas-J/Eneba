package catalog

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestDiscountPercent(t *testing.T) {
	tests := []struct {
		price, original, want int
	}{
		{1000, 2000, 50},
		{1557, 4999, 69}, // the example on Eneba's product page: €49.99 -> €15.57 = -69%
		{999, 1000, 0},   // 0.1% rounds down to 0
		{995, 1000, 1},   // 0.5% rounds up to 1
		{1, 1000, 100},   // 99.9% rounds to 100
		{1000, 1000, 0},
		{1200, 1000, 0}, // a price above the reference is not a negative discount
		{500, 0, 0},     // no division by zero
	}
	for _, tt := range tests {
		if got := DiscountPercent(tt.price, tt.original); got != tt.want {
			t.Errorf("DiscountPercent(%d, %d) = %d, want %d", tt.price, tt.original, got, tt.want)
		}
	}
}

func TestParseSort(t *testing.T) {
	valid := map[string]Sort{
		"":           SortPopular,
		"popular":    SortPopular,
		"price_asc":  SortPriceAsc,
		"price_desc": SortPriceDesc,
		"newest":     SortNewest,
		"oldest":     SortOldest,
		"name":       SortName,
	}
	for input, want := range valid {
		got, ok := ParseSort(input)
		if !ok || got != want {
			t.Errorf("ParseSort(%q) = %q, %v; want %q, true", input, got, ok, want)
		}
		// Every value the parser accepts must have an ORDER BY clause.
		if _, exists := orderBy[got]; !exists {
			t.Errorf("sort %q has no ORDER BY clause", got)
		}
	}
	for _, input := range []string{"PRICE_ASC", "price", "id; DROP TABLE products", "popularity DESC"} {
		if _, ok := ParseSort(input); ok {
			t.Errorf("ParseSort(%q) accepted an unknown value", input)
		}
	}
}

func TestBuildWhere(t *testing.T) {
	tests := []struct {
		name      string
		filter    ListFilter
		wantSQL   []string // fragments that must appear, in this order
		wantArgs  []any
		wantEmpty bool
	}{
		{
			name:      "no filter gives no WHERE",
			filter:    ListFilter{},
			wantEmpty: true,
		},
		{
			name:     "one group",
			filter:   ListFilter{Platforms: []string{"steam", "gog"}},
			wantSQL:  []string{"WHERE platform_slug = ANY($1)"},
			wantArgs: []any{[]string{"steam", "gog"}},
		},
		{
			name: "every group, numbered in order",
			filter: ListFilter{
				Platforms: []string{"steam"},
				Regions:   []string{"europe"},
				Types:     []string{"game"},
				Genres:    []string{"rpg"},
				PriceMin:  intPtr(500),
				PriceMax:  intPtr(2000),
			},
			wantSQL: []string{
				"platform_slug = ANY($1)", " AND region_slug = ANY($2)", " AND product_type = ANY($3)",
				"g.slug = ANY($4)", " AND price_cents >= $5", " AND price_cents <= $6",
			},
			wantArgs: []any{[]string{"steam"}, []string{"europe"}, []string{"game"}, []string{"rpg"}, 500, 2000},
		},
		{
			name:     "a zero bound is still a bound",
			filter:   ListFilter{PriceMax: intPtr(0)},
			wantSQL:  []string{"WHERE price_cents <= $1"},
			wantArgs: []any{0},
		},
		{
			name:     "hostile text stays an argument and never reaches the SQL",
			filter:   ListFilter{Platforms: []string{"x'); DROP TABLE products; --"}},
			wantSQL:  []string{"WHERE platform_slug = ANY($1)"},
			wantArgs: []any{[]string{"x'); DROP TABLE products; --"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildWhere(tt.filter)

			if tt.wantEmpty {
				if sql != "" || len(args) != 0 {
					t.Fatalf("buildWhere() = %q, %v; want empty", sql, args)
				}
				return
			}

			position := 0
			for _, fragment := range tt.wantSQL {
				index := strings.Index(sql[position:], fragment)
				if index < 0 {
					t.Fatalf("SQL %q does not contain %q after position %d", sql, fragment, position)
				}
				position += index + len(fragment)
			}
			if strings.Contains(sql, "DROP") {
				t.Errorf("request text leaked into the SQL: %q", sql)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %#v, want %#v", args, tt.wantArgs)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	valid := ListFilter{Sort: SortPopular, Page: 1}

	tests := []struct {
		name    string
		change  func(f *ListFilter)
		wantErr bool
	}{
		{"defaults", func(f *ListFilter) {}, false},
		{"price range", func(f *ListFilter) { f.PriceMin, f.PriceMax = intPtr(100), intPtr(100) }, false},
		{"unknown sort", func(f *ListFilter) { f.Sort = "random" }, true},
		{"page zero", func(f *ListFilter) { f.Page = 0 }, true},
		{"negative page", func(f *ListFilter) { f.Page = -3 }, true},
		{"absurd page", func(f *ListFilter) { f.Page = maxPage + 1 }, true},
		{"negative minimum", func(f *ListFilter) { f.PriceMin = intPtr(-1) }, true},
		{"negative maximum", func(f *ListFilter) { f.PriceMax = intPtr(-1) }, true},
		{"minimum above maximum", func(f *ListFilter) { f.PriceMin, f.PriceMax = intPtr(200), intPtr(100) }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := valid
			tt.change(&filter)

			err := validate(filter)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidFilter) {
				t.Errorf("error %v is not an ErrInvalidFilter", err)
			}
		})
	}
}
