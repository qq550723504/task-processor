package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/store"
)

func TestTaskRepositoryMarkBlockedRetryablePersistsMetadata(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			task := retryableTaskFixture("task-blocked-persist", time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC))
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			blockedAt := time.Date(2026, 6, 6, 10, 5, 0, 0, time.UTC)
			lastRetryAt := blockedAt.Add(-10 * time.Minute)
			nextRetryAt := blockedAt.Add(20 * time.Minute)
			block := &listingkit.RetryableBlock{
				ReasonCode:           "worker_queue_backpressure",
				ReasonMessage:        "queue full",
				BlockedAt:            blockedAt,
				LastRetryAt:          &lastRetryAt,
				NextRetryAt:          &nextRetryAt,
				RetryAttempts:        2,
				MaxAutoRetryAttempts: 8,
				RecoveryScope:        "task",
				AutoResumeEnabled:    true,
			}

			if err := repo.MarkBlockedRetryable(ctx, task.ID, block, "queue temporarily full"); err != nil {
				t.Fatalf("MarkBlockedRetryable() error = %v", err)
			}

			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusBlockedRetryable {
				t.Fatalf("Status = %q, want %q", got.Status, core.TaskStatusBlockedRetryable)
			}
			if got.Error != "queue temporarily full" {
				t.Fatalf("Error = %q, want queue temporarily full", got.Error)
			}
			if got.RetryableBlock == nil {
				t.Fatal("RetryableBlock = nil, want metadata")
			}
			if got.RetryableBlock.ReasonCode != block.ReasonCode {
				t.Fatalf("ReasonCode = %q, want %q", got.RetryableBlock.ReasonCode, block.ReasonCode)
			}
			if got.RetryableBlock.NextRetryAt == nil || !got.RetryableBlock.NextRetryAt.Equal(nextRetryAt) {
				t.Fatalf("NextRetryAt = %v, want %v", got.RetryableBlock.NextRetryAt, nextRetryAt)
			}
			if got.RetryableBlock.LastRetryAt == nil || !got.RetryableBlock.LastRetryAt.Equal(lastRetryAt) {
				t.Fatalf("LastRetryAt = %v, want %v", got.RetryableBlock.LastRetryAt, lastRetryAt)
			}
			if got.RetryableBlock.RetryAttempts != 2 {
				t.Fatalf("RetryAttempts = %d, want 2", got.RetryableBlock.RetryAttempts)
			}
			if got.RetryableBlock.AutoResumeEnabled != true {
				t.Fatalf("AutoResumeEnabled = %t, want true", got.RetryableBlock.AutoResumeEnabled)
			}
		})
	}
}

func TestTaskRepositoryMarkFailedClearsRetryableBlock(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			task := retryableTaskFixture("task-failed-clears-block", time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC))
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			if err := repo.MarkBlockedRetryable(ctx, task.ID, &listingkit.RetryableBlock{ReasonCode: "usage_release_pending", AutoResumeEnabled: true}, "release pending"); err != nil {
				t.Fatalf("MarkBlockedRetryable() error = %v", err)
			}
			if err := repo.MarkFailed(ctx, task.ID, "release completed"); err != nil {
				t.Fatalf("MarkFailed() error = %v", err)
			}
			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusFailed || got.RetryableBlock != nil {
				t.Fatalf("task = %#v, want failed task without retryable block", got)
			}
		})
	}
}

