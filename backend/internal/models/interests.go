package models

import "strings"

// ParseInterests keeps legacy comma/semicolon-separated text usable as a list.
// New writes use one interest per line; no database migration is needed.
func ParseInterests(text string) []string {
	items := make([]string, 0)
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	}) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		duplicate := false
		for _, item := range items {
			if strings.EqualFold(item, part) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			items = append(items, part)
		}
	}
	return items
}
