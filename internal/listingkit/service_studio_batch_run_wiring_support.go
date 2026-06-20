package listingkit

import (
	"context"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

type taskStudioBatchRunRepoWiring struct {
	repo              StudioBatchRunRepository
	batchRepo         StudioBatchRepository
	studioSessionRepo StudioSessionRepository
}

type taskStudioBatchRunWiring struct {
	repo              StudioBatchRunRepository
	batchRepo         StudioBatchRepository
	studioSessionRepo StudioSessionRepository
}

type taskStudioBatchRunConfigWiring struct {
	batchRun taskStudioBatchRunWiring
}

type taskStudioBatchRunCollaboratorWiring struct {
	service *service
	wiring  taskStudioBatchRunWiring
}

type taskStudioBatchRunCollaborators struct {
	batchRun       *taskStudioBatchRunService
	runExecutor    *taskStudioBatchRunExecutor
	runCoordinator *studioBatchRunCoordinator
}

func buildTaskStudioBatchRunRepoWiring(s *service) taskStudioBatchRunRepoWiring {
	return taskStudioBatchRunRepoWiring{
		repo:              resolveStudioBatchRunRepo(s),
		batchRepo:         resolveStudioBatchRepo(s),
		studioSessionRepo: resolveStudioSessionRepo(s),
	}
}

func buildTaskStudioBatchRunWiring(s *service) taskStudioBatchRunWiring {
	repoWiring := buildTaskStudioBatchRunRepoWiring(s)
	return taskStudioBatchRunWiring{
		repo:              repoWiring.repo,
		batchRepo:         repoWiring.batchRepo,
		studioSessionRepo: repoWiring.studioSessionRepo,
	}
}

func (w taskStudioBatchRunWiring) newServiceRunner(startRun func(context.Context, string) error) *studiodomain.BatchRunService {
	return newListingStudioBatchRunService(w.repo, w.studioSessionRepo, startRun)
}

func (w taskStudioBatchRunWiring) newCompletionRunner(now func() time.Time) *listingStudioBatchRunCompletionRunner {
	return newListingStudioBatchRunCompletionService(w.repo, now)
}

func buildTaskStudioBatchRunConfigWiring(s *service) taskStudioBatchRunConfigWiring {
	return taskStudioBatchRunConfigWiring{
		batchRun: buildTaskStudioBatchRunWiring(s),
	}
}

func buildTaskStudioBatchRunCollaboratorWiring(s *service) taskStudioBatchRunCollaboratorWiring {
	return taskStudioBatchRunCollaboratorWiring{
		service: s,
		wiring:  buildTaskStudioBatchRunWiring(s),
	}
}

func (w taskStudioBatchRunCollaboratorWiring) newRunExecutor() *taskStudioBatchRunExecutor {
	if resolveStudioBatchRunRepo(w.service) == nil || resolveStudioSessionRepo(w.service) == nil {
		return nil
	}
	if resolveStudioImageGenerator(w.service) == nil || resolveStudioUploadStore(w.service) == nil {
		return nil
	}
	return newTaskStudioBatchRunExecutor(buildTaskStudioBatchRunExecutorConfigWithWiring(w.service, w.wiring))
}

func (w taskStudioBatchRunCollaboratorWiring) newRunCoordinator(executor *taskStudioBatchRunExecutor) *studioBatchRunCoordinator {
	if executor == nil {
		return nil
	}
	return newStudioBatchRunCoordinator(buildStudioBatchRunCoordinatorConfigWithCollaborators(w.wiring, executor))
}

func (w taskStudioBatchRunCollaboratorWiring) newBatchRun(coordinator *studioBatchRunCoordinator) *taskStudioBatchRunService {
	return newTaskStudioBatchRunService(buildTaskStudioBatchRunServiceConfigWithCollaborators(w.wiring, coordinator))
}

func (w taskStudioBatchRunCollaboratorWiring) resolve(existing taskStudioBatchRunCollaborators) taskStudioBatchRunCollaborators {
	runExecutor := existing.runExecutor
	if runExecutor == nil {
		runExecutor = w.newRunExecutor()
	}
	runCoordinator := existing.runCoordinator
	if runCoordinator == nil {
		runCoordinator = w.newRunCoordinator(runExecutor)
	}
	batchRun := existing.batchRun
	if batchRun == nil {
		batchRun = w.newBatchRun(runCoordinator)
	}
	return taskStudioBatchRunCollaborators{
		batchRun:       batchRun,
		runExecutor:    runExecutor,
		runCoordinator: runCoordinator,
	}
}

func buildTaskStudioBatchRunServiceConfigWithCollaborators(
	wiring taskStudioBatchRunWiring,
	coordinator *studioBatchRunCoordinator,
) taskStudioBatchRunServiceConfig {
	var startRun func(ctx context.Context, runID string) error
	if coordinator != nil {
		startRun = coordinator.StartRun
	}
	return taskStudioBatchRunServiceConfig{
		repo:              wiring.repo,
		batchRepo:         wiring.batchRepo,
		studioSessionRepo: wiring.studioSessionRepo,
		startRun:          startRun,
		runner:            wiring.newServiceRunner(startRun),
	}
}

func buildStudioBatchRunCoordinatorConfigWithCollaborators(
	wiring taskStudioBatchRunWiring,
	executor *taskStudioBatchRunExecutor,
) studioBatchRunCoordinatorConfig {
	return studioBatchRunCoordinatorConfig{
		repo:     wiring.repo,
		executor: executor,
	}
}

func buildTaskStudioBatchRunExecutorConfigWithWiring(
	s *service,
	wiring taskStudioBatchRunWiring,
) taskStudioBatchRunExecutorConfig {
	return taskStudioBatchRunExecutorConfig{
		repo:               wiring.repo,
		executeGenerateOne: s.executeStudioBatchRunItem,
		executeCreateTasks: s.executeStudioBatchRunTaskCreation,
		completionRunner:   wiring.newCompletionRunner(nil),
	}
}
