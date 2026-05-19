package listingkit

import "context"

func (s *service) runWorkflow(ctx context.Context, task *Task) (*ListingKitResult, error) {
	state, err := s.runStandardProductWorkflow(ctx, task)
	if err != nil {
		if state == nil {
			return nil, err
		}
		return state.result, err
	}
	final := s.runPlatformAdaptation(
		ctx,
		task,
		state.snapshot,
		state.recipesByPlatform,
		state.generationPlan,
		state.inventory,
		state.persistedGenerationTasks,
		state.enableAssetGeneration,
		state.sdsOptions,
	)
	return final, nil
}
