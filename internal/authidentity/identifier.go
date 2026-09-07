package authidentity

import "regexp"

var boundedIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// IsBoundedIdentifier reports whether an identity or organization identifier
// is safe to carry through account and authorization boundaries.
func IsBoundedIdentifier(value string) bool {
	return boundedIdentifier.MatchString(value)
}
