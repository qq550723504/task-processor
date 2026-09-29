package membership

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (p PageRequest) Normalize() (PageRequest, error) {
	if p.Limit < 1 || p.Limit > 100 || p.Offset < 0 || p.Offset > 10000 ||
		len(p.Filter.Search) > 200 || !utf8.ValidString(p.Filter.Search) || strings.ContainsFunc(p.Filter.Search, unicode.IsControl) {
		return PageRequest{}, ErrInvalidRequest
	}
	switch p.Filter.Role {
	case "", "listingkit_viewer", "listingkit_operator", "listingkit_admin":
	default:
		return PageRequest{}, ErrInvalidRequest
	}
	if p.Filter.State != "" && p.Filter.State != "active" && p.Filter.State != "inactive" {
		return PageRequest{}, ErrInvalidRequest
	}
	p.Filter.Search = strings.TrimSpace(p.Filter.Search)
	return p, nil
}

// Matches consumes the current directory projection, never a contact profile or
// an inferred role. Search is literal Unicode substring matching, without SQL
// wildcards, regular expressions or accent folding.
func (f ListFilter) Matches(member Member) bool {
	if (f.Role != "" && !slices.Contains(member.Roles, f.Role)) || (f.State != "" && member.State != f.State) {
		return false
	}
	q := strings.ToLower(f.Search)
	return q == "" || strings.Contains(strings.ToLower(member.DisplayName), q) || strings.Contains(strings.ToLower(member.LoginName), q)
}
