package membership

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

// Fold is stateless and concurrency-safe; do not replace it with lowercasing,
// which misses equivalent Unicode case forms such as Greek sigma.
var directorySearchFold = cases.Fold()

func (p PageRequest) Normalize() (PageRequest, error) {
	if p.Limit < 1 || p.Limit > 100 || p.Offset < 0 || p.Offset > 10000 ||
		len(p.Filter.Search) > 200 || !utf8.ValidString(p.Filter.Search) || strings.ContainsFunc(p.Filter.Search, unicode.IsControl) {
		return PageRequest{}, ErrInvalidRequest
	}
	switch p.Filter.Role {
	case "", "listingkit_admin":
	default:
		if !regexp.MustCompile(`^sumi_role_[a-f0-9]{32}_(0[1-9]|[1-5][0-9]|6[0-4])$`).MatchString(p.Filter.Role) {
			return PageRequest{}, ErrInvalidRequest
		}
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
	if f.Search == "" {
		return true
	}
	q := directorySearchFold.String(f.Search)
	return strings.Contains(directorySearchFold.String(member.DisplayName), q) || strings.Contains(directorySearchFold.String(member.LoginName), q)
}
