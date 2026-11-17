package catalog

import (
	"reflect"
	"testing"
)

func TestExpandQuery(t *testing.T) {
	synonyms := []Synonym{
		{"gta", "grand theft auto"},
		{"5", "v"},
		{"3", "iii"},
		{"halflife", "half life"},
	}

	tests := []struct {
		query string
		want  []slot
	}{
		{"witcher", []slot{{typed: []string{"witcher"}}}},
		{"gta 5", []slot{
			{typed: []string{"gta"}, alternatives: []string{"grand theft auto"}},
			{typed: []string{"5"}, alternatives: []string{"v"}},
		}},
		// The pair works backwards too, and the longest phrase wins over "grand" alone.
		{"Grand Theft Auto V", []slot{
			{typed: []string{"grand", "theft", "auto"}, alternatives: []string{"gta"}},
			{typed: []string{"v"}, alternatives: []string{"5"}},
		}},
		{"half life 3", []slot{
			{typed: []string{"half", "life"}, alternatives: []string{"halflife"}},
			{typed: []string{"3"}, alternatives: []string{"iii"}},
		}},
		// Only whole words are replaced: "gtav" is not "gta" + "v".
		{"gtav", []slot{{typed: []string{"gtav"}}}},
	}
	for _, tt := range tests {
		if got := expandQuery(NormalizeQuery(tt.query), synonyms); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("expandQuery(%q) = %+v\n  want %+v", tt.query, got, tt.want)
		}
	}
}
