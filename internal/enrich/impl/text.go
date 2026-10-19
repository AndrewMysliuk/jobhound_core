package impl

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// VacancyMatchText is the normalized corpus for quote checks and hashing:
// NFC title, description, raw location, and raw salary joined by newlines,
// then newlines to spaces, collapsed whitespace, and trim.
// Collector country codes and the URL are not included.
func VacancyMatchText(title, description, locationRaw, salaryRaw string) string {
	joined := strings.Join([]string{
		norm.NFC.String(title),
		norm.NFC.String(description),
		norm.NFC.String(locationRaw),
		norm.NFC.String(salaryRaw),
	}, "\n")
	return normalizeMatchText(joined)
}

// InputHash returns the hex SHA-256 of VacancyMatchText for the four text fields.
func InputHash(title, description, locationRaw, salaryRaw string) string {
	sum := sha256.Sum256([]byte(VacancyMatchText(title, description, locationRaw, salaryRaw)))
	return hex.EncodeToString(sum[:])
}

// QuoteMatches reports whether quote is a case-sensitive substring of
// vacancyMatchText after the same NFC / whitespace normalization.
// An empty quote after normalization fails closed.
func QuoteMatches(quote, vacancyMatchText string) bool {
	q := normalizeMatchText(quote)
	if q == "" {
		return false
	}
	return strings.Contains(vacancyMatchText, q)
}

func normalizeMatchText(s string) string {
	s = norm.NFC.String(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}
