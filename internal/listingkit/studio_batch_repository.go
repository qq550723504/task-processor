package listingkit

import (
	studiodomain "task-processor/internal/listing/studio"
)

var ErrStudioBatchUnknownItemReference = studiodomain.ErrBatchUnknownItemReference
var ErrStudioBatchOwnershipConflict = studiodomain.ErrBatchOwnershipConflict

type StudioBatchRepository = studiodomain.BatchRepository[
	StudioBatchRecord,
	StudioBatchItemRecord,
	StudioGenerationAttemptRecord,
	StudioMaterializedDesignRecord,
	StudioBatchDetailGraph,
	StudioBatchItemStatus,
]
