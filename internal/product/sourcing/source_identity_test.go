package sourcing

import "testing"

func TestSourceRequestIdentityPreservesLegacyKey(t *testing.T) {
	id := SourceRequest{
		Platform:  " Amazon ",
		Region:    " UK ",
		ProductID: " B001 ",
		StoreID:   42,
	}.Identity()

	if id.SourceType != SourceTypeCrawler {
		t.Fatalf("SourceType = %q, want %q", id.SourceType, SourceTypeCrawler)
	}
	if id.SourcePlatform != "amazon" {
		t.Fatalf("SourcePlatform = %q, want amazon", id.SourcePlatform)
	}
	if id.SourceID != "B001" {
		t.Fatalf("SourceID = %q, want B001", id.SourceID)
	}
	if got := id.Key(); got != "amazon:uk:B001:42" {
		t.Fatalf("Key() = %q, want legacy key", got)
	}
	if got := id.SourceKey(); got != "crawler:amazon:B001" {
		t.Fatalf("SourceKey() = %q, want source key", got)
	}
}

func TestSourceIdentityValidationDistinguishesMissingSourceID(t *testing.T) {
	id := NormalizeSourceIdentity(SourceIdentity{
		SourceType:     SourceTypeWarehouseCatalog,
		SourcePlatform: " dajian ",
		SourceURL:      " https://source.example/products/123 ",
		SourceVersion:  " snapshot-1 ",
	})

	validation := id.Validation()
	if validation.Valid() {
		t.Fatal("Validation().Valid() = true, want weak identity to require explicit acceptance")
	}
	if !validation.MissingSourceID {
		t.Fatal("MissingSourceID = false, want true")
	}
	if !validation.Fingerprintable {
		t.Fatal("Fingerprintable = false, want true")
	}
	if validation.MissingFingerprint {
		t.Fatal("MissingFingerprint = true, want derived fingerprint")
	}
	if !validation.WeakButFingerprintable() {
		t.Fatal("WeakButFingerprintable() = false, want true")
	}
	if id.SourceFingerprint == "" {
		t.Fatal("SourceFingerprint is empty, want derived fingerprint")
	}
	if got := id.SourceKey(); got == "" || got == "warehouse_catalog:dajian:fingerprint:" {
		t.Fatalf("SourceKey() = %q, want fingerprint-backed key", got)
	}
}

func TestSourceIdentityFingerprintIsStable(t *testing.T) {
	first := NormalizeSourceIdentity(SourceIdentity{
		SourceType:     " Warehouse_Catalog ",
		SourcePlatform: " DAJIAN ",
		SourceURL:      " https://source.example/products/123 ",
		SourceVersion:  " snapshot-1 ",
	})
	second := NormalizeSourceIdentity(SourceIdentity{
		SourceType:     "warehouse_catalog",
		SourcePlatform: "dajian",
		SourceURL:      "https://source.example/products/123",
		SourceVersion:  "snapshot-1",
	})

	if first.SourceFingerprint == "" {
		t.Fatal("first fingerprint is empty")
	}
	if first.SourceFingerprint != second.SourceFingerprint {
		t.Fatalf("fingerprint changed: %q != %q", first.SourceFingerprint, second.SourceFingerprint)
	}
	if first.SourceKey() != second.SourceKey() {
		t.Fatalf("source key changed: %q != %q", first.SourceKey(), second.SourceKey())
	}
}

func TestSourceEnvelopeNormalizeTrimsIdentityAndWarnings(t *testing.T) {
	envelope := SourceEnvelope{
		Identity: SourceIdentity{
			SourceType:     " CRAWLER ",
			SourcePlatform: " Amazon ",
			SourceID:       " B001 ",
		},
		Warnings: []SourceWarning{{Code: " Missing_Title ", Field: " title ", Message: " title is empty "}},
	}

	got := envelope.Normalize()
	if got.Identity.SourceType != SourceTypeCrawler {
		t.Fatalf("SourceType = %q, want %q", got.Identity.SourceType, SourceTypeCrawler)
	}
	if got.Identity.SourcePlatform != "amazon" {
		t.Fatalf("SourcePlatform = %q, want amazon", got.Identity.SourcePlatform)
	}
	if len(got.Warnings) != 1 {
		t.Fatalf("warnings = %d, want 1", len(got.Warnings))
	}
	if got.Warnings[0].Code != "missing_title" || got.Warnings[0].Message != "title is empty" {
		t.Fatalf("warning = %+v, want normalized metadata", got.Warnings[0])
	}
}
