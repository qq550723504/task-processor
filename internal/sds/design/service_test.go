package design

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sdsclient "task-processor/internal/sds/client"
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

func TestGetDesignProductForPrototypeGroupSendsPrototypeGroupQuery(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/ps/design/products/101" {
			t.Fatalf("path = %s, want /ps/design/products/101", r.URL.Path)
		}
		if got := r.URL.Query().Get("prototypeGroupId"); got != "7001" {
			t.Fatalf("prototypeGroupId query = %q, want 7001", got)
		}
		_, _ = w.Write([]byte(`{
			"product":{"id":101},
			"prototypeGroup":{"id":7001},
			"layers":[{"id":"layer-1"}]
		}`))
	}))
	defer server.Close()

	cfg := sdsclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.Endpoints.DesignProductPath = "/ps/design/products/%d"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	page, err := NewService(client).GetDesignProductForPrototypeGroup(context.Background(), 101, 7001)
	if err != nil {
		t.Fatalf("GetDesignProductForPrototypeGroup() error = %v", err)
	}
	if page == nil || page.Product.ID != 101 || page.PrototypeGroup.ID != 7001 {
		t.Fatalf("page = %+v, want selected prototype group page", page)
	}
}

func TestPrepareSyncDesignUsesRelatedVariantLayerIDs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/ps/design/products/101":
			_, _ = w.Write([]byte(`{
				"product":{"id":101,"prototypeId":"prototype-primary"},
				"prototypeGroup":{"id":15506},
				"layers":[{"id":"layer-primary","prototypeId":"prototype-primary","name":"primary-print","printWidth":100,"printHeight":100}],
				"psds":[{"id":"psd-primary","prototypeId":"prototype-primary","thumbnail_url":"https://cdn.sdspod.com/images/primary.jpg"}]
			}`))
		case "/ps/design/products/102":
			_, _ = w.Write([]byte(`{
				"product":{"id":102,"prototypeId":"prototype-related"},
				"prototypeGroup":{"id":15506},
				"layers":[
					{"id":"layer-related-wrong","prototypeId":"prototype-related","name":"wrong-layer","printWidth":100,"printHeight":100},
					{"id":"layer-related-correct","prototypeId":"prototype-related","name":"correct-layer","printWidth":200,"printHeight":120}
				],
				"psds":[{"id":"psd-related","prototypeId":"prototype-related","thumbnail_url":"https://cdn.sdspod.com/images/related.jpg"}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	cfg := sdsclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.Endpoints.DesignProductPath = "/ps/design/products/%d"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := NewService(client).PrepareSyncDesign(context.Background(), PrepareSyncDesignInput{
		VariantID:              101,
		RelatedVariantIDs:      []int64{102},
		RelatedVariantLayerIDs: map[int64]string{102: "layer-related-correct"},
		PrototypeGroupID:       15506,
		LayerID:                "layer-primary",
		FitLevel:               1,
	}, &UploadedMaterial{
		Image: &UploadedImage{
			Width:  50,
			Height: 50,
		},
		Material: &Material{
			ID:       460264499,
			Width:    50,
			Height:   50,
			ImageURL: "https://cdn.sdspod.com/material.png",
		},
	})
	if err != nil {
		t.Fatalf("PrepareSyncDesign() error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.Prototypes) != 2 {
		t.Fatalf("result request = %+v, want 2 prototypes", result)
	}
	relatedLayer := result.Request.Prototypes[1].Layers[0]
	if relatedLayer.LayerID != "layer-related-correct" {
		t.Fatalf("related layer id = %q, want layer-related-correct", relatedLayer.LayerID)
	}
	if relatedLayer.ImgWidth != 200 || relatedLayer.ImgHeight != 120 {
		t.Fatalf("related layer dimensions = %dx%d, want selected related layer dimensions", relatedLayer.ImgWidth, relatedLayer.ImgHeight)
	}
}

func TestDesignProductPageUnmarshalNumericPrototypeIDs(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"product": {
			"id": 10947,
			"parent_id": 10946,
			"sku": "XB0602011001",
			"parentSku": "XB0602011",
			"prototypeId": 10019364,
			"prototypeType": "FREE",
			"size": "One size",
			"sizeId": 1,
			"colorId": 1004,
			"color_name": "white"
		},
		"layers": [
			{
				"id": 10059417,
				"prototypeId": 10019364,
				"name": "素材",
				"type": 1
			}
		],
		"psds": [
			{
				"id": 782092292330442752,
				"prototypeId": 10019364,
				"fileId": "1",
				"fileCode": "abc.psd",
				"sort": 1
			}
		]
	}`)

	var page DesignProductPage
	if err := json.Unmarshal(payload, &page); err != nil {
		t.Fatalf("unmarshal design product page with numeric prototype ids: %v", err)
	}

	if page.Product.PrototypeID != "10019364" {
		t.Fatalf("product prototype id = %q, want numeric value normalized to string", page.Product.PrototypeID)
	}
	if len(page.Layers) != 1 || page.Layers[0].PrototypeID != "10019364" || page.Layers[0].ID != "10059417" {
		t.Fatalf("layers = %+v, want numeric ids normalized to string", page.Layers)
	}
	if len(page.PSDs) != 1 || page.PSDs[0].PrototypeID != "10019364" || page.PSDs[0].ID != "782092292330442752" {
		t.Fatalf("psds = %+v, want numeric ids normalized to string", page.PSDs)
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

func TestSelectFinishedProductImageURLsFallsBackToFinishedProductThumbnails(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        30,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs: []string{
				"https://cdn.sdspod.com/output/placeholder.jpg",
			},
			ThumbnailImageURLs: []string{
				"https://cdn.sdspod.com/outThumbs/0/202604/main.jpg",
				"https://cdn.sdspod.com/outThumbs/60678/202604/gallery-1.jpg",
				"https://cdn.sdspod.com/outThumbs/60678/202604/gallery-2.jpg",
			},
		},
	}, 101, "listingkit-studio-design")

	want := []string{
		"https://cdn.sdspod.com/outThumbs/0/202604/main.jpg",
		"https://cdn.sdspod.com/outThumbs/60678/202604/gallery-1.jpg",
		"https://cdn.sdspod.com/outThumbs/60678/202604/gallery-2.jpg",
	}
	if strings.Join(urls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected finished product urls: %+v", urls)
	}
}

