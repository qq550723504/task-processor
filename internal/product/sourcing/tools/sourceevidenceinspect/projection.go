package sourceevidenceinspect

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/product/sourcing"
)

const MaxOutputBytes = 64 << 10
const MaxCatalogSnapshotBytes = sourcing.MaxEncodedSnapshotBytes

var ErrProjectionTooLarge = errors.New("source evidence projection exceeds size limit")
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var decimalSourceID = regexp.MustCompile(`^[1-9][0-9]{0,31}$`)

type PublicIdentity struct {
	SourceType        string `json:"source_type,omitempty"`
	SourcePlatform    string `json:"source_platform,omitempty"`
	SourceID          string `json:"source_id,omitempty"`
	SourceFingerprint string `json:"source_fingerprint,omitempty"`
}
type Producer struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
}
type Lineage struct {
	InputHash    string `json:"input_hash"`
	EnvelopeHash string `json:"envelope_hash"`
	SnapshotHash string `json:"snapshot_hash"`
}
type Capture struct {
	CapturedAt *time.Time `json:"captured_at,omitempty"`
	Checksum   string     `json:"checksum,omitempty"`
}
type Warning struct {
	Kind  string `json:"kind"`
	Field string `json:"field,omitempty"`
}
type MissingFact struct {
	Field string `json:"field"`
}
type Disclosure struct {
	RawTextReturned       bool     `json:"raw_text_returned"`
	SourceURLProvided     bool     `json:"source_url_provided"`
	DiagnosticsPartial    bool     `json:"diagnostics_partial"`
	WarningCodesOmitted   int      `json:"warning_codes_omitted"`
	WarningsTotal         int      `json:"warnings_total"`
	MissingFactsTotal     int      `json:"missing_facts_total"`
	WarningFieldsOmitted  int      `json:"warning_fields_omitted"`
	MissingFactsOmitted   int      `json:"missing_facts_omitted"`
	IdentityFieldsOmitted []string `json:"identity_fields_omitted"`
	CaptureFieldsOmitted  []string `json:"capture_fields_omitted"`
}
type Output struct {
	ProductKey           string         `json:"product_key"`
	CatalogVersion       string         `json:"catalog_version"`
	PublicationID        string         `json:"publication_id"`
	CatalogPublicationID string         `json:"catalog_publication_id"`
	Producer             Producer       `json:"producer"`
	PublishedAt          time.Time      `json:"published_at"`
	SourceIdentity       PublicIdentity `json:"source_identity"`
	Lineage              Lineage        `json:"lineage"`
	Capture              Capture        `json:"capture"`
	Warnings             []Warning      `json:"warnings"`
	MissingFacts         []MissingFact  `json:"missing_facts"`
	Disclosure           Disclosure     `json:"disclosure"`
}

