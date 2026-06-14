package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

type HistoryNavigation = sheinmarketplace.HistoryNavigation

type RestorePreviewCoreData[Req any, Ctx any, Safety any, Compare any] = sheinmarketplace.RestorePreviewCoreData[Req, Ctx, Safety, Compare]
type RestorePreviewPayload[Req any, Ctx any, Safety any, Compare any, Pres any] = sheinmarketplace.RestorePreviewPayload[Req, Ctx, Safety, Compare, Pres]

func BuildHistoryNavigation(prevRevisionID, nextRevisionID string) *HistoryNavigation {
	return sheinmarketplace.BuildHistoryNavigation(prevRevisionID, nextRevisionID)
}
