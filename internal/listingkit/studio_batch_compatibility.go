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
		intString(selection.PrintableWidth),
		intString(selection.PrintableHeight),
		strings.TrimSpace(selection.TemplateImageURL),
		strings.TrimSpace(selection.MaskImageURL),
	}, "|")
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func int64String(v int64) string {
	return strconv.FormatInt(v, 10)
}

func intString(v int) string {
	return strconv.Itoa(v)
}