func TestTaskRepositoryMarkBlockedRetryableIfCurrentDoesNotOverwriteResolvedTask(t *testing.T) {
	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			repo := repoFactory.new(t)
			conditional, ok := repo.(listingkit.ConditionalRetryableBlockRepository)
			if !ok {
				t.Fatal("repository does not implement ConditionalRetryableBlockRepository")
			}
			ctx := listingkit.WithTenantID(context.Background(), "tenant-conditional-reblock")
			now := time.Date(2026, 8, 18, 10, 30, 0, 0, time.UTC)
			task := retryableTaskFixture("task-conditional-reblock", now)
			task.TenantID = "tenant-conditional-reblock"
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			expected := recoverableBlock(now, now.Add(-time.Minute))
			expected.ReasonCode = "usage_commit_pending"
			if err := repo.MarkBlockedRetryable(ctx, task.ID, expected, "usage settlement pending"); err != nil {
				t.Fatalf("MarkBlockedRetryable() error = %v", err)
			}
			loaded, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() before conditional update error = %v", err)
			}
			next := &listingkit.RetryableBlock{ReasonCode: "usage_commit_pending", RetryAttempts: 3, AutoResumeEnabled: true}
			applied, err := conditional.MarkBlockedRetryableIfCurrent(ctx, task.ID, loaded.RetryableBlock, next, "ledger unavailable")
			if err != nil {
				t.Fatalf("MarkBlockedRetryableIfCurrent(matching) error = %v", err)
			}
			if !applied {
				t.Fatal("MarkBlockedRetryableIfCurrent(matching) applied = false, want true")
			}
			if err := repo.MarkCompleted(ctx, task.ID, &listingkit.ListingKitResult{Status: string(core.TaskStatusCompleted)}); err != nil {
				t.Fatalf("MarkCompleted() error = %v", err)
			}

			applied, err = conditional.MarkBlockedRetryableIfCurrent(ctx, task.ID, next, &listingkit.RetryableBlock{ReasonCode: "usage_commit_pending", AutoResumeEnabled: true}, "ledger unavailable")
			if err != nil {
				t.Fatalf("MarkBlockedRetryableIfCurrent() error = %v", err)
			}
			if applied {
				t.Fatal("MarkBlockedRetryableIfCurrent() applied = true, want false after terminal resolution")
			}
			stored, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if stored.Status != core.TaskStatusCompleted || stored.RetryableBlock != nil {
				t.Fatalf("stored task = %#v, want unchanged completed task", stored)
			}
		})
	}
}

func TestTaskRepositoryListRecoverableTasksReturnsDueItemsOnly(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

			due := retryableTaskFixture("task-due", now.Add(-20*time.Minute))
			future := retryableTaskFixture("task-future", now.Add(-19*time.Minute))
			paused := retryableTaskFixture("task-paused", now.Add(-18*time.Minute))
			disabled := retryableTaskFixture("task-disabled", now.Add(-17*time.Minute))
			noRetryAt := retryableTaskFixture("task-no-retry", now.Add(-16*time.Minute))
			completed := retryableTaskFixture("task-completed", now.Add(-15*time.Minute))
			otherTenant := retryableTaskFixture("task-other-tenant", now.Add(-14*time.Minute))
			otherTenant.TenantID = "tenant-b"

			for _, task := range []*listingkit.Task{due, future, paused, disabled, noRetryAt, completed, otherTenant} {
				createCtx := ctx
				if task.TenantID == "tenant-b" {
					createCtx = listingkit.WithTenantID(context.Background(), "tenant-b")
				}
				if err := repo.CreateTask(createCtx, task); err != nil {
					t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
				}
			}

			dueRetryAt := now.Add(-time.Minute)
			futureRetryAt := now.Add(15 * time.Minute)
			pausedRetryAt := now.Add(-2 * time.Minute)
			disabledRetryAt := now.Add(-3 * time.Minute)
			otherTenantRetryAt := now.Add(-4 * time.Minute)

			mustMarkBlockedRetryable(t, repo, ctx, due.ID, recoverableBlock(now, dueRetryAt), "due block")
			mustMarkBlockedRetryable(t, repo, ctx, future.ID, recoverableBlock(now, futureRetryAt), "future block")

			pausedBlock := recoverableBlock(now, pausedRetryAt)
			pausedBlock.AutoRetryPaused = true
			mustMarkBlockedRetryable(t, repo, ctx, paused.ID, pausedBlock, "paused block")

			disabledBlock := recoverableBlock(now, disabledRetryAt)
			disabledBlock.AutoResumeEnabled = false
			mustMarkBlockedRetryable(t, repo, ctx, disabled.ID, disabledBlock, "disabled block")

			mustMarkBlockedRetryable(t, repo, ctx, noRetryAt.ID, &listingkit.RetryableBlock{
				ReasonCode:           "worker_queue_backpressure",
				ReasonMessage:        "queue full",
				BlockedAt:            now,
				RetryAttempts:        2,
				MaxAutoRetryAttempts: 8,
				RecoveryScope:        "task",
				AutoResumeEnabled:    true,
			}, "missing retry")

			if err := repo.MarkCompleted(ctx, completed.ID, &listingkit.ListingKitResult{}); err != nil {
				t.Fatalf("MarkCompleted() error = %v", err)
			}

			otherTenantCtx := listingkit.WithTenantID(context.Background(), "tenant-b")
			mustMarkBlockedRetryable(t, repo, otherTenantCtx, otherTenant.ID, recoverableBlock(now, otherTenantRetryAt), "other tenant")

			items, err := repo.ListRecoverableTasks(ctx, &listingkit.RecoverableTaskQuery{
				DueBefore: now,
				Limit:     10,
			})
			if err != nil {
				t.Fatalf("ListRecoverableTasks() error = %v", err)
			}
			if len(items) != 1 {
				t.Fatalf("len(items) = %d, want 1", len(items))
			}
			if items[0].ID != due.ID {
				t.Fatalf("items[0].ID = %q, want %q", items[0].ID, due.ID)
			}
		})
	}
}

