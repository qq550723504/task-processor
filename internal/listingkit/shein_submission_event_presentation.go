package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func attachSheinSubmissionEventStoreResolution(
	events []sheinpub.SubmissionEvent,
	storeResolution *sheinpub.SubmissionStoreResolution,
) []sheinpub.SubmissionEvent {
	if len(events) == 0 {
		return nil
	}

	items := append([]sheinpub.SubmissionEvent(nil), events...)
	if storeResolution == nil {
		return items
	}
	for idx := range items {
		if items[idx].StoreResolution != nil && items[idx].StoreResolution.StoreID > 0 {
			continue
		}
		items[idx].StoreResolution = storeResolution
	}
	return items
}
