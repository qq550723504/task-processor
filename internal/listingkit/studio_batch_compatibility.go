package listingkit

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
)

func buildStudioBatchCompatibilityFingerprint(selection SheinStudioSelection) string {
	normalized := strings.Join([]string{
		int64String(selection.ParentProductID),
		int64String(selection.PrototypeGroupID),
		strings.TrimSpace(selection.LayerID),
		strings.TrimSpace(selection.DesignType),
		strconv.Itoa(selection.PrintableWidth),
		strconv.Itoa(selection.PrintableHeight),
		strings.TrimSpace(selection.TemplateImageURL),
		strings.TrimSpace(selection.MaskImageURL),
		strings.TrimSpace(selection.ProductSize),
		strings.TrimSpace(selection.PackagingSpecification),
	}, "|")
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func studioBatchCompatibilityFingerprintComplete(selection SheinStudioSelection) bool {
	return selection.ParentProductID > 0 &&
		selection.PrototypeGroupID > 0 &&
		strings.TrimSpace(selection.LayerID) != "" &&
		strings.TrimSpace(selection.DesignType) != "" &&
		selection.PrintableWidth > 0 &&
		selection.PrintableHeight > 0 &&
		strings.TrimSpace(selection.TemplateImageURL) != "" &&
		strings.TrimSpace(selection.MaskImageURL) != ""
}

func int64String(v int64) string {
	return strconv.FormatInt(v, 10)
}
