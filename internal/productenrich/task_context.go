package productenrich

import "context"

type inlineTaskExecutionKey struct{}

// WithInlineTaskExecution marks task creation so the caller can execute it inline
// without also enqueueing it to the worker pool.
func WithInlineTaskExecution(ctx context.Context) context.Context {
	return context.WithValue(ctx, inlineTaskExecutionKey{}, true)
}

func shouldEnqueueTask(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	inline, _ := ctx.Value(inlineTaskExecutionKey{}).(bool)
	return !inline
}
