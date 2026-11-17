package catalog

import (
	"fmt"
	"strings"
	"unicode"

	// The Go team's text package (golang.org/x/... is maintained alongside the
	// standard library). Already in the module graph through goose; used here for
	// Unicode normalization, which the standard library does not include.
	"golang.org/x/text/unicode/norm"
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
// with the first letters of the name's words ("rdr2" -> Red Dead Redemption 2).
//
// Before matching, the query is expanded with synonyms from the search_synonyms
// table: a word or phrase of the query may also be matched by its synonym, which
// must then appear in the name as whole words. "gta 5" becomes
// ("gta" or "grand theft auto") and ("5" or "v"), which finds Grand Theft Auto V.
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

	// A 3-4 letter word has so few trigrams that one wrong letter drops it to about
	// 0.50 ("evl" against "evil"). In a query of two or more words the other words
	// already keep the results on topic, so short words get this lower bar there.
	// Alone, "dak" at 0.50 would match DayZ, Darkest Dungeon and Dragon's Dogma.
	shortWordMaxLength = 4
	shortWordThreshold = "0.5"

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
// to product names (SQL function normalize_search): accents removed, lower case,
// apostrophes removed, everything that is not a-z or 0-9 turned into a space. It
// returns the words.
//
// Because only a-z and 0-9 survive, the words can never contain the LIKE wildcards
// % and _ or regular-expression syntax, so they are safe inside those patterns.
func NormalizeQuery(text string) []string {
	// Cut by runes, not bytes: slicing a string by bytes could split a multi-byte
	// character in half.
	if runes := []rune(text); len(runes) > maxQueryLength {
		text = string(runes[:maxQueryLength])
	}

	var b strings.Builder
	for _, r := range strings.ToLower(removeAccents(text)) {
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

// specialLetters are letters that are not "a plain letter plus an accent mark" in
// Unicode, so decomposition (below) leaves them alone. PostgreSQL's unaccent
// dictionary replaces them like this, and so do we.
var specialLetters = strings.NewReplacer(
	"ß", "ss", "æ", "ae", "Æ", "AE", "œ", "oe", "Œ", "OE", "ø", "o", "Ø", "O",
	"ł", "l", "Ł", "L", "đ", "d", "Đ", "D", "ð", "d", "Ð", "D", "þ", "th", "Þ", "TH",
	"ı", "i", "ﬁ", "fi", "ﬂ", "fl",
)

// removeAccents is the Go counterpart of PostgreSQL's unaccent: "Ragnarök" ->
// "Ragnarok", "Pokémon" -> "Pokemon".
//
// Unicode can write "ö" as one character or as "o" followed by a combining
// diaeresis mark. NFD ("canonical decomposition") converts every such letter into
// the second form; the marks (category Mn, "nonspacing mark") are then dropped.
// It is done before lower-casing, because lower-casing "İ" (Turkish capital I with
// a dot) produces "i" plus a mark, which would otherwise survive.
func removeAccents(text string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(specialLetters.Replace(text)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Synonym is one pair of interchangeable phrases from the search_synonyms table,
// both already simplified like a query ("gta" and "grand theft auto").
type Synonym struct {
	Term    string
	Synonym string
}

// slot is one part of the query that must match: the words as typed, plus phrases
// that may stand in for them.
type slot struct {
	typed        []string // matched loosely: prefix or typo-tolerant, word by word
	alternatives []string // synonyms: must appear in the name as whole words
}

// expandQuery splits the query words into slots and attaches synonyms.
//
// It walks the words from left to right and, at each position, looks for the
// LONGEST run of words that has a synonym ("grand theft auto" before "grand"), so a
// phrase is replaced as a whole. Words without synonyms become a slot of their own.
// Synonyms work in both directions: the table stores each pair once.
func expandQuery(words []string, synonyms []Synonym) []slot {
	partners := map[string][]string{}
	longest := 1
	for _, pair := range synonyms {
		partners[pair.Term] = append(partners[pair.Term], pair.Synonym)
		partners[pair.Synonym] = append(partners[pair.Synonym], pair.Term)
		longest = max(longest, len(strings.Fields(pair.Term)), len(strings.Fields(pair.Synonym)))
	}

	var slots []slot
	for i := 0; i < len(words); {
		matched := false
		for n := min(longest, len(words)-i); n >= 1; n-- {
			if alternatives, ok := partners[strings.Join(words[i:i+n], " ")]; ok {
				slots = append(slots, slot{typed: words[i : i+n], alternatives: alternatives})
				i += n
				matched = true
				break
			}
		}
		if !matched {
			slots = append(slots, slot{typed: words[i : i+1]})
			i++
		}
	}
	return slots
}

// wordPrefix is the SQL test "this query word begins the name or one of its words".
//
// arg registers a value as a query argument and returns its placeholder ("$3");
// the words themselves never become part of the SQL text.
func wordPrefix(word string, arg func(any) string) string {
	return fmt.Sprintf("(search_name LIKE %s OR search_name LIKE %s)", arg(word+"%"), arg("% "+word+"%"))
}

// wholePhrase is the SQL test "this phrase appears in the name as whole words".
// \m and \M are PostgreSQL's regular-expression marks for the start and end of a
// word, so "v" matches "grand theft auto v enhanced" but not "valley".
func wholePhrase(phrase string, arg func(any) string) string {
	return fmt.Sprintf("search_name ~ %s", arg(`\m`+phrase+`\M`))
}

// wordMatch is the condition for one typed word of a query of queryWords words.
func wordMatch(word string, queryWords int, arg func(any) string) string {
	match := wordPrefix(word, arg)
	if len(word) < minFuzzyLength {
		return match
	}
	// "<%" is pg_trgm's "is word-similar" operator. It is true when
	// word_similarity(left, right) reaches pg_trgm.word_similarity_threshold, which
	// the repository sets to similarityThreshold for the transaction. The operator
	// form (rather than "word_similarity(...) >= 0.55") is what lets PostgreSQL
	// answer it from the trigram index.
	match = fmt.Sprintf("(%s OR %s <%% search_name)", match, arg(word))
	if queryWords >= 2 && len(word) <= shortWordMaxLength {
		match = fmt.Sprintf("(%s OR word_similarity(%s, search_name) >= %s)", match, arg(word), shortWordThreshold)
	}
	return match
}

// isAcronymQuery reports whether the query may be an abbreviation: one short word.
func isAcronymQuery(words []string) bool {
	return len(words) == 1 && len(words[0]) >= minAcronymLength && len(words[0]) <= maxAcronymLength
}

// searchCondition builds the WHERE condition that selects the matching products.
func searchCondition(words []string, synonyms []Synonym, arg func(any) string) string {
	var slotMatches []string
	for _, s := range expandQuery(words, synonyms) {
		var typed []string
		for _, word := range s.typed {
			typed = append(typed, wordMatch(word, len(words), arg))
		}
		options := []string{"(" + strings.Join(typed, " AND ") + ")"}
		for _, phrase := range s.alternatives {
			options = append(options, wholePhrase(phrase, arg))
		}
		slotMatches = append(slotMatches, "("+strings.Join(options, " OR ")+")")
	}
	condition := "(" + strings.Join(slotMatches, " AND ") + ")"

	if isAcronymQuery(words) {
		condition = fmt.Sprintf("(%s OR search_initials LIKE %s)", condition, arg(words[0]+"%"))
	}
	return condition
}

// searchScore builds the expression that ranks the matches (higher = better).
// It is a separate function from searchCondition because only the query that sorts
// needs it; the count query does not, and PostgreSQL rejects unused arguments.
func searchScore(words []string, synonyms []Synonym, arg func(any) string) string {
	slots := expandQuery(words, synonyms)

	var slotScores []string
	for _, s := range slots {
		// Typed words: an exact prefix is a full point, a typo is worth its similarity.
		var typed []string
		for _, word := range s.typed {
			typed = append(typed, fmt.Sprintf("(CASE WHEN %s THEN 1.0 ELSE word_similarity(%s, search_name) END)",
				wordPrefix(word, arg), arg(word)))
		}
		options := []string{fmt.Sprintf("(%s) / %d.0", strings.Join(typed, " + "), len(typed))}
		// A synonym that matches counts as an exact match of the slot.
		for _, phrase := range s.alternatives {
			options = append(options, fmt.Sprintf("(CASE WHEN %s THEN 1.0 ELSE 0 END)", wholePhrase(phrase, arg)))
		}
		slotScores = append(slotScores, "GREATEST("+strings.Join(options, ", ")+")")
	}
	// Average over the slots, so a three-word query is not automatically "better"
	// than a one-word query.
	score := fmt.Sprintf("(%s) / %d.0", strings.Join(slotScores, " + "), len(slots))

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
