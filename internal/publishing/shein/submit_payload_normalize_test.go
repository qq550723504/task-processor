package shein

import (
	"encoding/json"
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestNormalizeSubmitCollectionsAndExtra(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{}

	NormalizeSubmitCollections(product)
	NormalizeSubmitExtra(product)

	if product.BrandSeriesList == nil ||
		product.MultiLanguageMakeupIngredientList == nil ||
		product.ProductVideoList == nil ||
		product.PartInfoList == nil ||
		product.PLMPatternIDList == nil ||
		product.SizeAttributeList == nil ||
		product.BackSizeAttributeList == nil ||
		product.ProductCertificateList == nil ||
		product.CertificateList == nil ||
		product.DelOtherCertificateSNList == nil ||
		product.CustomAttributeRelation == nil {
		t.Fatalf("normalized collections still contain nil slices: %+v", product)
	}
	if product.Extra.FromPageID == nil || *product.Extra.FromPageID != "product_publish" {
		t.Fatalf("from_page_id = %+v, want product_publish", product.Extra.FromPageID)
	}
	if product.Extra.SwitchToSPUPic || product.Extra.UseCVTransformImage || product.Extra.TransformCVSizeImage {
		t.Fatalf("extra flags = %+v, want disabled", product.Extra)
	}
}

func TestFinalizeSubmitTransportFieldsSerializesEmptyCollections(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			SKUS: []sheinproduct.SKU{{SupplierSKU: "SKU-1"}},
		}},
	}

	FinalizeSubmitTransportFields(product)

	data, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("marshal product: %v", err)
	}
	jsonText := string(data)
	for _, want := range []string{
		`"site_detail_image_info_list":[]`,
		`"site_spec_image_info_list":[]`,
		`"skc_scope_attribute_list":[]`,
		`"proof_of_stock_list":[]`,
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
}
