package acquisition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/product/sourcing"
)

func publicBrowserStagingOperation(t *testing.T, channel string) (sourcing.AcquisitionOperation, sourcing.PublicationCommand) {
	t.Helper()
	source, err := sourcing.Canonical1688Source("981645030344")
	if err != nil {
		t.Fatal(err)
	}
	op := sourcing.AcquisitionOperation{
		Scope:  sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"},
		Key:    "8a2b1c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d",
		Source: source,
		// public_browser keeps the content-independent, empty CaptureSHA256
		// fingerprint exactly like public_http (design D1/D11).
		CaptureSHA256: "",
	}
	identity, _ := json.Marshal([]string{op.Scope.OrganizationID, op.Scope.ActorID, op.Key})
	op.ID = uuid.NewSHA1(uuid.NameSpaceURL, identity).String()
	input, _ := json.Marshal([]string{sourcing.AcquisitionContractVersion, "acquire", op.Source.URL})
	sum := sha256.Sum256(input)
	op.Fingerprint = hex.EncodeToString(sum[:])

	amount, currency, sourceID, title := "12.50", "CNY", "sku-1", "Browser fixture bottle"
	evidence := sourcing.AcquisitionEvidence{
		SchemaVersion: 1, OfferID: source.OfferID, SourceURL: source.URL,
		Title:        &title,
		Attributes:   []sourcing.AcquisitionAttribute{{Name: "material", Value: "steel"}},
		Variants:     []sourcing.AcquisitionVariant{{SourceID: &sourceID, Price: &sourcing.AcquisitionPrice{Amount: amount, Currency: &currency}}},
		Images:       []sourcing.AcquisitionImage{{URL: "https://cbu01.alicdn.com/fixture-bottle.jpg", Role: "primary"}},
		CapturedAt:   time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		ContentSHA256: strings.Repeat("b", 64), ParserVersion: "1688-browser-dom/v1",
	}
	envelope, err := sourcing.MapAcquisitionEvidence(op.Source, evidence, channel, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
	if err != nil {
		t.Fatal(err)
	}
	base := uint64(0)
	command := sourcing.PublicationCommand{
		PublicationID: publicationID, ProductKey: productKey,
		Producer:            sourcing.ProducerDescriptor{Kind: sourcing.AcquisitionProducerKind, Version: "v1"},
		ExpectedBaseVersion: &base, Envelope: envelope,
	}
	return op, command
}

func TestValidCommandAdmitsPublicBrowserCommand(t *testing.T) {
	op, command := publicBrowserStagingOperation(t, sourcing.AcquisitionChannelPublicBrowser)
	if !validCommand(op, command) {
		t.Fatal("public_browser command must be admitted by validCommand")
	}
	if command.Producer.Kind != sourcing.AcquisitionProducerKind {
		t.Fatalf("public_browser must not introduce a new producer kind, got %q", command.Producer.Kind)
	}
	if op.CaptureSHA256 != "" {
		t.Fatalf("public_browser keeps CaptureSHA256 empty, got %q", op.CaptureSHA256)
	}
	if command.Envelope.RawReference.Metadata["capture_sha256"] != "" {
		t.Fatal("public_browser command must not carry capture_sha256 metadata")
	}
	if got := command.Envelope.RawReference.Metadata["channel"]; got != sourcing.AcquisitionChannelPublicBrowser {
		t.Fatalf("channel metadata = %q", got)
	}
}

func TestValidCommandRejectsPublicBrowserOutsideAnonymousPublicFamily(t *testing.T) {
	// A public_browser command must not claim a capture digest: that would route
	// it into the browser_acquisition validation branch, which requires a
	// browser_capture parser version and a matching op.CaptureSHA256.
	op, command := publicBrowserStagingOperation(t, sourcing.AcquisitionChannelPublicBrowser)
	command.Envelope.RawReference.Metadata["capture_sha256"] = strings.Repeat("c", 64)
	if validCommand(op, command) {
		t.Fatal("public_browser command carrying capture_sha256 must be rejected")
	}

	// An unknown channel must be rejected, not defaulted.
	for _, forged := range []string{"public", "browser", "public_browser_v2", ""} {
		op, command := publicBrowserStagingOperation(t, sourcing.AcquisitionChannelPublicBrowser)
		command.Envelope.RawReference.Metadata["channel"] = forged
		if validCommand(op, command) {
			t.Fatalf("unknown channel %q must be rejected", forged)
		}
	}

	// The operation fingerprint is intentionally content- and channel-independent,
	// so public_http and public_browser commands for the same (scope, key, source)
	// are the same admitted intent. Channel is server-set diagnostic metadata, not
	// caller input, so cross-relabeling between the two anonymous public channels
	// is neither a security boundary nor an idempotency violation.
	opHTTP, commandHTTP := publicBrowserStagingOperation(t, sourcing.AcquisitionChannelPublicHTTP)
	opBrowser, commandBrowser := publicBrowserStagingOperation(t, sourcing.AcquisitionChannelPublicBrowser)
	if opHTTP.ID != opBrowser.ID || opHTTP.Fingerprint != opBrowser.Fingerprint {
		t.Fatal("public_http and public_browser must share the same operation identity")
	}
	if !validCommand(opHTTP, commandHTTP) || !validCommand(opBrowser, commandBrowser) {
		t.Fatal("both anonymous public channels must be admitted")
	}
}