func TestSelectFinishedProductImageURLsPreservesSDSFinishedProductItemImages(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        1_782_811_502_466,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs: []string{
				"https://cdn.sdspod.com/out/0/202604/front.jpg",
				"https://cdn.sdspod.com/images/source-front.jpg",
				"https://cdn.sdspod.com/images/source-back.jpg",
				"https://cdn.sdspod.com/images/source-detail.jpg",
				"https://cdn.sdspod.com/images/source-model.jpg",
			},
		},
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        1_782_811_502_416,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs: []string{
				"https://cdn.sdspod.com/out/60678/202604/side.jpg",
				"https://cdn.sdspod.com/images/source-side.jpg",
			},
		},
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        1_782_810_483_930,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs: []string{
				"https://cdn.sdspod.com/out/60678/202604/old-run.jpg",
			},
		},
	}, 101, "listingkit-studio-design")

	want := []string{
		"https://cdn.sdspod.com/out/0/202604/front.jpg",
		"https://cdn.sdspod.com/images/source-front.jpg",
		"https://cdn.sdspod.com/images/source-back.jpg",
		"https://cdn.sdspod.com/images/source-detail.jpg",
		"https://cdn.sdspod.com/images/source-model.jpg",
	}
	if strings.Join(urls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected finished product urls: %+v", urls)
	}
}

