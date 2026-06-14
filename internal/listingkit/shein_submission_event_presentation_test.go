package listingkit

import (
	"testing"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestAttachSheinSubmissionEventStoreResolutionDecoratesMissingItems(t *testing.T) {
	t.Parallel()

	storeResolution := &sheinpub.SubmissionStoreResolution{StoreID: 903}
	items := sheinworkspace.AttachSubmissionEventStoreResolution([]sheinpub.SubmissionEvent{
		{ID: "event-1"},
	}, storeResolution)
	if len(items) != 1 || items[0].StoreResolution == nil || items[0].StoreResolution.StoreID != 903 {
		t.Fatalf("items = %+v, want attached store resolution", items)
	}
}

func TestAttachSheinSubmissionEventStoreResolutionPreservesExistingItems(t *testing.T) {
	t.Parallel()

	items := sheinworkspace.AttachSubmissionEventStoreResolution([]sheinpub.SubmissionEvent{
		{
			ID:              "event-1",
			StoreResolution: &sheinpub.SubmissionStoreResolution{StoreID: 777},
		},
	}, &sheinpub.SubmissionStoreResolution{StoreID: 903})
	if len(items) != 1 || items[0].StoreResolution == nil || items[0].StoreResolution.StoreID != 777 {
		t.Fatalf("items = %+v, want existing store resolution preserved", items)
	}
}
