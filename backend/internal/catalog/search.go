package catalog

import (
	"fmt"
	"strings"
)

// How the product search decides what matches and what comes first.
//
// The query is cut into words. A product matches when EVERY word is found in its
// simplified name (products.search_name), where "found" means either
//
//   - the word is the beginning of a word of the name ("witc" -> "witcher"), or
//   - the word is similar enough to a word of the name ("witchr" -> "witcher"),
//     measured with pg_trgm's word similarity (see below).
//
// In addition, a single short word may match as an abbreviation: it is compared
// with the first letters of the name's words ("gta" -> Grand Theft Auto).
//
// Requiring every word (AND) is what makes "witchr 3" return The Witcher 3 and not
// every game with a 3 in its name.
//
// Trigram similarity in one paragraph: pg_trgm cuts text into overlapping groups of
// three characters ("witcher" -> wit, itc, tch, che, her, plus padded ends). Two
// words that share most of their trigrams are similar. A missing or swapped letter
// changes only the few trigrams around it, so a typo still scores high.
// word_similarity(word, name) compares the word with the best-matching stretch of
// the name instead of the whole name; our names are long ("... (PC) Steam Key
// GLOBAL"), and plain similarity() would be dragged down by all the text that the
// visitor never typed.
const (
	// A typed word counts as "similar enough" from this score. Chosen from the
	// measured scores in docs/search.md: real one-letter typos score 0.56-0.71,
	// unrelated words mostly 0.50 or less.
	similarityThreshold = "0.55"

	// Words shorter than this have too few trigrams for similarity to mean
	// anything, so they must match as a prefix ("3", "v", "of").
	minFuzzyLength = 3

	// An abbreviation is one word of 2-6 characters.
	minAcronymLength = 2
	maxAcronymLength = 6

	// Limits that keep a pasted paragraph from becoming a huge query.
	maxQueryLength = 100
	maxQueryWords  = 8

	// Score of an abbreviation match. Above any typo match (those are below 1.0 x
	// their similarity), below an exact word match (1.0).
	acronymScore = "0.8"
)

// NormalizeQuery simplifies a search query with the same rules the database applies
// to product names (SQL function normalize_search): lower case, apostrophes removed,
// everything that is not a-z or 0-9 turned into a space. It returns the words.
//
// Because only a-z and 0-9 survive, the words can never contain the LIKE wildcards
// % and _, so they are safe to use inside LIKE patterns.
func NormalizeQuery(text string) []string {
	// Cut by runes, not bytes: slicing a string by bytes could split a multi-byte
	// character in half.
	if runes := []rune(text); len(runes) > maxQueryLength {
		text = string(runes[:maxQueryLength])
	}

	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == '\'' || r == '’':
			// dropped: "baldur's" becomes "baldurs"
		default:
			b.WriteByte(' ')
		}
	}
	words := strings.Fields(b.String())
	if len(words) > maxQueryWords {
		words = words[:maxQueryWords]
	}
	return words
}

// wordPrefix is the SQL test "this query word begins the name or one of its words".
//
// arg registers a value as a query argument and returns its placeholder ("$3");
// the words themselves never become part of the SQL text.
func wordPrefix(word string, arg func(any) string) string {
	return fmt.Sprintf("(search_name LIKE %s OR search_name LIKE %s)", arg(word+"%"), arg("% "+word+"%"))
}

// isAcronymQuery reports whether the query may be an abbreviation: one short word.
func isAcronymQuery(words []string) bool {
	return len(words) == 1 && len(words[0]) >= minAcronymLength && len(words[0]) <= maxAcronymLength
}

// searchCondition builds the WHERE condition that selects the matching products.
func searchCondition(words []string, arg func(any) string) string {
	var matches []string
	for _, word := range words {
		match := wordPrefix(word, arg)
		if len(word) >= minFuzzyLength {
			// "<%" is pg_trgm's "is word-similar" operator. It is true when
			// word_similarity(left, right) reaches pg_trgm.word_similarity_threshold,
			// which the repository sets to similarityThreshold for the transaction.
			// The operator form (rather than "word_similarity(...) >= 0.55") is what
			// lets PostgreSQL answer it from the trigram index.
			match = fmt.Sprintf("(%s OR %s <%% search_name)", match, arg(word))
		}
		matches = append(matches, match)
	}
	condition := "(" + strings.Join(matches, " AND ") + ")"

	if isAcronymQuery(words) {
		condition = fmt.Sprintf("(%s OR search_initials LIKE %s)", condition, arg(words[0]+"%"))
	}
	return condition
}

// searchScore builds the expression that ranks the matches (higher = better).
// It is a separate function from searchCondition because only the query that sorts
// needs it; the count query does not, and PostgreSQL rejects unused arguments.
func searchScore(words []string, arg func(any) string) string {
	var wordScores []string
	for _, word := range words {
		// An exact prefix is a full point; a typo is worth its similarity.
		wordScores = append(wordScores, fmt.Sprintf("(CASE WHEN %s THEN 1.0 ELSE word_similarity(%s, search_name) END)",
			wordPrefix(word, arg), arg(word)))
	}
	// Average over the words, so a three-word query is not automatically "better"
	// than a one-word query.
	score := fmt.Sprintf("(%s) / %d.0", strings.Join(wordScores, " + "), len(words))

	// Bonus when the words appear together and in the typed order: "dark souls"
	// should put Dark Souls above a game that merely contains both words. Starting
	// the name is worth more than appearing in the middle of it.
	phrase := strings.Join(words, " ")
	score = fmt.Sprintf("(%s + CASE WHEN search_name LIKE %s THEN 0.5 WHEN search_name LIKE %s THEN 0.25 ELSE 0 END)",
		score, arg(phrase+"%"), arg("% "+phrase+"%"))

	if isAcronymQuery(words) {
		score = fmt.Sprintf("GREATEST(%s, CASE WHEN search_initials LIKE %s THEN %s ELSE 0 END)",
			score, arg(words[0]+"%"), acronymScore)
	}
	return score
}