func TestTaskRepositoryListRecoverableTasksFiltersReasonCodesBeforeLimit(t *testing.T) {
	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-recovery-reason-filter")
			now := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
			for _, fixture := range []struct {
				id     string
				reason string
			}{
				{id: "task-recovery-provider-1", reason: "worker_queue_backpressure"},
				{id: "task-recovery-provider-2", reason: "worker_queue_backpressure"},
				{id: "task-recovery-settlement", reason: "usage_commit_pending"},
			} {
				task := retryableTaskFixture(fixture.id, now)
				task.TenantID = "tenant-recovery-reason-filter"
				if err := repo.CreateTask(ctx, task); err != nil {
					t.Fatalf("CreateTask(%s) error = %v", fixture.id, err)
				}
				block := recoverableBlock(now, now.Add(-time.Minute))
				block.ReasonCode = fixture.reason
				mustMarkBlockedRetryable(t, repo, ctx, task.ID, block, fixture.reason)
			}

			settlement, err := repo.ListRecoverableTasks(ctx, &listingkit.RecoverableTaskQuery{
				DueBefore:   now,
				ReasonCodes: []string{"usage_commit_pending"},
				Limit:       1,
			})
			if err != nil {
				t.Fatalf("ListRecoverableTasks(settlement) error = %v", err)
			}
			if len(settlement) != 1 || settlement[0].ID != "task-recovery-settlement" {
				t.Fatalf("settlement candidates = %#v, want bounded settlement task", settlement)
			}

			providers, err := repo.ListRecoverableTasks(ctx, &listingkit.RecoverableTaskQuery{
				DueBefore:          now,
				ExcludeReasonCodes: []string{"usage_commit_pending"},
				Limit:              1,
			})
			if err != nil {
				t.Fatalf("ListRecoverableTasks(providers) error = %v", err)
			}
			if len(providers) != 1 || providers[0].RetryableBlock == nil || providers[0].RetryableBlock.ReasonCode != "worker_queue_backpressure" {
				t.Fatalf("provider candidates = %#v, want bounded provider task", providers)
			}
		})
	}
}

func TestTaskRepositoryRecoverBlockedTaskNowResetsBlockedState(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			task := retryableTaskFixture("task-recover-now", time.Date(2026, 6, 6, 13, 0, 0, 0, time.UTC))
			if err := repo.CreateTask(ctx, task); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			recoverAt := time.Date(2026, 6, 6, 13, 15, 0, 0, time.UTC)
			nextRetryAt := recoverAt.Add(-time.Minute)
			mustMarkBlockedRetryable(t, repo, ctx, task.ID, recoverableBlock(recoverAt.Add(-5*time.Minute), nextRetryAt), "recover me")

			if err := repo.RecoverBlockedTaskNow(ctx, task.ID, recoverAt); err != nil {
				t.Fatalf("RecoverBlockedTaskNow() error = %v", err)
			}

			got, err := repo.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got.Status != core.TaskStatusPending {
				t.Fatalf("Status = %q, want %q", got.Status, core.TaskStatusPending)
			}
			if got.Error != "" {
				t.Fatalf("Error = %q, want empty", got.Error)
			}
			if got.RetryableBlock == nil {
				t.Fatal("RetryableBlock = nil, want retained retry metadata")
			}
			if got.RetryableBlock.NextRetryAt != nil {
				t.Fatalf("NextRetryAt = %v, want nil", got.RetryableBlock.NextRetryAt)
			}
			if got.RetryableBlock.LastRetryAt == nil || !got.RetryableBlock.LastRetryAt.Equal(recoverAt) {
				t.Fatalf("LastRetryAt = %v, want %v", got.RetryableBlock.LastRetryAt, recoverAt)
			}
		})
	}
}

