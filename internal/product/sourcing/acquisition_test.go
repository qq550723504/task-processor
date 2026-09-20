package sourcing

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func acquisitionString(value string) *string { return &value }

func TestAcquisitionRawLengthIsCheckedBeforeNormalization(t *testing.T) {
	for _, field := range []string{"title", "sku"} {
		e := acquisitionEvidenceFixture()
		raw := strings.Repeat(" ", 8192) + "x"
		if field == "title" {
			e.Title = &raw
		} else {
			e.Variants[0].SKU = &raw
		}
		_, err := MapAcquisitionEvidence(AcquisitionSource{OfferID: e.OfferID, URL: e.SourceURL}, e, "public_http", "raw-length")
		require.ErrorIs(t, err, ErrSourcePublicationTooLarge, field)
	}
}

func TestAcquisitionPrecisionLossRetainsEvidenceWithoutFalseCatalogPrice(t *testing.T) {
	e := acquisitionEvidenceFixture()
	e.Variants[0].Price.Amount = "9007199254740993"
	envelope, err := MapAcquisitionEvidence(AcquisitionSource{OfferID: e.OfferID, URL: e.SourceURL}, e, "public_http", "precision")
	require.NoError(t, err)
	require.Zero(t, envelope.ProductCandidate.Variants[0].Price)
	require.Equal(t, "9007199254740993", envelope.ProductCandidate.Variants[0].Attributes["source.price.amount"])
	require.Contains(t, envelope.ProductCandidate.Variants[0].Attributes["source.price.fact"], `"currency":"CNY"`)
	found := false
	for _, warning := range envelope.Warnings {
		if warning.Code == "source_price_precision" {
			found = true
		}
	}
	require.True(t, found)
	_, err = Canonical1688Source("https://detail.1688.com:/offer/123.html")
	require.Error(t, err)
}

func acquisitionEvidenceFixture() AcquisitionEvidence {
	return AcquisitionEvidence{
		SchemaVersion: 1, OfferID: "981645030344", SourceURL: "https://detail.1688.com/offer/981645030344.html",
		Title: acquisitionString("Business fixture bottle"), Description: acquisitionString("Stainless steel bottle"),
		Attributes: []AcquisitionAttribute{{Name: "material", Value: "steel"}},
		Variants:   []AcquisitionVariant{{SourceID: acquisitionString("sku-1"), SKU: acquisitionString("BOTTLE-RED"), Attributes: []AcquisitionAttribute{{Name: "color", Value: "red"}}, Price: &AcquisitionPrice{Amount: "12.50", Currency: acquisitionString("CNY")}}},
		Images:     []AcquisitionImage{{URL: "https://cbu01.alicdn.com/fixture-bottle.jpg", Role: "primary"}},
		CapturedAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), ContentSHA256: strings.Repeat("a", 64), ParserVersion: "1688-context-v1",
	}
}

func TestAcquisitionCanonicalIdentityAcrossEquivalentInputs(t *testing.T) {
	for _, input := range []string{"981645030344", "https://detail.1688.com/offer/981645030344.html", "http://DETAIL.1688.com/offer/981645030344.html?tracking=one#details"} {
		source, err := Canonical1688Source(input)
		require.NoError(t, err, input)
		require.Equal(t, AcquisitionSource{OfferID: "981645030344", URL: "https://detail.1688.com/offer/981645030344.html"}, source)
		var previous string
		for _, channel := range []string{"public_http", "browser_capture"} {
			envelope, err := MapAcquisitionEvidence(source, acquisitionEvidenceFixture(), channel, "operation-1")
			require.NoError(t, err)
			key, _, err := PublicationIdentity(envelope)
			require.NoError(t, err)
			require.Equal(t, "crawler:1688:981645030344", key)
			if previous != "" {
				require.Equal(t, previous, key)
			}
			previous = key
			require.Empty(t, envelope.Identity.Platform)
			require.Empty(t, envelope.Identity.ProductID)
		}
	}
}

func TestAcquisitionRejectsForeignAmbiguousAndOversizedInputs(t *testing.T) {
	for _, input := range []string{"0", "00123", "-1", "1e3", "https://detail.1688.com.evil.test/offer/12.html", "https://user@detail.1688.com/offer/12.html", "https://detail.1688.com:8443/offer/12.html", "https://detail.1688.com./offer/12.html", "https://detail.1688.com/offer/%31%32.html", "https://detail.1688.com/offer/12.html/../13.html", "file:///offer/12.html", "https://detail.1688.com/offer/12.html?bad=%xx", strings.Repeat("1", 2049)} {
		_, err := Canonical1688Source(input)
		require.Error(t, err, input)
	}
}

func TestAcquisitionMappingPreservesMissingFactsWithoutInventedDefaults(t *testing.T) {
	evidence := acquisitionEvidenceFixture()
	evidence.Variants = nil
	evidence.Images = nil
	evidence.Description = nil
	source := AcquisitionSource{OfferID: evidence.OfferID, URL: evidence.SourceURL}
	envelope, err := MapAcquisitionEvidence(source, evidence, "public_http", "operation-2")
	require.NoError(t, err)
	require.Empty(t, envelope.ProductCandidate.Variants)
	require.Empty(t, envelope.AssetCandidates)
	require.Empty(t, envelope.ProductCandidate.Description)
	require.Empty(t, envelope.SupplierOrCostFacts.Currency)
	fields := map[string]bool{}
	for _, missing := range envelope.MissingFacts {
		fields[missing.Field] = true
	}
	for _, field := range []string{"variants", "images", "description", "price"} {
		require.True(t, fields[field], field)
	}
	require.NotEmpty(t, envelope.Warnings)
}

func TestAcquisitionRejectsUntrustedMismatchesAndMalformedFacts(t *testing.T) {
	for name, mutate := range map[string]func(*AcquisitionEvidence){
		"foreign offer":       func(e *AcquisitionEvidence) { e.OfferID = "99" },
		"foreign URL":         func(e *AcquisitionEvidence) { e.SourceURL = "https://evil.test/" },
		"schema":              func(e *AcquisitionEvidence) { e.SchemaVersion = 2 },
		"missing title":       func(e *AcquisitionEvidence) { e.Title = nil },
		"duplicate attribute": func(e *AcquisitionEvidence) { e.Attributes = append(e.Attributes, e.Attributes[0]) },
		"malformed price":     func(e *AcquisitionEvidence) { e.Variants[0].Price.Amount = "NaN" },
		"negative price":      func(e *AcquisitionEvidence) { e.Variants[0].Price.Amount = "-2" },
		"private image":       func(e *AcquisitionEvidence) { e.Images[0].URL = "https://127.0.0.1/a.jpg" },
		"oversized title":     func(e *AcquisitionEvidence) { e.Title = acquisitionString(strings.Repeat("a", 8193)) },
	} {
		t.Run(name, func(t *testing.T) {
			evidence := acquisitionEvidenceFixture()
			source := AcquisitionSource{OfferID: evidence.OfferID, URL: evidence.SourceURL}
			mutate(&evidence)
			_, err := MapAcquisitionEvidence(source, evidence, "public_http", "operation-3")
			require.Error(t, err)
		})
	}
}
