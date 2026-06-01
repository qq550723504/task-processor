package listingkit

type taskGenerationTasksReadPagePhase struct{}

func buildTaskGenerationTasksReadPagePhase() *taskGenerationTasksReadPagePhase {
	return &taskGenerationTasksReadPagePhase{}
}

func (p *taskGenerationTasksReadPagePhase) run(snapshot *taskGenerationTasksReadSnapshot, query *GenerationTaskQuery) *GenerationTaskPage {
	if snapshot == nil {
		snapshot = &taskGenerationTasksReadSnapshot{task: &Task{}}
	}
	if snapshot.task == nil {
		snapshot.task = &Task{}
	}
	filtered := filterGenerationTasks(snapshot.tasks, query)
	sorted := sortGenerationTasks(filtered, query)
	paged, meta := paginateGenerationTasks(sorted, query)
	return buildGenerationTaskPage(snapshot.task.ID, snapshot.task.UpdatedAt, filtered, paged, meta)
}
