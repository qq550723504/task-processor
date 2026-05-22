package design

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func selectLayer(layers []DesignLayer, layerID string) (*DesignLayer, error) {
	if len(layers) == 0 {
		return nil, fmt.Errorf("no design layers available")
	}
	if strings.TrimSpace(layerID) == "" {
		return &layers[0], nil
	}
	for i := range layers {
		if string(layers[i].ID) == layerID {
			return &layers[i], nil
		}
	}
	return nil, fmt.Errorf("layer %s not found", layerID)
}

func collectPSDIDs(psds []PSDDocument) []string {
	ids := make([]string, 0, len(psds))
	for _, psd := range psds {
		if strings.TrimSpace(string(psd.ID)) != "" {
			ids = append(ids, string(psd.ID))
		}
	}
	return ids
}

func buildPreviewImageURLs(psds []PSDDocument, layerName string, material *UploadedMaterial, resizeMode int) []string {
	urls := make([]string, 0, len(psds))
	replaceContent := materialContentPath(material)
	if replaceContent == "" {
		replaceContent = materialImageURL(material)
	}
	imageWidth, imageHeight := materialDimensions(material)
	for _, psd := range psds {
		modelFile := psdModelFile(psd)
		if modelFile == "" {
			continue
		}
		payload := map[string]any{
			"model_file": modelFile,
			"replace_layers_content": []map[string]any{
				{
					"layer_name":      layerName,
					"replace_type":    1,
					"replace_content": replaceContent,
					"image_width":     imageWidth,
					"image_height":    imageHeight,
					"resize_mode":     resizeMode,
					"image_filter":    nil,
				},
			},
			"output_format": "jpg_thumb",
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			continue
		}
		urls = append(urls, "http://e.sdspod.com/builds?content="+url.QueryEscape(string(raw)))
	}
	return urls
}

func buildSaveDesignRequest(result *PrepareSyncDesignResult) SaveDesignRequest {
	if result == nil || result.Page == nil || result.Request == nil {
		return SaveDesignRequest{}
	}

	prototypes := make([]SyncDesignPrototype, 0, len(result.Request.Prototypes))
	for _, prototype := range result.Request.Prototypes {
		layers := make([]SyncDesignLayer, 0, len(prototype.Layers))
		for _, layer := range prototype.Layers {
			if result.Material != nil && result.Material.Material != nil && result.Material.Material.ID > 0 {
				layer.MaterialID = result.Material.Material.ID
				layer.DesignMaterialID = result.Material.Material.ID
			}
			if content := materialContentPath(result.Material); content != "" {
				layer.Content = content
			}
			layers = append(layers, layer)
		}
		prototype.Layers = layers
		prototype.Images = saveDesignImagesForPrototype(result, prototype)
		prototypes = append(prototypes, prototype)
	}

	req := SaveDesignRequest{
		ProductID:        result.Request.ProductID,
		PrototypeGroupID: result.Request.PrototypeGroupID,
		DesignType:       result.Request.DesignType,
		Prototypes:       prototypes,
	}
	return req
}

func saveDesignImagesForPrototype(result *PrepareSyncDesignResult, prototype SyncDesignPrototype) []string {
	if result == nil {
		return nil
	}
	for _, productID := range prototype.ProductIDs {
		if result.RelatedPages != nil {
			if page := result.RelatedPages[productID]; page != nil {
				if urls := psdThumbnailURLs(page.PSDs); len(urls) > 0 {
					return urls
				}
			}
		}
		if result.Page != nil && result.Page.Product.ID == productID {
			if urls := psdThumbnailURLs(result.Page.PSDs); len(urls) > 0 {
				return urls
			}
		}
	}
	if result.Page != nil {
		if urls := psdThumbnailURLs(result.Page.PSDs); len(urls) > 0 {
			return urls
		}
	}
	return append([]string(nil), prototype.Images...)
}

func psdThumbnailURLs(psds []PSDDocument) []string {
	urls := make([]string, 0, len(psds))
	for _, psd := range psds {
		if strings.TrimSpace(psd.ThumbnailURL) != "" {
			urls = append(urls, strings.TrimSpace(psd.ThumbnailURL))
		}
	}
	return urls
}

func psdModelFile(psd PSDDocument) string {
	fileURL := strings.TrimSpace(psd.FileURL)
	if fileURL != "" {
		if parsed, err := url.Parse(fileURL); err == nil {
			path := strings.TrimPrefix(parsed.EscapedPath(), "/")
			path = strings.TrimPrefix(path, "psds/")
			if decoded, err := url.PathUnescape(path); err == nil {
				path = decoded
			}
			if path != "" {
				return path
			}
		}
	}
	return strings.TrimSpace(psd.FileCode)
}
