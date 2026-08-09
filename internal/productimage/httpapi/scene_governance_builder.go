package httpapi

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	productimage "task-processor/internal/productimage"
)

func buildGovernedProductImageSceneGenerator(cfg *config.Config, legacy productimage.SceneGenerator, resolver openaiclient.ClientConfigResolver, recorder aicapability.InvocationRecorder, logger *logrus.Logger) (productimage.SceneGenerator, error) {
	if cfg == nil || !cfg.AICapability.ProductImageSceneEnabled {
		return legacy, nil
	}
	if legacy == nil || resolver == nil || recorder == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductImageSceneGenerate), nil)
	}
	routed, ok := legacy.(productimage.SceneGeneratorWithRoute)
	if !ok {
		return nil, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductImageSceneGenerate), nil)
	}
	return productimage.NewGovernedSceneGenerator(productimage.GovernedSceneGeneratorConfig{
		Router:   BuildProductImageSceneCapabilityRouter(resolver),
		Recorder: recorder,
		Provider: routed,
		Identity: func(ctx context.Context) productimage.SceneAIIdentity {
			identity := productimage.AIIdentityFromContext(ctx)
			return productimage.SceneAIIdentity{TenantID: identity.TenantID, UserID: identity.UserID}
		},
		OnRecordError: func(record aicapability.InvocationRecord, err error) {
			if logger != nil {
				logger.WithError(err).WithFields(logrus.Fields{
					"invocation_id": string(record.InvocationID),
					"capability":    string(record.Capability),
					"operation":     string(record.Operation),
				}).Warn("ai invocation ledger write failed")
			}
		},
	})
}
