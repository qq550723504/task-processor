package design

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	sdstemplate "task-processor/internal/sds/template"
)

func TestMaterialUnmarshalRealFields(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id": 459408618,
		"name": "微信图片_20260422125356_5_41.jpg",
		"file_code": "911e5ba9de3abe515e34cefacf9f38bb.jpg",
		"content_type": "image/jpeg",
		"width": 1080,
		"height": 2340,
		"img_url": "https://cdn.sdspod.com/material.jpg"
	}`)

	var material Material
	if err := json.Unmarshal(payload, &material); err != nil {
		t.Fatalf("unmarshal material: %v", err)
	}

	if material.ID != 459408618 {
		t.Fatalf("unexpected material id: %d", material.ID)
	}
	if material.FileCode != "911e5ba9de3abe515e34cefacf9f38bb.jpg" {
		t.Fatalf("unexpected file code: %s", material.FileCode)
	}
	if material.Width != 1080 || material.Height != 2340 {
		t.Fatalf("unexpected material size: %dx%d", material.Width, material.Height)
	}
}

func TestSyncDesignRequestUnmarshalRealFields(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"product_id": 89764,
		"prototypeGroupId": 14555,
		"merchantProductResultGroupId": 0,
		"designType": "material",
		"prototypes": [
			{
				"prototype_id": "698744758228934657",
				"product_ids": [89764],
				"psd_ids": ["782092292330442752"],
				"layers": [
					{
						"material_id": "",
						"layer_id": "698744758333792256",
						"content": "",
						"img_width": 850,
						"img_height": 1049,
						"resize_mode": 0,
						"fit_level": 1,
						"fabric_json": "{\"version\":\"5.2.1\"}",
						"related_material_ids": [396548287]
					}
				],
				"images": [
					"http://e.sdspod.com/builds?content=..."
				]
			}
		]
	}`)

	var req SyncDesignRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		t.Fatalf("unmarshal sync design request: %v", err)
	}

	if req.ProductID != 89764 {
		t.Fatalf("unexpected product id: %d", req.ProductID)
	}
	if req.PrototypeGroupID != 14555 {
		t.Fatalf("unexpected prototype group id: %d", req.PrototypeGroupID)
	}
	if len(req.Prototypes) != 1 {
		t.Fatalf("unexpected prototypes: %+v", req.Prototypes)
	}

	prototype := req.Prototypes[0]
	if prototype.PrototypeID != "698744758228934657" {
		t.Fatalf("unexpected prototype id: %s", prototype.PrototypeID)
	}
	if len(prototype.Layers) != 1 {
		t.Fatalf("unexpected layers: %+v", prototype.Layers)
	}
	if prototype.Layers[0].LayerID != "698744758333792256" {
		t.Fatalf("unexpected layer id: %s", prototype.Layers[0].LayerID)
	}
	if len(prototype.Layers[0].RelatedMaterialIDs) != 1 || prototype.Layers[0].RelatedMaterialIDs[0] != 396548287 {
		t.Fatalf("unexpected related material ids: %+v", prototype.Layers[0].RelatedMaterialIDs)
	}
}