func TestBuildSaveDesignRequestPreservesPrintableLayerInputs(t *testing.T) {
	t.Parallel()

	result := &PrepareSyncDesignResult{
		Page: &DesignProductPage{
			PSDs: []PSDDocument{
				{ThumbnailURL: "https://cdn.sdspod.com/images/baseball-cap-thumb.jpg"},
			},
		},
		Material: &UploadedMaterial{
			Material: &Material{
				ID:       460264499,
				ImageURL: "https://cdn.sdspod.com/imagesThumbs/91rr3AHARTasVhdqqVyNm4TGH9ub5wHb8VhZiE45/material.png",
			},
		},
		Request: &SyncDesignRequest{
			ProductID:        96770,
			PrototypeGroupID: 15508,
			DesignType:       "material",
			Prototypes: []SyncDesignPrototype{
				{
					PrototypeID: "743672383889936385",
					ProductIDs:  []int64{96770},
					PSDIDs:      []string{"743672653353951232"},
					Images:      []string{"http://e.sdspod.com/builds?content=fused"},
					Layers: []SyncDesignLayer{
						{
							LayerID:            "743672384191926272",
							Content:            "",
							ImgWidth:           1000,
							ImgHeight:          600,
							ResizeMode:         0,
							FitLevel:           1,
							FabricJSON:         `{"objects":[{"src":"https://cdn.sdspod.com/material.png?material_id=460264499"}]}`,
							RelatedMaterialIDs: []int64{460264499},
						},
					},
				},
			},
		},
	}

	req := buildSaveDesignRequest(result)
	if len(req.Prototypes) != 1 || len(req.Prototypes[0].Images) != 1 {
		t.Fatalf("save request images = %+v, want SDS thumbnail urls", req.Prototypes)
	}
	if req.Prototypes[0].Images[0] != "https://cdn.sdspod.com/images/baseball-cap-thumb.jpg" {
		t.Fatalf("save request image = %q, want SDS thumbnail url", req.Prototypes[0].Images[0])
	}
	layer := req.Prototypes[0].Layers[0]
	if layer.ImgWidth != 1000 || layer.ImgHeight != 600 {
		t.Fatalf("save layer dimensions = %dx%d, want original printable area", layer.ImgWidth, layer.ImgHeight)
	}
	if layer.FabricJSON == "" {
		t.Fatal("save layer lost fabric json")
	}
	if layer.Content != "91rr3AHARTasVhdqqVyNm4TGH9ub5wHb8VhZiE45/material.png" {
		t.Fatalf("save layer content = %q, want material content path", layer.Content)
	}
	if layer.MaterialID != int64(460264499) || layer.DesignMaterialID != 460264499 {
		t.Fatalf("save material ids = %+v/%d, want uploaded material id", layer.MaterialID, layer.DesignMaterialID)
	}
}

func TestBuildSaveDesignRequestUsesVariantPageThumbnails(t *testing.T) {
	t.Parallel()

	primaryPage := &DesignProductPage{
		Product: DesignProduct{ID: 101},
		PSDs: []PSDDocument{
			{ThumbnailURL: "https://cdn.sdspod.com/images/red-thumb.jpg"},
		},
	}
	relatedPage := &DesignProductPage{
		Product: DesignProduct{ID: 102},
		PSDs: []PSDDocument{
			{ThumbnailURL: "https://cdn.sdspod.com/images/green-thumb.jpg"},
		},
	}
	result := &PrepareSyncDesignResult{
		Page: primaryPage,
		RelatedPages: map[int64]*DesignProductPage{
			101: primaryPage,
			102: relatedPage,
		},
		Material: &UploadedMaterial{
			Material: &Material{
				ID:       460264499,
				ImageURL: "https://cdn.sdspod.com/imagesThumbs/91rr3AHARTasVhdqqVyNm4TGH9ub5wHb8VhZiE45/material.png",
			},
		},
		Request: &SyncDesignRequest{
			ProductID:        101,
			PrototypeGroupID: 15508,
			DesignType:       "material",
			Prototypes: []SyncDesignPrototype{
				{
					PrototypeID: "prototype-red",
					ProductIDs:  []int64{101},
					Images:      []string{"http://e.sdspod.com/builds?content=red-preview"},
					Layers: []SyncDesignLayer{
						{LayerID: "layer-red", ImgWidth: 1000, ImgHeight: 600},
					},
				},
				{
					PrototypeID: "prototype-green",
					ProductIDs:  []int64{102},
					Images:      []string{"http://e.sdspod.com/builds?content=green-preview"},
					Layers: []SyncDesignLayer{
						{LayerID: "layer-green", ImgWidth: 1000, ImgHeight: 600},
					},
				},
			},
		},
	}

	req := buildSaveDesignRequest(result)
	if len(req.Prototypes) != 2 {
		t.Fatalf("prototypes = %+v, want 2", req.Prototypes)
	}
	if got := req.Prototypes[0].Images; len(got) != 1 || got[0] != "https://cdn.sdspod.com/images/red-thumb.jpg" {
		t.Fatalf("primary images = %+v", got)
	}
	if got := req.Prototypes[1].Images; len(got) != 1 || got[0] != "https://cdn.sdspod.com/images/green-thumb.jpg" {
		t.Fatalf("related images = %+v", got)
	}
}

