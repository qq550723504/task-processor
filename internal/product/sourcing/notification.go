package sourcing

import "context"

// AcquisitionNoticeReader enumerates native facts without dispatching, publishing
// or querying a provider. The original owner still authorizes every read.
type AcquisitionNoticeReader interface {
	ListNoticeOperations(context.Context, PublicationScope, string, int) ([]AcquisitionOperation, string, error)
}

type AcquisitionNoticeFact struct {
	Operation   AcquisitionOperation
	Publication *PersistedPublication
}
