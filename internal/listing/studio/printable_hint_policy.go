package studio

import "fmt"

// BuildPrintableHint formats the Studio prompt constraint for a requested
// print area, returning no hint when either dimension is not usable.
func BuildPrintableHint(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf(
		"Mandatory print size requirement: target print area: %d by %d pixels. Preserve this exact %d:%d aspect ratio. Compose the artwork for this requested print area and do not output a square design unless the requested print area is square.",
		width,
		height,
		width,
		height,
	)
}
