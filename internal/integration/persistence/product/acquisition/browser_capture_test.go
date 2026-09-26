package acquisition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"task-processor/internal/product/sourcing"
)

func browserStagingOperation(t *testing.T) (sourcing.AcquisitionOperation, sourcing.PublicationCommand) {
	t.Helper()
	body, err := os.ReadFile("../../../../product/sourcing/testdata/browser-capture-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := sourcing.ParseBrowserCapture(body)
	if err != nil {
		t.Fatal(err)
	}
	op := sourcing.AcquisitionOperation{Scope: sourcing.PublicationScope{OrganizationID: "browser-org", ActorID: "browser-actor"}, Key: "12aa5bfc-669c-47b6-af07-561178f9c149", Source: capture.Source, CaptureSHA256: capture.PayloadSHA256}
	identity, _ := json.Marshal([]string{op.Scope.OrganizationID, op.Scope.ActorID, op.Key})
	op.ID = uuid.NewSHA1(uuid.NameSpaceURL, identity).String()
	op.Fingerprint, err = sourcing.BrowserAcquisitionFingerprint(op.Source, op.CaptureSHA256)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := sourcing.MapAcquisitionEvidence(op.Source, capture.Evidence, "browser_capture", op.ID)
	if err != nil {
		t.Fatal(err)
	}
	envelope.RawReference.Metadata["capture_sha256"] = capture.PayloadSHA256
	productKey, publicationID, err := sourcing.PublicationIdentity(envelope)
	if err != nil {
		t.Fatal(err)
	}
	base := uint64(0)
	return op, sourcing.PublicationCommand{PublicationID: publicationID, ProductKey: productKey, Producer: sourcing.ProducerDescriptor{Kind: sourcing.BrowserAcquisitionProducerKind, Version: "v1"}, ExpectedBaseVersion: &base, Envelope: envelope}
}

func TestBrowserCaptureStagingIdentityAndCommand(t *testing.T) {
	op, command := browserStagingOperation(t)
	if !validIdentity(op) {
		t.Fatal("server-derived Browser intent rejected")
	}
	if !validCommand(op, command) {
		t.Fatal("bound Browser frozen command rejected")
	}
	public := op
	public.CaptureSHA256 = ""
	input, _ := json.Marshal([]string{sourcing.AcquisitionContractVersion, "acquire", public.Source.URL})
	hash := sha256.Sum256(input)
	public.Fingerprint = hex.EncodeToString(hash[:])
	if !validIdentity(public) || public.ID != op.ID {
		t.Fatal("Public identity bytes or shared operation ID changed")
	}
	if public.Fingerprint == op.Fingerprint {
		t.Fatal("cross-action intents share a fingerprint")
	}
}

func TestBrowserCaptureStagingRejectsUnboundIntent(t *testing.T) {
	for _, mode := range []string{"missing digest", "short digest", "uppercase digest", "changed digest", "changed fingerprint", "changed source"} {
		t.Run(mode, func(t *testing.T) {
			op, _ := browserStagingOperation(t)
			switch mode {
			case "missing digest":
				op.CaptureSHA256 = ""
			case "short digest":
				op.CaptureSHA256 = "abcd"
			case "uppercase digest":
				op.CaptureSHA256 = strings.ToUpper(op.CaptureSHA256)
			case "changed digest":
				op.CaptureSHA256 = strings.Repeat("a", 64)
			case "changed fingerprint":
				op.Fingerprint = strings.Repeat("a", 64)
			case "changed source":
				op.Source, _ = sourcing.Canonical1688Source("981645030345")
			}
			if validIdentity(op) {
				t.Fatal("unbound Browser intent accepted")
			}
		})
	}
}

func TestBrowserCaptureStagingRejectsUnboundCommand(t *testing.T) {
	for _, mode := range []string{"producer", "version", "channel", "parser", "contract", "digest", "missing digest"} {
		t.Run(mode, func(t *testing.T) {
			op, command := browserStagingOperation(t)
			switch mode {
			case "producer":
				command.Producer.Kind = sourcing.AcquisitionProducerKind
			case "version":
				command.Producer.Version = "v2"
			case "channel":
				command.Envelope.RawReference.Metadata["channel"] = "public_http"
			case "parser":
				command.Envelope.RawReference.Metadata["parser_version"] = "wrong/v1"
			case "contract":
				command.Envelope.RawReference.Metadata["contract_version"] = "wrong/v1"
			case "digest":
				command.Envelope.RawReference.Metadata["capture_sha256"] = strings.Repeat("a", 64)
			case "missing digest":
				delete(command.Envelope.RawReference.Metadata, "capture_sha256")
			}
			if validCommand(op, command) {
				t.Fatal("unbound frozen command accepted")
			}
		})
	}
}