func TestTaskRepositoryRecoverBlockedTaskNowRejectsNotEligibleTasks(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			recoverAt := time.Date(2026, 6, 6, 13, 15, 0, 0, time.UTC)

			pendingTask := retryableTaskFixture("task-pending", recoverAt.Add(-10*time.Minute))
			futureBlockedTask := retryableTaskFixture("task-future-blocked", recoverAt.Add(-9*time.Minute))
			if err := repo.CreateTask(ctx, pendingTask); err != nil {
				t.Fatalf("CreateTask(%s) error = %v", pendingTask.ID, err)
			}
			if err := repo.CreateTask(ctx, futureBlockedTask); err != nil {
				t.Fatalf("CreateTask(%s) error = %v", futureBlockedTask.ID, err)
			}
			mustMarkBlockedRetryable(t, repo, ctx, futureBlockedTask.ID, recoverableBlock(recoverAt.Add(-5*time.Minute), recoverAt.Add(10*time.Minute)), "future blocked")

			for _, taskID := range []string{pendingTask.ID, futureBlockedTask.ID} {
				err := repo.RecoverBlockedTaskNow(ctx, taskID, recoverAt)
				if !errors.Is(err, core.ErrTaskNotRecoverable) {
					t.Fatalf("RecoverBlockedTaskNow(%s) error = %v, want ErrTaskNotRecoverable", taskID, err)
				}
			}

			gotPending, err := repo.GetTask(ctx, pendingTask.ID)
			if err != nil {
				t.Fatalf("GetTask(%s) error = %v", pendingTask.ID, err)
			}
			if gotPending.Status != core.TaskStatusPending {
				t.Fatalf("pending task status = %q, want %q", gotPending.Status, core.TaskStatusPending)
			}

			gotFuture, err := repo.GetTask(ctx, futureBlockedTask.ID)
			if err != nil {
				t.Fatalf("GetTask(%s) error = %v", futureBlockedTask.ID, err)
			}
			if gotFuture.Status != core.TaskStatusBlockedRetryable {
				t.Fatalf("future blocked status = %q, want %q", gotFuture.Status, core.TaskStatusBlockedRetryable)
			}
			if gotFuture.RetryableBlock == nil || gotFuture.RetryableBlock.NextRetryAt == nil || !gotFuture.RetryableBlock.NextRetryAt.Equal(recoverAt.Add(10*time.Minute)) {
				t.Fatalf("future blocked RetryableBlock = %+v, want unchanged NextRetryAt", gotFuture.RetryableBlock)
			}
		})
	}
}

func TestTaskRepositoryBulkRecoverBlockedTasksCountsRecoveredItems(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			now := time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC)

			dueA := retryableTaskFixture("task-due-a", now.Add(-10*time.Minute))
			dueB := retryableTaskFixture("task-due-b", now.Add(-9*time.Minute))
			future := retryableTaskFixture("task-future", now.Add(-8*time.Minute))

			for _, task := range []*listingkit.Task{dueA, dueB, future} {
				if err := repo.CreateTask(ctx, task); err != nil {
					t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
				}
			}

			mustMarkBlockedRetryable(t, repo, ctx, dueA.ID, recoverableBlock(now, now.Add(-time.Minute)), "due a")
			mustMarkBlockedRetryable(t, repo, ctx, dueB.ID, recoverableBlock(now, now.Add(-2*time.Minute)), "due b")
			mustMarkBlockedRetryable(t, repo, ctx, future.ID, recoverableBlock(now, now.Add(20*time.Minute)), "future")

			recovered, err := repo.BulkRecoverBlockedTasks(ctx, &listingkit.RecoverBlockedTasksQuery{
				DueBefore: now,
				RecoverAt: now,
				Limit:     10,
			})
			if err != nil {
				t.Fatalf("BulkRecoverBlockedTasks() error = %v", err)
			}
			if recovered != 2 {
				t.Fatalf("BulkRecoverBlockedTasks() recovered = %d, want 2", recovered)
			}

			for _, taskID := range []string{dueA.ID, dueB.ID} {
				got, err := repo.GetTask(ctx, taskID)
				if err != nil {
					t.Fatalf("GetTask(%s) error = %v", taskID, err)
				}
				if got.Status != core.TaskStatusPending {
					t.Fatalf("%s status = %q, want %q", taskID, got.Status, core.TaskStatusPending)
				}
				if got.RetryableBlock == nil || got.RetryableBlock.NextRetryAt != nil {
					t.Fatalf("%s retryable block = %+v, want NextRetryAt cleared", taskID, got.RetryableBlock)
				}
			}

			futureTask, err := repo.GetTask(ctx, future.ID)
			if err != nil {
				t.Fatalf("GetTask(%s) error = %v", future.ID, err)
			}
			if futureTask.Status != core.TaskStatusBlockedRetryable {
				t.Fatalf("future status = %q, want %q", futureTask.Status, core.TaskStatusBlockedRetryable)
			}
		})
	}
}

