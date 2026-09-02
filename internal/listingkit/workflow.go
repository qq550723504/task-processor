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
	if state.blocked {
		return state.result, nil
	}
	final := s.runPlatformAdaptation(ctx, task, state.snapshot)
	return final, nil
}
