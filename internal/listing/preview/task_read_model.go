package preview

// TaskReadModelInput combines task-shell facts with result-driven preview data
// so callers can build a task-scoped preview without re-owning composition.
type TaskReadModelInput struct {
	Task      TaskShellInput
	ReadModel ReadModelInput
}

func BuildTaskReadModel(input TaskReadModelInput) *Preview {
	return buildPreviewWithReadModel(BuildTaskShell(input.Task), input.ReadModel)
}
