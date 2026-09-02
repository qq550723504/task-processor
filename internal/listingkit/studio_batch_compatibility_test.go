package listingkit

import "testing"

func TestStudioBatchCompatibilityFingerprintUsesDesignMaterializationInputs(t *testing.T) {
	base := SheinStudioSelection{ParentProductID: 2002, PrototypeGroupID: 4004, LayerID: "layer-front", DesignType: "material", PrintableWidth: 1200, PrintableHeight: 1200, TemplateImageURL: "https://cdn.example.com/template-a.png", MaskImageURL: "https://cdn.example.com/mask-a.png", ProductSize: "small"}
	equivalent := base
	equivalent.TemplateImageURL = " https://cdn.example.com/template-a.png "
	if buildStudioBatchCompatibilityFingerprint(base) != buildStudioBatchCompatibilityFingerprint(equivalent) {
		t.Fatal("equivalent selections should share a fingerprint")
	}

	for name, mutate := range map[string]func(*SheinStudioSelection){
		"template": func(value *SheinStudioSelection) { value.TemplateImageURL = "https://cdn.example.com/template-b.png" },
		"mask":     func(value *SheinStudioSelection) { value.MaskImageURL = "https://cdn.example.com/mask-b.png" },
		"size":     func(value *SheinStudioSelection) { value.ProductSize = "large" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if buildStudioBatchCompatibilityFingerprint(base) == buildStudioBatchCompatibilityFingerprint(changed) {
				t.Fatalf("%s change must alter the fingerprint", name)
			}
		})
	}
}