func TestBuildPreviewImageURLs(t *testing.T) {
	t.Parallel()

	material := &UploadedMaterial{
		Image:    &UploadedImage{ImageURL: "https://cdn.sdspod.com/upload.png", Width: 1200, Height: 1800},
		Material: &Material{ID: 1, ImageURL: "https://cdn.sdspod.com/imagesThumbs/tenant-a/material.png", Width: 1080, Height: 1620},
	}
	urls := buildPreviewImageURLs([]PSDDocument{
		{ID: "1", FileCode: "abc.psd", FileURL: "https://cdn.sdspod.com/psds/tenant-a/abc.psd"},
		{ID: "2", FileCode: "def.psd"},
	}, "素材", material, 0)

	if len(urls) != 2 {
		t.Fatalf("unexpected preview url count: %d", len(urls))
	}
	if !strings.Contains(urls[0], "e.sdspod.com/builds?content=") {
		t.Fatalf("unexpected preview url: %s", urls[0])
	}

	parsed, err := url.Parse(urls[0])
	if err != nil {
		t.Fatalf("parse preview url: %v", err)
	}
	content := parsed.Query().Get("content")
	if content == "" {
		t.Fatalf("missing content query: %s", urls[0])
	}

	var payload struct {
		ModelFile            string `json:"model_file"`
		ReplaceLayersContent []struct {
			LayerName      string `json:"layer_name"`
			ReplaceContent string `json:"replace_content"`
			ImageWidth     int    `json:"image_width"`
			ImageHeight    int    `json:"image_height"`
			ResizeMode     int    `json:"resize_mode"`
		} `json:"replace_layers_content"`
		OutputFormat string `json:"output_format"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("unmarshal preview payload: %v", err)
	}
	if payload.ModelFile != "tenant-a/abc.psd" || payload.OutputFormat != "jpg_thumb" {
		t.Fatalf("unexpected preview payload: %+v", payload)
	}
	if len(payload.ReplaceLayersContent) != 1 {
		t.Fatalf("unexpected replacement layers: %+v", payload.ReplaceLayersContent)
	}
	replacement := payload.ReplaceLayersContent[0]
	if replacement.LayerName != "素材" {
		t.Fatalf("unexpected layer name: %s", replacement.LayerName)
	}
	if replacement.ReplaceContent != "tenant-a/material.png" {
		t.Fatalf("unexpected replace content: %s", replacement.ReplaceContent)
	}
	if replacement.ImageWidth != 1200 || replacement.ImageHeight != 1800 {
		t.Fatalf("unexpected replacement dimensions: %dx%d", replacement.ImageWidth, replacement.ImageHeight)
	}
	if replacement.ResizeMode != 0 {
		t.Fatalf("unexpected resize mode: %d", replacement.ResizeMode)
	}
}

func TestRenderedImageURLsFromProductUsesVariantResultGroups(t *testing.T) {
	t.Parallel()

	product := &sdstemplate.ProductDetail{
		ProductSummary: sdstemplate.ProductSummary{
			ImgURL: "https://cdn.sdspod.com/parent.jpg",
			Subproducts: &sdstemplate.Subproducts{Items: []sdstemplate.ProductSummary{
				{
					ID: 101,
					DesignPrototype: &sdstemplate.DesignPrototype{PrototypeResultGroups: []sdstemplate.PrototypeResultGroup{
						{ResultImage: "https://cdn.sdspod.com/second.jpg", Sort: 2},
						{ResultImage: "https://cdn.sdspod.com/first.jpg", Sort: 1},
					}},
					ImgURL:    "https://cdn.sdspod.com/first.jpg",
					PSDImgURL: "https://cdn.sdspod.com/psd.jpg",
				},
			}},
		},
	}

	urls := renderedImageURLsFromProduct(product, 101)
	want := []string{
		"https://cdn.sdspod.com/first.jpg",
		"https://cdn.sdspod.com/second.jpg",
		"https://cdn.sdspod.com/psd.jpg",
	}
	if strings.Join(urls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected rendered urls: %+v", urls)
	}
}

func TestRenderedImageURLsFromProductSkipsSDSPlaceholder(t *testing.T) {
	t.Parallel()

	product := &sdstemplate.ProductDetail{
		ProductSummary: sdstemplate.ProductSummary{
			Subproducts: &sdstemplate.Subproducts{Items: []sdstemplate.ProductSummary{
				{
					ID: 101,
					DesignPrototype: &sdstemplate.DesignPrototype{PrototypeResultGroups: []sdstemplate.PrototypeResultGroup{
						{ResultImage: "https://cdn.sdspod.com/output/shengchengzhong.png", Sort: 1},
						{ResultImage: "https://cdn.sdspod.com/out/0/202604/rendered.jpg", Sort: 2},
					}},
					ImgURL: "https://cdn.sdspod.com/output/loading.png",
				},
			}},
		},
	}

	urls := renderedImageURLsFromProduct(product, 101)
	if strings.Join(urls, "|") != "https://cdn.sdspod.com/out/0/202604/rendered.jpg" {
		t.Fatalf("unexpected rendered urls: %+v", urls)
	}
}

func TestStaleRenderedImageURLsDetectsInitialProductImage(t *testing.T) {
	t.Parallel()

	result := &PrepareSyncDesignResult{
		Page: &DesignProductPage{
			Product: DesignProduct{
				ImgURL:    "https://cdn.sdspod.com/old-main.jpg",
				PSDImgURL: "https://cdn.sdspod.com/old-main.jpg",
			},
		},
	}
	if !staleRenderedImageURLs([]string{"https://cdn.sdspod.com/old-main.jpg"}, result) {
		t.Fatal("expected stale rendered urls")
	}
	if staleRenderedImageURLs([]string{"https://cdn.sdspod.com/new-main.jpg"}, result) {
		t.Fatal("new rendered url should not be stale")
	}
}

func TestStaleRenderedImageURLsDetectsSDSPlaceholder(t *testing.T) {
	t.Parallel()

	result := &PrepareSyncDesignResult{
		Page: &DesignProductPage{Product: DesignProduct{ImgURL: "https://cdn.sdspod.com/old-main.jpg"}},
	}
	if !staleRenderedImageURLs([]string{"https://cdn.sdspod.com/output/shengchengzhong.png"}, result) {
		t.Fatal("expected SDS generating placeholder to be stale")
	}
}

func TestSelectFinishedProductImageURLsUsesNewestMatchingMaterial(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        20,
			MaterialImageName: "other",
			ImageURLs:         []string{"https://cdn.sdspod.com/other.jpg"},
		},
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        10,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs:         []string{"https://cdn.sdspod.com/old.jpg"},
		},
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        30,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs: []string{
				"https://cdn.sdspod.com/main.jpg",
				"https://cdn.sdspod.com/main.jpg",
				"https://cdn.sdspod.com/gallery.jpg",
			},
		},
	}, 101, "listingkit-studio-design")

	want := []string{"https://cdn.sdspod.com/main.jpg", "https://cdn.sdspod.com/gallery.jpg"}
	if strings.Join(urls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected finished product urls: %+v", urls)
	}
}

func TestSelectFinishedProductImageURLsRejectsMismatchedMaterial(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        10,
			MaterialImageName: "finished-library-design",
			ImageURLs:         []string{"https://cdn.sdspod.com/rendered.jpg"},
		},
	}, 101, "local-file-name")

	if len(urls) != 0 {
		t.Fatalf("expected mismatched finished product urls to be ignored: %+v", urls)
	}
}

func TestSelectFinishedProductImageURLsFallsBackWhenMaterialUnknown(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        10,
			MaterialImageName: "finished-library-design",
			ImageURLs:         []string{"https://cdn.sdspod.com/out/0/202604/rendered.jpg"},
		},
	}, 101, "")

	if strings.Join(urls, "|") != "https://cdn.sdspod.com/out/0/202604/rendered.jpg" {
		t.Fatalf("unexpected fallback finished product urls: %+v", urls)
	}
}

func TestSelectFinishedProductImageURLsFiltersPlaceholders(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:   101,
			BuildFinish: true,
			FinishTime:  10,
			ImageURLs: []string{
				"https://cdn.sdspod.com/output/shengchengzhong.png",
				"https://cdn.sdspod.com/out/0/202604/rendered.jpg",
			},
		},
	}, 101, "")

	if strings.Join(urls, "|") != "https://cdn.sdspod.com/out/0/202604/rendered.jpg" {
		t.Fatalf("unexpected finished product urls: %+v", urls)
	}
}

func TestRenderedImageURLsReadyWaitsForExpectedPSDCount(t *testing.T) {
	t.Parallel()

	result := &PrepareSyncDesignResult{
		Page: &DesignProductPage{PSDs: []PSDDocument{
			{FileCode: "main.psd"},
			{FileCode: "gallery.psd"},
			{FileCode: "scene.psd"},
		}},
	}

	expected := expectedRenderedImageCount(result)
	if expected != 3 {
		t.Fatalf("unexpected expected count: %d", expected)
	}
	if renderedImageURLsReady([]string{"https://cdn.sdspod.com/out/main.jpg"}, expected) {
		t.Fatal("single early render should not be treated as ready")
	}
	if !renderedImageURLsReady([]string{
		"https://cdn.sdspod.com/out/main.jpg",
		"https://cdn.sdspod.com/out/gallery.jpg",
		"https://cdn.sdspod.com/out/scene.jpg",
	}, expected) {
		t.Fatal("all expected renders should be ready")
	}
}

func TestBuildFabricJSON(t *testing.T) {
	t.Parallel()

	raw, err := buildFabricJSON(&UploadedMaterial{
		Image:    &UploadedImage{ImageURL: "https://cdn.sdspod.com/upload.jpg", Width: 1080, Height: 2340},
		Material: &Material{ID: 1, ImageURL: "https://cdn.sdspod.com/material.jpg", Width: 1080, Height: 2340},
	}, &DesignLayer{
		ID:          "698744758333792256",
		Name:        "素材",
		PrintWidth:  850,
		PrintHeight: 1049,
	}, 1)
	if err != nil {
		t.Fatalf("build fabric json: %v", err)
	}

	if !strings.Contains(raw, `"src":"https://cdn.sdspod.com/material.jpg?material_id=1"`) {
		t.Fatalf("unexpected fabric json: %s", raw)
	}
	if !strings.Contains(raw, `"scaleX"`) {
		t.Fatalf("expected scale in fabric json: %s", raw)
	}
}