// Project copies only the approved structured publication references and
// classifications. It never copies arbitrary source text, URLs, metadata or
// source trace fields. Identifier syntax limits an approved identifier class;
// it is not a secret detector. Unknown diagnostic values are omitted honestly.
func Project(p sourcing.PersistedPublication) (json.RawMessage, error) {
	r := p.Receipt
	for _, id := range []string{r.ProductKey, r.PublicationID, r.CatalogPublicationID, r.Producer.Kind, r.Producer.Version} {
		if !authidentity.IsBoundedIdentifier(id) {
			return nil, sourcing.ErrSourcePublicationStateInvalid
		}
	}
	if r.PublicationID != r.CatalogPublicationID || r.CatalogVersion == 0 || r.CatalogVersion > math.MaxInt64 || r.PublishedAt.IsZero() || !digest.MatchString(r.InputHash) || !digest.MatchString(r.EnvelopeHash) || !digest.MatchString(r.SnapshotHash) {
		return nil, sourcing.ErrSourcePublicationStateInvalid
	}
	if len(p.Envelope.Warnings) > sourcing.MaxSourceEnvelopeCollectionItems || len(p.Envelope.MissingFacts) > sourcing.MaxSourceEnvelopeCollectionItems {
		return nil, ErrProjectionTooLarge
	}
	o := Output{ProductKey: r.ProductKey, CatalogVersion: strconv.FormatUint(r.CatalogVersion, 10), PublicationID: r.PublicationID, CatalogPublicationID: r.CatalogPublicationID,
		Producer: Producer{r.Producer.Kind, r.Producer.Version}, PublishedAt: r.PublishedAt.UTC(), Lineage: Lineage{r.InputHash, r.EnvelopeHash, r.SnapshotHash},
		Warnings: []Warning{}, MissingFacts: []MissingFact{}, Disclosure: Disclosure{IdentityFieldsOmitted: []string{}, CaptureFieldsOmitted: []string{}}}
	id := p.Envelope.Identity
	// Only the explicitly disclosed controlled 1688 product identifier category
	// is returned. Other neutral sources remain readable through their exact
	// publication binding, with their source identifier fields marked omitted.
	admittedIdentity := r.Producer.Kind == sourcing.ControlledSnapshotProducerKind && r.Producer.Version == sourcing.ControlledSnapshotProducerVersion && id.SourceType == sourcing.SourceTypeCrawler && id.SourcePlatform == "1688"
	if admittedIdentity {
		o.SourceIdentity.SourceType = id.SourceType
		o.SourceIdentity.SourcePlatform = id.SourcePlatform
	}
	for _, field := range []struct {
		name, value string
		allowed     bool
	}{{"source_type", id.SourceType, admittedIdentity}, {"source_platform", id.SourcePlatform, admittedIdentity}, {"source_id", id.SourceID, admittedIdentity && decimalSourceID.MatchString(id.SourceID)}} {
		if !field.allowed && field.value != "" {
			o.Disclosure.IdentityFieldsOmitted = append(o.Disclosure.IdentityFieldsOmitted, field.name)
		}
	}
	if admittedIdentity && decimalSourceID.MatchString(id.SourceID) {
		o.SourceIdentity.SourceID = id.SourceID
	}
	if digest.MatchString(id.SourceFingerprint) {
		o.SourceIdentity.SourceFingerprint = id.SourceFingerprint
	} else if id.SourceFingerprint != "" {
		o.Disclosure.IdentityFieldsOmitted = append(o.Disclosure.IdentityFieldsOmitted, "source_fingerprint")
	}
	if !p.Envelope.RawReference.CapturedAt.IsZero() {
		captured := p.Envelope.RawReference.CapturedAt.UTC()
		o.Capture.CapturedAt = &captured
	}
	checksum := p.Envelope.RawReference.Checksum
	if strings.HasPrefix(checksum, "sha256:") && digest.MatchString(strings.TrimPrefix(checksum, "sha256:")) {
		o.Capture.Checksum = checksum
	} else if checksum != "" {
		o.Disclosure.CaptureFieldsOmitted = append(o.Disclosure.CaptureFieldsOmitted, "checksum")
	}
	for _, warning := range p.Envelope.Warnings {
		// controlled_snapshot/v1 has no closed diagnostic-code vocabulary. A
		// record's presence is known; its arbitrary code is never public authority.
		if warning.Code != "" {
			o.Disclosure.WarningCodesOmitted++
		}
		w := Warning{Kind: "source_warning"}
		if publicField(warning.Field) {
			w.Field = warning.Field
		} else if warning.Field != "" {
			o.Disclosure.WarningFieldsOmitted++
		}
		o.Warnings = append(o.Warnings, w)
	}
	for _, missing := range p.Envelope.MissingFacts {
		if !publicField(missing.Field) {
			o.Disclosure.MissingFactsOmitted++
			continue
		}
		o.MissingFacts = append(o.MissingFacts, MissingFact{Field: missing.Field})
	}
	o.Disclosure.WarningsTotal = len(p.Envelope.Warnings)
	o.Disclosure.MissingFactsTotal = len(p.Envelope.MissingFacts)
	o.Disclosure.DiagnosticsPartial = o.Disclosure.WarningCodesOmitted+o.Disclosure.WarningFieldsOmitted+o.Disclosure.MissingFactsOmitted > 0
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, sourcing.ErrSourcePublicationStateInvalid
	}
	if len(raw) > MaxOutputBytes {
		return nil, ErrProjectionTooLarge
	}
	return raw, nil
}

// Field names refer only to current source candidate facts; legacy producer
// aliases and arbitrary provider DTO keys are not mapped or disclosed.
func publicField(field string) bool {
	switch field {
	case "title", "images", "brand", "description", "category_path", "attributes", "variants", "price", "stock", "currency":
		return true
	}
	return false
}