func TestSelectFinishedProductImageURLsKeepsStaticGalleryForSingleRenderableView(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLsWithAcceptAndStaticGallery([]DesignProductListItem{
		{
			ProductID:         101,
			MaterialImageName: "single-print",
			BuildFinish:       true,
			FinishTime:        20,
			ImageURLs: []string{
				"https://cdn.sdspod.com/out/0/202604/front-rendered.jpg",
				"https://cdn.sdspod.com/images/side.jpg",
				"https://cdn.sdspod.com/images/back.jpg",
			},
		},
	}, 101, "single-print", nil, true)

	want := []string{
		"https://cdn.sdspod.com/out/0/202604/front-rendered.jpg",
		"https://cdn.sdspod.com/images/side.jpg",
		"https://cdn.sdspod.com/images/back.jpg",
	}
	if strings.Join(urls, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected single-renderable-view gallery urls: %+v", urls)
	}
}

func TestSelectFinishedProductImageURLsSkipsRejectedCandidate(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLsWithAccept([]DesignProductListItem{
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        30,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs:         []string{"https://cdn.sdspod.com/out/blank.jpg"},
		},
		{
			ProductID:         101,
			BuildFinish:       true,
			FinishTime:        20,
			MaterialImageName: "listingkit-studio-design",
			ImageURLs:         []string{"https://cdn.sdspod.com/out/rendered.jpg"},
		},
	}, 101, "listingkit-studio-design", func(urls []string) bool {
		return len(urls) > 0 && !strings.Contains(urls[0], "blank")
	})

	if strings.Join(urls, "|") != "https://cdn.sdspod.com/out/rendered.jpg" {
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

func TestSelectFinishedProductImageURLsRejectsMissingMaterialWhenExpected(t *testing.T) {
	t.Parallel()

	urls := selectFinishedProductImageURLs([]DesignProductListItem{
		{
			ProductID:   101,
			BuildFinish: true,
			FinishTime:  10,
			ImageURLs:   []string{"https://cdn.sdspod.com/out/0/202604/rendered.jpg"},
		},
	}, 101, "listingkit-studio-design")

	if len(urls) != 0 {
		t.Fatalf("expected unidentified finished product urls to be ignored: %+v", urls)
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

func TestRenderedImageURLsByProductReadyUsesReturnedSDSImages(t *testing.T) {
	t.Parallel()

	urlsByProduct := map[int64][]string{
		101: []string{"https://cdn.sdspod.com/out/front.jpg"},
	}

	if !renderedImageURLsByProductReady(urlsByProduct, []int64{101}, 1) {
		t.Fatal("returned SDS image should be ready when no stricter PSD count is known")
	}
	if renderedImageURLsByProductReady(urlsByProduct, []int64{101}, 2) {
		t.Fatal("single returned SDS image should not be ready when the PSD count explicitly expects two renders")
	}
}

func TestRenderedImageURLsByProductReadyWaitsForExpectedOutRenders(t *testing.T) {
	t.Parallel()

	earlyURLsByProduct := map[int64][]string{
		101: []string{
			"https://cdn.sdspod.com/out/0/202607/front-rendered.jpg",
			"https://cdn.sdspod.com/images/static-1.jpg",
			"https://cdn.sdspod.com/images/static-2.jpg",
			"https://cdn.sdspod.com/images/static-3.jpg",
			"https://cdn.sdspod.com/images/static-4.jpg",
			"https://cdn.sdspod.com/images/static-5.jpg",
			"https://cdn.sdspod.com/images/static-6.jpg",
		},
	}
	if renderedImageURLsByProductReady(earlyURLsByProduct, []int64{101}, 6) {
		t.Fatal("static gallery images must not satisfy multi-PSD rendered image readiness")
	}

	readyURLsByProduct := map[int64][]string{
		101: []string{
			"https://cdn.sdspod.com/out/0/202607/front-rendered.jpg",
			"https://cdn.sdspod.com/out/60678/202607/side-rendered.jpg",
			"https://cdn.sdspod.com/out/60678/202607/back-rendered.jpg",
			"https://cdn.sdspod.com/out/60678/202607/detail-rendered.jpg",
			"https://cdn.sdspod.com/out/60678/202607/bottom-rendered.jpg",
			"https://cdn.sdspod.com/out/60678/202607/top-rendered.jpg",
			"https://cdn.sdspod.com/images/static-1.jpg",
		},
	}
	if !renderedImageURLsByProductReady(readyURLsByProduct, []int64{101}, 6) {
		t.Fatal("ready multi-PSD result should count rendered /out images and still allow static gallery images")
	}
}

func TestRenderedImagePollAttemptsScalesForVariantTargets(t *testing.T) {
	t.Parallel()

	if got := renderedImagePollAttempts(1); got != maxRenderedImagePollAttempts {
		t.Fatalf("single target attempts = %d, want %d", got, maxRenderedImagePollAttempts)
	}
	if got := renderedImagePollAttempts(3); got != maxRenderedImagePollAttempts+16 {
		t.Fatalf("three target attempts = %d, want %d", got, maxRenderedImagePollAttempts+16)
	}
	if got := renderedImagePollAttempts(10); got != maxRenderedImagePollAttempts+maxRenderedImagePollExtra {
		t.Fatalf("capped attempts = %d, want %d", got, maxRenderedImagePollAttempts+maxRenderedImagePollExtra)
	}
}

func TestIsSDSTooFrequentError(t *testing.T) {
	t.Parallel()

	if !isSDSTooFrequentError(assertErr("sds POST /ps/design/add_and_design failed with status 400: {\"msg\":\"您提交得太频繁了，请稍后再试!\"}")) {
		t.Fatal("expected Chinese SDS rate limit error to be detected")
	}
	if !isSDSTooFrequentError(assertErr("too frequent")) {
		t.Fatal("expected English rate limit error to be detected")
	}
	if isSDSTooFrequentError(assertErr("permission denied")) {
		t.Fatal("unrelated error should not be treated as rate limit")
	}
}

func assertErr(message string) error {
	return errString(message)
}

type errString string

func (e errString) Error() string { return string(e) }

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

func TestBuildSensitiveDesignProductUpdatesRemovesBlockedExportWords(t *testing.T) {
	t.Parallel()

	updates := buildSensitiveDesignProductUpdates([]DesignProductListItem{
		{
			ID:                "904272754285088768",
			ExportName:        "Simulated Silk Sleep Masks",
			MaterialImageName: "listingkit-studio-design-91e86def",
			MaterialColor:     "style",
			ParentAttribute:   2,
			MaterialVariant: []DesignProductListItem{
				{
					ID:                "904272754280894464",
					ExportName:        "Simulated Silk Sleep Masks",
					MaterialImageName: "listingkit-studio-design-91e86def",
					ParentAttribute:   2,
				},
			},
		},
	}, map[string][]SensitiveWordHit{
		"904272754285088768": {{SensitiveWord: "Mask", PositionStrs: "导出名称"}},
		"904272754280894464": {{SensitiveWord: "Mask", PositionStrs: "导出名称"}},
	})

	if len(updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(updates))
	}
	for _, update := range updates {
		if strings.Contains(strings.ToLower(update.Name), "mask") {
			t.Fatalf("sanitized name still contains blocked word: %+v", update)
		}
		if update.Name != "Simulated Silk Sleep" {
			t.Fatalf("sanitized name = %q, want %q", update.Name, "Simulated Silk Sleep")
		}
	}
}

func TestUpdateDesignProductsUsesConfiguredEndpoint(t *testing.T) {
	t.Parallel()

	var method string
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		if r.URL.Path != "/design_products" {
			t.Fatalf("path = %s, want /design_products", r.URL.Path)
		}
		defer r.Body.Close()
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		body = payload
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ret":0,"msg":"ok"}`))
	}))
	defer server.Close()

	cfg := sdsclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.Endpoints.DesignProductsUpdatePath = server.URL + "/design_products"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	service := NewService(client)
	err = service.UpdateDesignProducts(context.Background(), []UpdateDesignProductRequest{{
		ID:                "904272754285088768",
		Name:              "Simulated Silk Sleep",
		MaterialImageName: "listingkit-studio-design-91e86def",
	}})
	if err != nil {
		t.Fatalf("update design products: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("method = %s, want PUT", method)
	}
	if !strings.Contains(string(body), `"name":"Simulated Silk Sleep"`) {
		t.Fatalf("unexpected body: %s", string(body))
	}
}

