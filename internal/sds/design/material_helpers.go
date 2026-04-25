package design

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func materialImageURL(material *UploadedMaterial) string {
	if material == nil {
		return ""
	}
	if material.Material != nil && strings.TrimSpace(material.Material.ImageURL) != "" {
		return strings.TrimSpace(material.Material.ImageURL)
	}
	if material.Material != nil && strings.TrimSpace(material.Material.ImageURLAlt) != "" {
		return strings.TrimSpace(material.Material.ImageURLAlt)
	}
	if material.Image != nil && strings.TrimSpace(material.Image.ImageURL) != "" {
		return strings.TrimSpace(material.Image.ImageURL)
	}
	return ""
}

func materialDimensions(material *UploadedMaterial) (int, int) {
	if material == nil {
		return 1, 1
	}
	width, height := 0, 0
	if material.Image != nil {
		width = material.Image.Width
		height = material.Image.Height
	}
	if material.Material != nil {
		if width <= 0 {
			width = int(material.Material.Width)
		}
		if height <= 0 {
			height = int(material.Material.Height)
		}
	}
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	return width, height
}

func buildFabricJSON(material *UploadedMaterial, layer *DesignLayer, fitLevel float64) (string, error) {
	imageURL := materialDesignURL(material)
	if strings.TrimSpace(imageURL) == "" {
		return "", fmt.Errorf("material image url is empty")
	}

	sourceWidth := int(material.Material.Width)
	sourceHeight := int(material.Material.Height)
	if sourceWidth <= 0 {
		sourceWidth = material.Image.Width
	}
	if sourceHeight <= 0 {
		sourceHeight = material.Image.Height
	}
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return "", fmt.Errorf("material dimensions are invalid")
	}

	printWidth := float64(layerPrintWidth(*layer))
	printHeight := float64(layerPrintHeight(*layer))
	scale := minFloat(printWidth/float64(sourceWidth), printHeight/float64(sourceHeight))
	scale *= fitLevel

	doc := map[string]any{
		"version":         "5.2.1",
		"centeredScaling": false,
		"objects": []map[string]any{
			{
				"type":                     "image",
				"version":                  "5.2.1",
				"originX":                  "center",
				"originY":                  "center",
				"left":                     300,
				"top":                      300,
				"width":                    sourceWidth,
				"height":                   sourceHeight,
				"fill":                     "rgb(0,0,0)",
				"stroke":                   nil,
				"strokeWidth":              0,
				"strokeDashArray":          nil,
				"strokeLineCap":            "butt",
				"strokeDashOffset":         0,
				"strokeLineJoin":           "miter",
				"strokeUniform":            false,
				"strokeMiterLimit":         4,
				"scaleX":                   scale,
				"scaleY":                   scale,
				"angle":                    0,
				"flipX":                    false,
				"flipY":                    false,
				"opacity":                  1,
				"shadow":                   nil,
				"visible":                  true,
				"backgroundColor":          "",
				"fillRule":                 "nonzero",
				"paintFirst":               "fill",
				"globalCompositeOperation": "source-over",
				"skewX":                    0,
				"skewY":                    0,
				"cropX":                    0,
				"cropY":                    0,
				"sds": map[string]any{
					"originUrl": imageURL,
					"styleKey":  "",
				},
				"selectable":      true,
				"centeredScaling": false,
				"src":             imageURL,
				"crossOrigin":     "anonymous",
				"filters":         []any{},
			},
		},
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func materialDesignURL(material *UploadedMaterial) string {
	imageURL := materialImageURL(material)
	if imageURL == "" || material == nil || material.Material == nil || material.Material.ID <= 0 {
		return imageURL
	}
	if strings.Contains(imageURL, "material_id=") {
		return imageURL
	}
	separator := "?"
	if strings.Contains(imageURL, "?") {
		separator = "&"
	}
	return imageURL + separator + "material_id=" + strconv.FormatInt(material.Material.ID, 10)
}

func materialContentPath(material *UploadedMaterial) string {
	imageURL := materialImageURL(material)
	if imageURL == "" {
		return ""
	}
	parsed, err := url.Parse(imageURL)
	if err != nil {
		return ""
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	for _, prefix := range []string{"images1000Thumbs/", "imagesThumbs/", "officeImgs1000Thumbs/", "images/"} {
		if strings.HasPrefix(path, prefix) {
			path = strings.TrimPrefix(path, prefix)
			break
		}
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	return strings.TrimSpace(path)
}

func layerPrintWidth(layer DesignLayer) int {
	if layer.PrintWidth > 0 {
		return int(layer.PrintWidth)
	}
	if layer.PrintWidthAlt > 0 {
		return int(layer.PrintWidthAlt)
	}
	return int(layer.Width)
}

func layerPrintHeight(layer DesignLayer) int {
	if layer.PrintHeight > 0 {
		return int(layer.PrintHeight)
	}
	if layer.PrintHeightAlt > 0 {
		return int(layer.PrintHeightAlt)
	}
	return int(layer.Height)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
