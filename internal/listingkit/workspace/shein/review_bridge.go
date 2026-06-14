package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

func InspectionReviewReasons(pkg *Package) []string {
	return sheinmarketplace.InspectionReviewReasons(pkg)
}

func CookieUnavailableReviewNotes(pkg *Package) []string {
	return sheinmarketplace.CookieUnavailableReviewNotes(pkg)
}

func StripCookieUnavailableReviewNotes(pkg *Package) {
	sheinmarketplace.StripCookieUnavailableReviewNotes(pkg)
}

func FilterOutCookieUnavailableReviewNotes(notes []string) []string {
	return sheinmarketplace.FilterOutCookieUnavailableReviewNotes(notes)
}

func HasCookieUnavailableReviewNotes(pkg *Package) bool {
	return sheinmarketplace.HasCookieUnavailableReviewNotes(pkg)
}

func IsCookieUnavailableText(value string) bool {
	return sheinmarketplace.IsCookieUnavailableText(value)
}
