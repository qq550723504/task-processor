package shein

import (
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sdspod "task-processor/internal/sds/adapter/product_source"
)

// PODSubmitReadiness is the action-aware SHEIN POD submit-readiness result.
type PODSubmitReadiness = sheinmarketpub.PODSubmitReadiness

// SubmitActionAllowsReadinessBlockers reports whether a SHEIN action may bypass readiness blockers.
func SubmitActionAllowsReadinessBlockers(action string) bool {
	return sheinmarketpub.SubmitActionAllowsReadinessBlockers(action)
}

// EvaluatePODSubmitReadiness evaluates POD readiness through the marketplace policy seam.
func EvaluatePODSubmitReadiness(action string, execution sdspod.Execution) PODSubmitReadiness {
	return sheinmarketpub.EvaluatePODSubmitReadiness(action, execution)
}
