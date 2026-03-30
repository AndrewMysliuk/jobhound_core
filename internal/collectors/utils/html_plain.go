package utils

import (
	"html"
	"regexp"
)

var htmlTagRE = regexp.MustCompile(`(?s)<[^>]*>`)

// StripHTMLToPlainText turns board HTML snippets into a single plain line (005 collectors).
func StripHTMLToPlainText(s string) string {
	s = htmlTagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return NormalizePlainText(s)
}
