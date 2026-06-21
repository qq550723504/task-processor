package shein

import "testing"

func TestSubmitTokenClassifiers(t *testing.T) {
	t.Parallel()

	if !LooksLikeSubmitTaskToken("TF79B3E36") {
		t.Fatal("LooksLikeSubmitTaskToken() = false, want true")
	}
	if LooksLikeSubmitTaskToken("TXYZ12345") {
		t.Fatal("LooksLikeSubmitTaskToken(non-hex) = true, want false")
	}
	if !LooksLikeSubmitRequestToken("RF898D") {
		t.Fatal("LooksLikeSubmitRequestToken() = false, want true")
	}
	if LooksLikeSubmitRequestToken("F898D") {
		t.Fatal("LooksLikeSubmitRequestToken(no prefix) = true, want false")
	}
}

func TestDeriveSubmitStyleSuffixUsesShortAndLongTokens(t *testing.T) {
	t.Parallel()

	got := DeriveSubmitStyleSuffix("fresh SDS image", "XL ceramic bottle opener")
	if got != "XLCERAMI" {
		t.Fatalf("DeriveSubmitStyleSuffix() = %q, want XLCERAMI", got)
	}
}

func TestSubmitDiscriminatorsNormalizeIDs(t *testing.T) {
	t.Parallel()

	if got := SubmitTaskDiscriminator("f79b3e36-d6b9-440d"); got != "TF79B3E36" {
		t.Fatalf("SubmitTaskDiscriminator() = %q, want TF79B3E36", got)
	}
	if got := SubmitRequestDiscriminator("f898d3ad-b007"); got != "RF898D3AD" {
		t.Fatalf("SubmitRequestDiscriminator() = %q, want RF898D3AD", got)
	}
	if got := CombineSubmitDiscriminators(" TF79B3E36 ", "", "RF898D3AD"); got != "TF79B3E36-RF898D3AD" {
		t.Fatalf("CombineSubmitDiscriminators() = %q, want joined discriminators", got)
	}
}

func TestNormalizeSubmitStyleSuffix(t *testing.T) {
	t.Parallel()

	if got := NormalizeSubmitStyleSuffix(" d7-e68190-extra "); got != "D7E68190" {
		t.Fatalf("NormalizeSubmitStyleSuffix() = %q, want D7E68190", got)
	}
}
