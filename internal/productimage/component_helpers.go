package productimage

import (
	"strings"

	"task-processor/internal/pkg/imagex"
)

func extensionForFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "jpg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	case "webp":
		return "webp"
	default:
		return "jpg"
	}
}

func imagexFormat(format string) imagex.Format {
	switch strings.ToLower(format) {
	case "png":
		return imagex.FormatPNG
	default:
		return imagex.FormatJPEG
	}
}
