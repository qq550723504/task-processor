package listingkit

import (
	"encoding/json"
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestPrepareSheinProductForNewSubmitDefaultsShelfWay(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{
					ImageType: 1,
					ImageSort: 1,
					ImageURL:  "https://img.example.com/a.jpg",
				}},
			},
			SKUS: []sheinproduct.SKU{{
				StockInfoList: []sheinproduct.StockInfo{{
					MerchantWarehouseCode: "DEFAULT",
					InventoryNum:          10,
				}},
			}},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	if got := product.SKCList[0].ShelfWay; got != 1 {
		t.Fatalf("shelf_way = %d, want 1", got)
	}
}

func TestPrepareSheinProductForSubmitRepairsMissingSKUImageFromSKC(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{
					{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example.com/skc-main.jpg"},
					{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/skc-gallery.jpg"},
				},
			},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: "SKU-1",
			}},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	got := product.SKCList[0].SKUS[0].ImageInfo
	if got == nil || len(got.ImageInfoList) != 1 {
		t.Fatalf("sku image info = %+v, want single fallback image", got)
	}
	if got.ImageInfoList[0].ImageURL != "https://img.example.com/skc-main.jpg" {
		t.Fatalf("sku image url = %q, want skc main image", got.ImageInfoList[0].ImageURL)
	}
	if got.ImageInfoList[0].ImageType != 1 || got.ImageInfoList[0].ImageSort != 1 {
		t.Fatalf("sku image detail = %+v, want type=1 sort=1", got.ImageInfoList[0])
	}
}

func TestPrepareSheinProductForSubmitTrimsSKUImagesToSingleImage(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{
					ImageType: 1,
					ImageSort: 1,
					ImageURL:  "https://img.example.com/skc-main.jpg",
				}},
			},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: "SKU-1",
				ImageInfo: &sheinproduct.ImageInfo{
					ImageInfoList: []sheinproduct.ImageDetail{
						{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/sku-1.jpg"},
						{ImageType: 2, ImageSort: 3, ImageURL: "https://img.example.com/sku-2.jpg"},
					},
				},
			}},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	got := product.SKCList[0].SKUS[0].ImageInfo
	if got == nil || len(got.ImageInfoList) != 1 {
		t.Fatalf("sku image info = %+v, want single image", got)
	}
	if got.ImageInfoList[0].ImageURL != "https://img.example.com/sku-1.jpg" {
		t.Fatalf("sku image url = %q, want first sku image", got.ImageInfoList[0].ImageURL)
	}
	if got.ImageInfoList[0].ImageType != 1 || got.ImageInfoList[0].ImageSort != 1 {
		t.Fatalf("sku image detail = %+v, want type=1 sort=1", got.ImageInfoList[0])
	}
}

func TestPrepareSheinProductForSubmitNormalizesSPUImagesForPublish(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example.com/main.jpg", MarketingMainImage: true},
				{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/gallery-1.jpg"},
				{ImageType: 2, ImageSort: 3, ImageURL: "https://img.example.com/gallery-2.jpg"},
			},
		},
	}

	prepareSheinProductForNewSubmit(product)

	got := product.ImageInfo
	if got == nil || len(got.ImageInfoList) != 4 {
		t.Fatalf("spu image info = %+v, want 4 normalized images without top-level color block", got)
	}
	if got.ImageInfoList[3].ImageType != 5 {
		t.Fatalf("square image = %+v, want type=5", got.ImageInfoList[3])
	}
	if got.ImageInfoList[0].ImageType != 1 || got.ImageInfoList[0].ImageSort != 1 {
		t.Fatalf("main image = %+v, want type=1 sort=1", got.ImageInfoList[0])
	}
	for _, image := range got.ImageInfoList {
		if image.ImageType == 6 {
			t.Fatalf("spu image list = %+v, want no top-level image_type=6", got.ImageInfoList)
		}
	}
}

func TestPrepareSheinProductForSubmitPopulatesSKCSiteDetailImages(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{
					{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example.com/skc-main.jpg"},
					{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/detail-1.jpg"},
					{ImageType: 2, ImageSort: 3, ImageURL: "https://img.example.com/detail-2.jpg"},
				},
			},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	got := product.SKCList[0].SiteDetailImageInfoList
	if len(got) != 1 {
		t.Fatalf("site_detail_image_info_list = %+v, want one populated detail image group", got)
	}
	if len(got[0].ImageInfoList) < 2 {
		t.Fatalf("detail images = %+v, want at least two images", got[0].ImageInfoList)
	}
	if got[0].ImageInfoList[0].ImageURL != "https://img.example.com/detail-1.jpg" {
		t.Fatalf("first detail image = %+v, want first gallery image", got[0].ImageInfoList[0])
	}
	if got[0].ImageInfoList[1].ImageURL != "https://img.example.com/detail-2.jpg" {
		t.Fatalf("second detail image = %+v, want second gallery image", got[0].ImageInfoList[1])
	}
}

func TestPrepareSheinProductForSubmitNormalizesExtraAndSupplierCode(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SupplierCode: "MG8089003001",
		SKCList: []sheinproduct.SKC{{
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: "MG8089003001-V245612-TED715457-PETBANDA",
			}},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	if strings.TrimSpace(product.PointKey) == "" {
		t.Fatal("point_key is empty, want generated UUID for direct publish parity with shein-listing")
	}
	if product.SupplierCode != "MG8089003001-PETBANDA" {
		t.Fatalf("supplier_code = %q, want MG8089003001-PETBANDA", product.SupplierCode)
	}
	if product.Extra.FromPageID == nil || *product.Extra.FromPageID != "product_publish" {
		t.Fatalf("from_page_id = %+v, want product_publish", product.Extra.FromPageID)
	}
	if product.Extra.UseCVTransformImage || product.Extra.TransformCVSizeImage || product.Extra.SwitchToSPUPic {
		t.Fatalf("extra = %+v, want publish defaults disabled", product.Extra)
	}
}

func TestPrepareSheinProductForNewSubmitSerializesPageStyleEmptyCollections(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example.com/main.jpg"},
				{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/gallery.jpg"},
			},
		},
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{
					{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example.com/skc-main.jpg"},
					{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/skc-gallery.jpg"},
				},
			},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: "SKU-1",
			}},
		}},
	}

	prepareSheinProductForNewSubmit(product)

	data, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("marshal product: %v", err)
	}
	jsonText := string(data)
	for _, want := range []string{
		`"site_detail_image_info_list":[{"site_abbr_list":[],"image_group_code":null,"image_info_list":[{"image_sort":1,"image_url":"https://img.example.com/skc-gallery.jpg"`,
		`"site_spec_image_info_list":[]`,
		`"skc_scope_attribute_list":[]`,
		`"competing_cost_price_images":[]`,
		`"confirm_volume_sku":[]`,
		`"confirm_weight_sku":[]`,
		`"spu_tag":[]`,
		`"control_price_data":{}`,
	} {
		if !strings.Contains(jsonText, want) {
			t.Fatalf("serialized product missing %s in %s", want, jsonText)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal product json: %v", err)
	}
	topImages, _ := decoded["image_info"].(map[string]any)
	topList, _ := topImages["image_info_list"].([]any)
	for _, raw := range topList {
		image, _ := raw.(map[string]any)
		if image["image_type"] == float64(6) {
			t.Fatalf("serialized spu payload should not include top-level color-block image: %s", jsonText)
		}
	}
}