func TestTaskRepositoryBulkRecoverBlockedTasksSkipsItemsThatAreNotRecoverableAtRecoverTime(t *testing.T) {
	t.Parallel()

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			ctx := listingkit.WithTenantID(context.Background(), "tenant-a")
			dueBefore := time.Date(2026, 6, 6, 14, 0, 0, 0, time.UTC)
			recoverAt := dueBefore.Add(-2 * time.Minute)

			stillEligible := retryableTaskFixture("task-still-eligible", dueBefore.Add(-10*time.Minute))
			noLongerEligible := retryableTaskFixture("task-no-longer-eligible", dueBefore.Add(-9*time.Minute))
			for _, task := range []*listingkit.Task{stillEligible, noLongerEligible} {
				if err := repo.CreateTask(ctx, task); err != nil {
					t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
				}
			}

			mustMarkBlockedRetryable(t, repo, ctx, stillEligible.ID, recoverableBlock(dueBefore.Add(-20*time.Minute), recoverAt.Add(-time.Minute)), "eligible")
			mustMarkBlockedRetryable(t, repo, ctx, noLongerEligible.ID, recoverableBlock(dueBefore.Add(-19*time.Minute), recoverAt.Add(3*time.Minute)), "not yet due at recover time")

			recovered, err := repo.BulkRecoverBlockedTasks(ctx, &listingkit.RecoverBlockedTasksQuery{
				DueBefore: dueBefore,
				RecoverAt: recoverAt,
				Limit:     10,
			})
			if err != nil {
				t.Fatalf("BulkRecoverBlockedTasks() error = %v", err)
			}
			if recovered != 1 {
				t.Fatalf("BulkRecoverBlockedTasks() recovered = %d, want 1", recovered)
			}

			gotEligible, err := repo.GetTask(ctx, stillEligible.ID)
			if err != nil {
				t.Fatalf("GetTask(%s) error = %v", stillEligible.ID, err)
			}
			if gotEligible.Status != core.TaskStatusPending {
				t.Fatalf("eligible status = %q, want %q", gotEligible.Status, core.TaskStatusPending)
			}

			gotIneligible, err := repo.GetTask(ctx, noLongerEligible.ID)
			if err != nil {
				t.Fatalf("GetTask(%s) error = %v", noLongerEligible.ID, err)
			}
			if gotIneligible.Status != core.TaskStatusBlockedRetryable {
				t.Fatalf("ineligible status = %q, want %q", gotIneligible.Status, core.TaskStatusBlockedRetryable)
			}
			if gotIneligible.RetryableBlock == nil || gotIneligible.RetryableBlock.NextRetryAt == nil || !gotIneligible.RetryableBlock.NextRetryAt.Equal(recoverAt.Add(3*time.Minute)) {
				t.Fatalf("ineligible RetryableBlock = %+v, want unchanged NextRetryAt", gotIneligible.RetryableBlock)
			}
		})
	}
}

