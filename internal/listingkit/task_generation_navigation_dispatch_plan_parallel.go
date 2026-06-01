package listingkit

import (
	"context"
	"sync"
)

type taskGenerationNavigationDispatchPlanParallelPhase struct {
	service *taskGenerationService
}

type taskGenerationNavigationDispatchPlanParallelEntry struct {
	step   GenerationNavigationDispatchStep
	result *GenerationNavigationDispatchExecutionStep
}

func buildTaskGenerationNavigationDispatchPlanParallelPhase(service *taskGenerationService) *taskGenerationNavigationDispatchPlanParallelPhase {
	return &taskGenerationNavigationDispatchPlanParallelPhase{service: service}
}

func (p *taskGenerationNavigationDispatchPlanParallelPhase) run(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	if p == nil || p.service == nil || plan == nil || execution == nil {
		return
	}

	entries := p.buildEntries(plan, responseMode)
	maxParallelism := p.maxParallelism(plan)

	sem := make(chan struct{}, maxParallelism)
	var wg sync.WaitGroup
	for index := range entries {
		if entries[index].result != nil && entries[index].result.Status == "deduplicated" {
			continue
		}

		wg.Add(1)
		go func(entry *taskGenerationNavigationDispatchPlanParallelEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			result := p.service.executeGenerationNavigationDispatchPlanStep(ctx, taskID, entry.step, responseMode)
			result.DeduplicationKey = generationNavigationDispatchStepDeduplicationKey(entry.step, responseMode)
			entry.result = result
		}(&entries[index])
	}

	wg.Wait()
	p.replayDeduplicatedSourceState(entries)

	for _, entry := range entries {
		if entry.result == nil {
			continue
		}
		execution.Steps = append(execution.Steps, *entry.result)
		applyGenerationNavigationDispatchExecutionStats(execution, entry.result)
	}
}

func (p *taskGenerationNavigationDispatchPlanParallelPhase) buildEntries(plan *GenerationNavigationDispatchPlan, responseMode string) []taskGenerationNavigationDispatchPlanParallelEntry {
	entries := make([]taskGenerationNavigationDispatchPlanParallelEntry, 0, len(plan.Steps))
	indexByKey := make(map[string]int, len(plan.Steps))
	for _, step := range plan.Steps {
		key := generationNavigationDispatchStepDeduplicationKey(step, responseMode)
		if existing, ok := indexByKey[key]; ok {
			deduplicated := generationNavigationDispatchPlanDeduplicatedStep(step, key, existing)
			entries = append(entries, taskGenerationNavigationDispatchPlanParallelEntry{
				step:   step,
				result: &deduplicated,
			})
			continue
		}

		indexByKey[key] = len(entries)
		entries = append(entries, taskGenerationNavigationDispatchPlanParallelEntry{
			step:   step,
			result: generationNavigationDispatchExecutionPendingStep(step, key, responseMode),
		})
	}
	return entries
}

func (p *taskGenerationNavigationDispatchPlanParallelPhase) maxParallelism(plan *GenerationNavigationDispatchPlan) int {
	if plan == nil || plan.MaxParallelism <= 0 {
		return 1
	}
	return plan.MaxParallelism
}

func (p *taskGenerationNavigationDispatchPlanParallelPhase) replayDeduplicatedSourceState(entries []taskGenerationNavigationDispatchPlanParallelEntry) {
	for index := range entries {
		stepResult := entries[index].result
		if stepResult == nil || stepResult.Status != "deduplicated" {
			continue
		}

		source := stepResult.DeduplicatedFrom
		if source < 0 || source >= len(entries) {
			continue
		}
		sourceResult := entries[source].result
		if sourceResult == nil {
			continue
		}

		stepResult.DeltaToken = sourceResult.DeltaToken
		stepResult.NotModified = sourceResult.NotModified
		stepResult.NoChanges = sourceResult.NoChanges
	}
}
