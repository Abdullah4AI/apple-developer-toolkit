package search

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// Preserve the original matcher as a differential oracle for fixed-size forms.
func legacyExactTokenContains(tokens []string, term string) bool {
	termForms := legacySearchTokenForms(strings.ToLower(strings.TrimSpace(term)))
	for _, token := range tokens {
		if legacySearchTokenFormsOverlap(legacySearchTokenForms(strings.ToLower(strings.TrimSpace(token))), termForms) {
			return true
		}
	}
	return false
}

func legacySearchTokensMatch(token, term string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	term = strings.ToLower(strings.TrimSpace(term))
	if token == "" || term == "" {
		return false
	}
	if token == term {
		return true
	}

	termForms := legacySearchTokenForms(term)
	tokenParts := strings.Split(token, "-")
	for _, tokenPart := range tokenParts {
		if legacySearchTokenFormsOverlap(legacySearchTokenForms(tokenPart), termForms) {
			return true
		}
	}
	return false
}

func legacySameSearchStem(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	return left != "" && right != "" && legacySearchTokenFormsOverlap(legacySearchTokenForms(left), legacySearchTokenForms(right))
}

func legacySearchTokenForms(token string) []string {
	forms := []string{token}
	if len(token) > 4 && strings.HasSuffix(token, "ies") {
		forms = append(forms, strings.TrimSuffix(token, "ies")+"y")
		return uniqueStrings(forms)
	}
	if len(token) > 3 && strings.HasSuffix(token, "es") {
		forms = append(forms, strings.TrimSuffix(token, "es"))
	}
	if len(token) > 3 && strings.HasSuffix(token, "s") &&
		!strings.HasSuffix(token, "ss") &&
		!strings.HasSuffix(token, "us") &&
		!strings.HasSuffix(token, "is") {
		forms = append(forms, strings.TrimSuffix(token, "s"))
	}
	return uniqueStrings(forms)
}

func legacySearchTokenFormsOverlap(left, right []string) bool {
	for _, leftForm := range left {
		for _, rightForm := range right {
			if leftForm == rightForm {
				return true
			}
		}
	}
	return false
}

func TestSearchMatcherFixedFormsPreservesSemantics(t *testing.T) {
	tokens := []string{"", " ", "ies", "cities", "CITY", "CITIES", " cities ", " city", "releases", "status", "class", "analysis", "build-localizations", "-list--all-", "É", "é", "İ", "東京", "東京s", "\x00", " apps\n"}
	r := rand.New(rand.NewPCG(13, 89))
	alphabet := []rune("ab-Sesiy é東京")
	for range 100 {
		var b strings.Builder
		for range r.IntN(16) {
			b.WriteRune(alphabet[r.IntN(len(alphabet))])
		}
		tokens = append(tokens, b.String())
	}
	for _, term := range tokens {
		if got, want := exactTokenContains(tokens, term), legacyExactTokenContains(tokens, term); got != want {
			t.Fatalf("exactTokenContains(%q)=%v, original=%v", term, got, want)
		}
		for _, token := range tokens {
			if got, want := searchTokensMatch(token, term), legacySearchTokensMatch(token, term); got != want {
				t.Fatalf("match(%q,%q)=%v, original=%v", token, term, got, want)
			}
			if got, want := sameSearchStem(token, term), legacySameSearchStem(token, term); got != want {
				t.Fatalf("stem(%q,%q)=%v, original=%v", token, term, got, want)
			}
			if got, want := searchTokenFormsOverlap(searchTokenForms(token), searchTokenForms(term)), legacySearchTokenFormsOverlap(legacySearchTokenForms(token), legacySearchTokenForms(term)); got != want {
				t.Fatalf("forms overlap(%q,%q)=%v, original=%v", token, term, got, want)
			}
		}
	}
}
