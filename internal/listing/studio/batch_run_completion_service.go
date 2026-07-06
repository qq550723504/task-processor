package studio

import (
	"context"
	"fmt"
	"time"
)

type BatchRunCompletionService[Item any, Status comparable] struct {
	updateItem               func(context.Context, *Item) error
	itemStatus               func(*Item) Status
	markCancelled            func(*Item, time.Time)
	now                      func() time.Time
	terminalStatuses         map[Status]struct{}
	succeededStatus          Status
	failedStatus             Status
	cancelledStatus          Status
	partiallySucceededStatus Status
}

type BatchRunCompletionServiceConfig[Item any, Status comparable] struct {
	UpdateItem               func(context.Context, *Item) error
	ItemStatus               func(*Item) Status
	MarkCancelled            func(*Item, time.Time)
	Now                      func() time.Time
	TerminalStatuses         []Status
	SucceededStatus          Status
	FailedStatus             Status
	CancelledStatus          Status
	PartiallySucceededStatus Status
}

type BatchRunItemCounters struct {
	Total     int
	Completed int
	Succeeded int
	Failed    int
}

func NewBatchRunCompletionService[Item any, Status comparable](config BatchRunCompletionServiceConfig[Item, Status]) *BatchRunCompletionService[Item, Status] {
	terminalStatuses := make(map[Status]struct{}, len(config.TerminalStatuses)+3)
	for _, status := range config.TerminalStatuses {
		terminalStatuses[status] = struct{}{}
	}
	terminalStatuses[config.SucceededStatus] = struct{}{}
	terminalStatuses[config.FailedStatus] = struct{}{}
	terminalStatuses[config.CancelledStatus] = struct{}{}
	return &BatchRunCompletionService[Item, Status]{
		updateItem:               config.UpdateItem,
		itemStatus:               config.ItemStatus,
		markCancelled:            config.MarkCancelled,
		now:                      config.Now,
		terminalStatuses:         terminalStatuses,
		succeededStatus:          config.SucceededStatus,
		failedStatus:             config.FailedStatus,
		cancelledStatus:          config.CancelledStatus,
		partiallySucceededStatus: config.PartiallySucceededStatus,
	}
}

func (s *BatchRunCompletionService[Item, Status]) CancelUnfinishedItems(ctx context.Context, items []Item) error {
	if s == nil || s.updateItem == nil {
		return fmt.Errorf("studio batch run item updater is not configured")
	}
	if s.itemStatus == nil {
		return fmt.Errorf("studio batch run item status reader is not configured")
	}
	if s.markCancelled == nil {
		return fmt.Errorf("studio batch run item cancellation adapter is not configured")
	}
	for index := range items {
		item := &items[index]
		if s.isTerminalStatus(s.itemStatus(item)) {
			continue
		}
		s.markCancelled(item, s.currentTime())
		if err := s.updateItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *BatchRunCompletionService[Item, Status]) ResolveFinalStatus(cancelRequested bool, items []Item) Status {
	if cancelRequested {
		return s.cancelledStatus
	}
	if s == nil || s.itemStatus == nil {
		return s.succeededStatus
	}
	if len(items) == 0 {
		return s.failedStatus
	}

	succeeded := 0
	failed := 0
	for index := range items {
		switch s.itemStatus(&items[index]) {
		case s.succeededStatus:
			succeeded++
		case s.failedStatus:
			failed++
		}
	}

	switch {
	case failed > 0 && succeeded > 0:
		return s.partiallySucceededStatus
	case failed > 0:
		return s.failedStatus
	default:
		return s.succeededStatus
	}
}

func (s *BatchRunCompletionService[Item, Status]) CountItems(items []Item) BatchRunItemCounters {
	counters := BatchRunItemCounters{Total: len(items)}
	if s == nil || s.itemStatus == nil {
		return counters
	}
	for index := range items {
		switch s.itemStatus(&items[index]) {
		case s.succeededStatus:
			counters.Completed++
			counters.Succeeded++
		case s.failedStatus, s.cancelledStatus:
			counters.Completed++
			counters.Failed++
		}
	}
	return counters
}

func (s *BatchRunCompletionService[Item, Status]) isTerminalStatus(status Status) bool {
	if s == nil {
		return false
	}
	_, ok := s.terminalStatuses[status]
	return ok
}

func (s *BatchRunCompletionService[Item, Status]) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