func TestDeleteSDSMaterialAndEndProductUseConfiguredEndpoints(t *testing.T) {
	t.Parallel()

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ret":0,"msg":"SUCCESS"}`))
	}))
	defer server.Close()

	cfg := sdsclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.Endpoints.MaterialDeletePath = "/ps/material/%d"
	cfg.Endpoints.EndProductDeletePath = "/ps/endproducts/%s"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	service := NewService(client)
	if err := service.DeleteMaterial(context.Background(), 467483443); err != nil {
		t.Fatalf("delete material: %v", err)
	}
	if err := service.DeleteEndProduct(context.Background(), "921516041142050816"); err != nil {
		t.Fatalf("delete endproduct: %v", err)
	}

	if strings.Join(paths, "|") != "/ps/material/467483443|/ps/endproducts/921516041142050816" {
		t.Fatalf("delete paths = %+v", paths)
	}
}

func TestPrepareAndSyncDesignCleansUpCreatedSDSArtifacts(t *testing.T) {
	t.Parallel()

	var deleted []string
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/upload-sign":
			_, _ = w.Write([]byte(`{"dir":"listingkit/","policy":"p","ossAccessKeyId":"ak","signature":"sig","host":"` + serverURL + `/oss"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/oss":
			_, _ = w.Write([]byte(`ok`))
		case r.Method == http.MethodPost && r.URL.Path == "/materials/one":
			_, _ = w.Write([]byte(`{"ret":0,"msg":"SUCCESS","data":[{"id":467483443,"name":"cleanup.png","file_code":"cleanup.png","content_type":"image/png","width":1,"height":1,"img_url":"https://cdn.sdspod.com/material.png"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/materials/findByIds":
			_, _ = w.Write([]byte(`[{"id":467483443,"name":"cleanup.png","file_code":"cleanup.png","content_type":"image/png","width":1,"height":1,"img_url":"https://cdn.sdspod.com/material.png"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/ps/design/products/82491":
			_, _ = w.Write([]byte(`{"product":{"id":82491,"parent_id":82490,"prototypeId":"proto-1","img_url":"https://cdn.sdspod.com/old.jpg"},"prototypeGroup":{"id":13229},"layers":[{"id":"layer-1","prototypeId":"proto-1","name":"素材","printWidth":100,"printHeight":100}],"psds":[{"id":"psd-1","prototypeId":"proto-1","fileCode":"model.psd","thumbnail_url":"https://cdn.sdspod.com/model-thumb.jpg"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/ps/design/syncDesign":
			_, _ = w.Write([]byte(`{"ret":0,"msg":"SUCCESS"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/ps/design/add_and_design":
			_, _ = w.Write([]byte(`{"ret":0,"msg":"SUCCESS"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/design_products":
			if got := r.URL.Query().Get("search"); got != "cleanup" {
				t.Fatalf("design_products search = %q, want cleanup", got)
			}
			_, _ = w.Write([]byte(`{"items":[{"id":"921516041142050816","product_id":82491,"buildFinish":true,"status":2,"finish_time":99,"material_img_name":"cleanup","img_urls":["https://cdn.sdspod.com/out/rendered.jpg"]}],"total_count":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/products/82490":
			_, _ = w.Write([]byte(`{"id":82490,"subproducts":{"items":[{"id":82491,"designPrototype":{"prototypeResultGroups":[{"sort":1,"resultImage":"https://cdn.sdspod.com/out/rendered.jpg"}]}}]}}`))
		case r.Method == http.MethodDelete && (r.URL.Path == "/ps/material/467483443" || r.URL.Path == "/ps/endproducts/921516041142050816"):
			deleted = append(deleted, r.URL.Path)
			_, _ = w.Write([]byte(`{"ret":0,"msg":"SUCCESS"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	serverURL = server.URL

	cfg := sdsclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.Endpoints.UploadSignPath = "/upload-sign"
	cfg.Endpoints.MaterialCreatePath = "/materials/one"
	cfg.Endpoints.MaterialFindByIDs = "/materials/findByIds"
	cfg.Endpoints.DesignProductPath = "/ps/design/products/%d"
	cfg.Endpoints.SyncDesignPath = "/ps/design/syncDesign"
	cfg.Endpoints.AddAndDesignPath = "/ps/design/add_and_design"
	cfg.Endpoints.DesignProductsPath = server.URL + "/design_products"
	cfg.Endpoints.MaterialDeletePath = "/ps/material/%d"
	cfg.Endpoints.EndProductDeletePath = "/ps/endproducts/%s"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	result, err := NewService(client).PrepareAndSyncDesign(context.Background(), PrepareSyncDesignInput{
		VariantID:        82491,
		ParentProductID:  82490,
		PrototypeGroupID: 13229,
		LayerID:          "layer-1",
		DesignType:       "material",
		FitLevel:         1,
	}, UploadRequest{
		FileName:    "cleanup.png",
		Content:     []byte{0x89, 0x50, 0x4e, 0x47},
		ContentType: "image/png",
		Width:       1,
		Height:      1,
	})
	if err != nil {
		t.Fatalf("PrepareAndSyncDesign() error = %v", err)
	}
	if result == nil || len(result.RenderedImageURLs) == 0 {
		t.Fatalf("result = %+v, want rendered image urls", result)
	}
	if strings.Join(deleted, "|") != "/ps/endproducts/921516041142050816|/ps/material/467483443" {
		t.Fatalf("deleted paths = %+v", deleted)
	}
}

func TestCleanupDesignArtifactsReportsDeleteFailures(t *testing.T) {
	t.Parallel()

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/ps/endproducts/921516041142050816" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ret":500,"msg":"delete endproduct failed"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ret":0,"msg":"SUCCESS"}`))
	}))
	defer server.Close()

	cfg := sdsclient.DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.RetryCount = 0
	cfg.Endpoints.MaterialDeletePath = "/ps/material/%d"
	cfg.Endpoints.EndProductDeletePath = "/ps/endproducts/%s"
	cfg.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = NewService(client).cleanupDesignArtifacts(context.Background(), &UploadedMaterial{
		Material: &Material{ID: 467483443},
	}, &PrepareSyncDesignResult{
		RenderedImageObservations: map[int64]RenderedImageObservation{
			82491: {ItemID: "921516041142050816"},
		},
	})
	if err == nil {
		t.Fatal("cleanupDesignArtifacts() error = nil, want delete failure")
	}
	if !strings.Contains(err.Error(), "delete SDS endproduct 921516041142050816") {
		t.Fatalf("cleanupDesignArtifacts() error = %v", err)
	}
	if strings.Join(paths, "|") != "/ps/endproducts/921516041142050816|/ps/material/467483443" {
		t.Fatalf("delete paths = %+v, want cleanup to continue after endproduct failure", paths)
	}
}

func TestCollectRenderedImageObservationsUsesItemIDAliasForCleanup(t *testing.T) {
	t.Parallel()

	var response DesignProductListResponse
	payload := []byte(`{"items":[{"item_id":"921516041142050816","product_id":82491,"buildFinish":true,"status":2,"finish_time":99,"img_urls":["https://cdn.sdspod.com/out/rendered.jpg"]}],"total_count":1}`)
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("unmarshal design products: %v", err)
	}

	observations := collectRenderedImageObservations(response.Items, []int64{82491})
	observation := observations[82491]
	if observation.ItemID != "921516041142050816" {
		t.Fatalf("observation item id = %q, want item_id alias for cleanup", observation.ItemID)
	}
}
