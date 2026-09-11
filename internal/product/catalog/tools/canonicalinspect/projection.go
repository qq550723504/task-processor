package canonicalinspect

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"task-processor/internal/product/catalog"
)

const MaxOutputBytes = 1 << 20

var ErrProjectionTooLarge = errors.New("canonical inspection projection exceeds size limit")

type Input struct {
	ProductKey     string `json:"product_key"`
	CatalogVersion string `json:"catalog_version"`
}

type Diagnostics struct {
	NeedsReview   bool              `json:"needs_review"`
	ReviewReasons []string          `json:"review_reasons"`
	Warnings      []catalog.Warning `json:"warnings"`
}

type Output struct {
	ProductKey           string                  `json:"product_key"`
	CatalogVersion       string                  `json:"catalog_version"`
	CatalogPublicationID string                  `json:"catalog_publication_id"`
	Snapshot             catalog.ProductSnapshot `json:"snapshot"`
	Diagnostics          Diagnostics             `json:"diagnostics"`
}

func Project(published catalog.PublishedSnapshot) (json.RawMessage, error) {
	cloned, err := catalog.CloneProductSnapshot(published.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("clone canonical snapshot: %w", err)
	}
	removeSourceMetadata(&cloned)

	diagnostics := Diagnostics{ReviewReasons: []string{}, Warnings: []catalog.Warning{}}
	if cloned.Review != nil {
		diagnostics.NeedsReview = cloned.Review.NeedsReview
		diagnostics.ReviewReasons = append([]string(nil), cloned.Review.Reasons...)
		if diagnostics.ReviewReasons == nil {
			diagnostics.ReviewReasons = []string{}
		}
	}
	diagnostics.Warnings = append([]catalog.Warning(nil), cloned.Warnings...)
	if diagnostics.Warnings == nil {
		diagnostics.Warnings = []catalog.Warning{}
	}

	encoded, err := json.Marshal(Output{
		ProductKey: published.Identity.ProductKey, CatalogVersion: strconv.FormatUint(published.Version, 10),
		CatalogPublicationID: published.PublicationID, Snapshot: cloned, Diagnostics: diagnostics,
	})
	if err != nil {
		return nil, fmt.Errorf("encode canonical inspection projection: %w", err)
	}
	if len(encoded) > MaxOutputBytes {
		return nil, ErrProjectionTooLarge
	}
	return encoded, nil
}

func removeSourceMetadata(snapshot *catalog.ProductSnapshot) {
	if snapshot == nil {
		return
	}
	clearSourceRecords(snapshot.Sources)
	for index := range snapshot.Attributes {
		clearTrace(&snapshot.Attributes[index].Trace)
	}
	for index := range snapshot.Images {
		clearTrace(&snapshot.Images[index].Trace)
	}
	for variantIndex := range snapshot.Variants {
		variant := &snapshot.Variants[variantIndex]
		clearTrace(&variant.Trace)
		for attributeIndex := range variant.Attributes {
			clearTrace(&variant.Attributes[attributeIndex].Trace)
		}
		for imageIndex := range variant.Images {
			clearTrace(&variant.Images[imageIndex].Trace)
		}
	}
}

func clearTrace(trace *catalog.Trace) {
	if trace != nil {
		clearSourceRecords(trace.Sources)
	}
}

func clearSourceRecords(records []catalog.SourceRecord) {
	for index := range records {
		records[index].Metadata = nil
	}
}
