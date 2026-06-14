package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

var AutoReviewNotes = sheinmarketplace.AutoReviewNotes

func FilterManualReviewNotes(notes []string) []string {
	return sheinmarketplace.FilterManualReviewNotes(notes)
}

func IsCategoryResolved(pkg *sheinpub.Package) bool {
	return sheinmarketplace.IsCategoryResolved(pkg)
}

func IsAttributeResolved(pkg *sheinpub.Package) bool {
	return sheinmarketplace.IsAttributeResolved(pkg)
}

func IsSaleAttributeResolved(pkg *sheinpub.Package) bool {
	return sheinmarketplace.IsSaleAttributeResolved(pkg)
}
