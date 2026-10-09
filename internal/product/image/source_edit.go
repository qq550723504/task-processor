package image

import (
	"context"
	"strings"
)

const SourceEditOperation = "render_source_edit"

// SourceEditRequest edits exact original references into one candidate. Sources
// carry provenance; the separate in-process bytes never enter public metadata.
type SourceEditRequest struct {
	Sources        []Asset
	ReferenceBytes [][]byte `json:"-"`
	Product        ProductContext
	Prompt         string
	PromptVersion  string
	Authorization  *UsageQuote
}

type SourceEditor interface {
	EditSources(context.Context, SourceEditRequest) (Candidate, error)
}
type sourceEditCapability struct{ backend SourceEditor }

func NewSourceEditCapability(backend SourceEditor) (SourceEditor, error) {
	if isNilCapability(backend) {
		return nil, ErrInputInvalid
	}
	return &sourceEditCapability{backend: backend}, nil
}

func (c *sourceEditCapability) EditSources(ctx context.Context, request SourceEditRequest) (Candidate, error) {
	if err := contextError(ctx); err != nil {
		return Candidate{}, err
	}
	cloned, err := CloneSourceEditRequest(request)
	if err != nil {
		return Candidate{}, err
	}
	candidate, err := c.backend.EditSources(ctx, cloned)
	if contextErr := contextError(ctx); contextErr != nil {
		return Candidate{}, contextErr
	}
	if err != nil {
		return Candidate{}, capabilityError(err)
	}
	return validateCandidate(candidate, cloned.Sources[0], RoleScene, SourceEditOperation, forbiddenArtifactURLs(cloned.Sources...))
}

// CloneSourceEditRequest is also used at concrete adapter boundaries so callers
// cannot bypass the exact-reference bounds by invoking an adapter directly.
func CloneSourceEditRequest(request SourceEditRequest) (SourceEditRequest, error) {
	if len(request.Sources) < 1 || len(request.Sources) > 8 || len(request.Sources) != len(request.ReferenceBytes) || request.Prompt == "" || strings.TrimSpace(request.Prompt) != request.Prompt || len(request.Prompt) > 64<<10 || !isCanonicalRequired(request.PromptVersion) || len(request.PromptVersion) > 128 {
		return SourceEditRequest{}, ErrInputInvalid
	}
	product, err := validateProductContext(request.Product)
	if err != nil {
		return SourceEditRequest{}, err
	}
	authorization, err := cloneUsageAuthorization(request.Authorization, SourceEditOperation)
	if err != nil || authorization == nil || authorization.MaximumModelCalls != 1 || authorization.MaximumOutputs != 1 {
		return SourceEditRequest{}, ErrInputInvalid
	}
	sources := make([]Asset, len(request.Sources))
	references := make([][]byte, len(request.Sources))
	seen := map[string]bool{}
	total := 0
	for i, source := range request.Sources {
		validated, err := validateSourceAsset(source)
		if err != nil || seen[source.SourceAssetID] || len(request.ReferenceBytes[i]) == 0 || len(request.ReferenceBytes[i]) > 16<<20 {
			return SourceEditRequest{}, ErrInputInvalid
		}
		if source.MediaType != "image/png" && source.MediaType != "image/jpeg" && source.MediaType != "image/webp" {
			return SourceEditRequest{}, ErrInputInvalid
		}
		total += len(request.ReferenceBytes[i])
		if total > 16<<20 {
			return SourceEditRequest{}, ErrInputInvalid
		}
		seen[source.SourceAssetID] = true
		sources[i] = validated
		references[i] = append([]byte(nil), request.ReferenceBytes[i]...)
	}
	return SourceEditRequest{Sources: sources, ReferenceBytes: references, Product: product, Prompt: request.Prompt, PromptVersion: request.PromptVersion, Authorization: authorization}, nil
}
