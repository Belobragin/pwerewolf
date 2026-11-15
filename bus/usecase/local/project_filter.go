package local

import "strings"

// likePattern turns the wildcards of a filter value into a LIKE pattern: * is any text, ? is any single character.
// The characters that are special in LIKE are escaped, so they match themselves.
func likePattern(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '*':
			b.WriteByte('%')
		case '?':
			b.WriteByte('_')
		case '%', '_', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
