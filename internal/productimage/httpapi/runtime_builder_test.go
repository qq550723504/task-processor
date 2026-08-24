package httpapi

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	productenrich "task-processor/internal/productenrich"
)

func TestBuildModuleFailsClosedWhenTypedGovernanceOptionsLackTenantRouting(t *testing.T) {
	t.Parallel()

	_, err := BuildModule(newRuntimeBuildInputWithoutTenantRouting(t))

	require.EqualError(t, err, "create productimage model provider: productimage scene governance requires a resolver-backed image client")
}

func newRuntimeBuildInputWithoutTenantRouting(t *testing.T) BuildModuleInput {
	t.Helper()

	return BuildModuleInput{
		Logger:        logrus.New(),
		InputParser:   runtimeInputParserStub{},
		Understanding: runtimeProductUnderstandingStub{},
		ImageWorkDir:  t.TempDir(),
		Options: productImageRuntimeOptions{
			requireAIIdentity: true,
			modelProvider: modelProviderOptions{
				sceneGovernanceEnabled: true,
			},
			sceneGovernance: sceneGovernanceOptions{enabled: true},
		},
	}
}

type runtimeInputParserStub struct{}

func (runtimeInputParserStub) ParseInput(context.Context, *productenrich.GenerateRequest) (*productenrich.ParsedInput, error) {
	return nil, nil
}

func (runtimeInputParserStub) CollectImages(context.Context, []string) ([]string, error) {
	return nil, nil
}

func (runtimeInputParserStub) CleanText(text string) string { return text }

func (runtimeInputParserStub) Scrape1688(context.Context, string) (*productenrich.ScrapedData, error) {
	return nil, nil
}

type runtimeProductUnderstandingStub struct{}

func (runtimeProductUnderstandingStub) AnalyzeProduct(context.Context, *productenrich.ParsedInput) (*productenrich.ProductAnalysis, error) {
	return nil, nil
}

func (runtimeProductUnderstandingStub) AnalyzeImage(context.Context, string) (*productenrich.ImageAttributes, error) {
	return nil, nil
}

func (runtimeProductUnderstandingStub) ExtractTextAttributes(context.Context, string) (*productenrich.TextAttributes, error) {
	return nil, nil
}

func (runtimeProductUnderstandingStub) FuseMultimodal(context.Context, *productenrich.ImageAttributes, *productenrich.TextAttributes) (*productenrich.ProductRepresentation, error) {
	return nil, nil
}