func TestTaskRepositoryOwnerScopeParityAcrossRepos(t *testing.T) {
	t.Parallel()

	restore := listingkit.SetOwnerScopeRequiredForTesting(true)
	t.Cleanup(restore)

	for _, repoFactory := range retryableTaskRepoFactories(t) {
		t.Run(repoFactory.name, func(t *testing.T) {
			t.Parallel()

			repo := repoFactory.new(t)
			baseCtx := listingkit.WithTenantID(context.Background(), "tenant-a")
			userCtx := openaiclient.WithIdentity(baseCtx, openaiclient.Identity{TenantID: "tenant-a", UserID: "legacy-user"})

			visible := retryableTaskFixture("task-visible", time.Date(2026, 6, 6, 15, 0, 0, 0, time.UTC))
			visible.UserID = ""
			visible.Request.UserID = "legacy-user"

			hidden := retryableTaskFixture("task-hidden", time.Date(2026, 6, 6, 14, 59, 0, 0, time.UTC))
			hidden.UserID = ""
			hidden.Request.UserID = "someone-else"

			for _, task := range []*listingkit.Task{visible, hidden} {
				if err := repo.CreateTask(baseCtx, task); err != nil {
					t.Fatalf("CreateTask(%s) error = %v", task.ID, err)
				}
			}

			storedVisible, err := repo.GetTask(baseCtx, visible.ID)
			if err != nil {
				t.Fatalf("GetTask(baseCtx, %s) error = %v", visible.ID, err)
			}
			if storedVisible.UserID != "legacy-user" {
				t.Fatalf("stored visible UserID = %q, want legacy-user", storedVisible.UserID)
			}
			if storedVisible.Request == nil || storedVisible.Request.UserID != "legacy-user" {
				t.Fatalf("stored visible Request.UserID = %v, want legacy-user", storedVisible.Request)
			}

			items, total, err := repo.ListTasks(userCtx, &listingkit.TaskListQuery{Page: 1, PageSize: 10})
			if err != nil {
				t.Fatalf("ListTasks() error = %v", err)
			}
			if total != 1 || len(items) != 1 || items[0].ID != visible.ID {
				t.Fatalf("owner-scoped items = %+v total=%d, want only %s", items, total, visible.ID)
			}

			gotVisible, err := repo.GetTask(userCtx, visible.ID)
			if err != nil {
				t.Fatalf("GetTask(userCtx, %s) error = %v", visible.ID, err)
			}
			if gotVisible.ID != visible.ID {
				t.Fatalf("GetTask(userCtx) id = %q, want %q", gotVisible.ID, visible.ID)
			}

			_, err = repo.GetTask(userCtx, hidden.ID)
			if !errors.Is(err, core.ErrTaskNotFound) {
				t.Fatalf("GetTask(userCtx, %s) error = %v, want ErrTaskNotFound", hidden.ID, err)
			}
		})
	}
}

type retryableTaskRepoFactory struct {
	name string
	new  func(t *testing.T) listingkit.Repository
}

func retryableTaskRepoFactories(t *testing.T) []retryableTaskRepoFactory {
	t.Helper()
	return []retryableTaskRepoFactory{
		{
			name: "gorm",
			new: func(t *testing.T) listingkit.Repository {
				t.Helper()
				db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
				if err != nil {
					t.Fatalf("open db: %v", err)
				}
				if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
					t.Fatalf("auto migrate: %v", err)
				}
				return store.NewTaskRepository(db)
			},
		},
		{
			name: "memory",
			new: func(t *testing.T) listingkit.Repository {
				t.Helper()
				return store.NewMemTaskRepository()
			},
		},
	}
}

func retryableTaskFixture(id string, createdAt time.Time) *listingkit.Task {
	return &listingkit.Task{
		ID:        id,
		TenantID:  "tenant-a",
		Status:    core.TaskStatusPending,
		Request:   &listingkit.GenerateRequest{Platforms: []string{"shein"}},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func recoverableBlock(blockedAt time.Time, nextRetryAt time.Time) *listingkit.RetryableBlock {
	return &listingkit.RetryableBlock{
		ReasonCode:           "worker_queue_backpressure",
		ReasonMessage:        "queue full",
		BlockedAt:            blockedAt,
		NextRetryAt:          &nextRetryAt,
		RetryAttempts:        2,
		MaxAutoRetryAttempts: 8,
		RecoveryScope:        "task",
		AutoResumeEnabled:    true,
	}
}

func mustMarkBlockedRetryable(t *testing.T, repo listingkit.Repository, ctx context.Context, taskID string, block *listingkit.RetryableBlock, errorMsg string) {
	t.Helper()
	if err := repo.MarkBlockedRetryable(ctx, taskID, block, errorMsg); err != nil {
		t.Fatalf("MarkBlockedRetryable(%s) error = %v", taskID, err)
	}
}
