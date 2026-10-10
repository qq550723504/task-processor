package image

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

type sourceEditorFunc func(context.Context, SourceEditRequest) (Candidate, error)

func (f sourceEditorFunc) EditSources(ctx context.Context, r SourceEditRequest) (Candidate, error) {
	return f(ctx, r)
}

func validSourceEditRequest() SourceEditRequest {
	second := validSourceAsset()
	second.MediaType = "image/png"
	second.SourceAssetID = "source-2"
	second.URL = "https://example.com/two.png"
	second.SourceURL = second.URL
	first := validSourceAsset()
	first.MediaType = "image/png"
	return SourceEditRequest{Sources: []Asset{first, second}, ReferenceBytes: [][]byte{[]byte("one"), []byte("two")}, Product: validProductContext(), Prompt: "preserve actual product, show its details", PromptVersion: "source-edit-v1", Authorization: &UsageQuote{Operation: SourceEditOperation, Provider: "grsai", RouteReference: "route", Model: "gpt-image-2.5", CredentialReference: "credential", ConfigurationVersion: "version", PricingVersion: "price", Fingerprint: "fingerprint", MaximumOutputs: 1, MaximumModelCalls: 1}}
}

func TestSourceEditCapabilityCopiesEveryReferenceAndRequiresSingleCall(t *testing.T) {
	request := validSourceEditRequest()
	var received SourceEditRequest
	capability, err := NewSourceEditCapability(sourceEditorFunc(func(_ context.Context, r SourceEditRequest) (Candidate, error) {
		received = r
		return Candidate{Asset: validInlineGeneratedAsset(RoleScene, SourceEditOperation, []byte("generated"))}, nil
	}))
	require.NoError(t, err)
	_, err = capability.EditSources(context.Background(), request)
	require.NoError(t, err)
	request.ReferenceBytes[1][0] = 'X'
	request.Sources[1].Operations[0] = "changed"
	require.Equal(t, []byte("two"), received.ReferenceBytes[1])
	require.NotEqual(t, "changed", received.Sources[1].Operations[0])
	for _, mode := range []string{"authorization", "calls", "sourceCount", "missingBytes", "duplicate", "prompt"} {
		t.Run(mode, func(t *testing.T) {
			bad := validSourceEditRequest()
			switch mode {
			case "authorization":
				bad.Authorization = nil
			case "calls":
				bad.Authorization.MaximumModelCalls = 2
			case "sourceCount":
				bad.Sources = bad.Sources[:1]
			case "missingBytes":
				bad.ReferenceBytes[1] = nil
			case "duplicate":
				bad.Sources[1] = bad.Sources[0]
			case "prompt":
				bad.Prompt = ""
			}
			_, err := capability.EditSources(context.Background(), bad)
			require.ErrorIs(t, err, ErrInputInvalid)
		})
	}
}

func TestSourceEditRejectsPassThroughOfAnyOriginal(t *testing.T) {
	request := validSourceEditRequest()
	capability, err := NewSourceEditCapability(sourceEditorFunc(func(_ context.Context, r SourceEditRequest) (Candidate, error) {
		asset := validInlineGeneratedAsset(RoleScene, SourceEditOperation, []byte("generated"))
		asset.Bytes = nil
		asset.URL = r.Sources[1].URL
		return Candidate{Asset: asset}, nil
	}))
	require.NoError(t, err)
	_, err = capability.EditSources(context.Background(), request)
	require.ErrorIs(t, err, ErrOutputValidation)
}
