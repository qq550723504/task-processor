package aicapability

import (
	"context"
	"fmt"
	"strings"
)

type CacheStatus string

const (
	CacheStatusNotApplicable CacheStatus = "not_applicable"
	CacheStatusHit           CacheStatus = "hit"
	CacheStatusMiss          CacheStatus = "miss"
)

type ExecutionPlan struct {
	Mode          RoutingMode
	RouteOutcome  RouteOutcome
	Decision      RouteDecision
	LegacyClients []string
}

type ExecutionPlanner interface {
	Plan(context.Context, RouteRequest) (ExecutionPlan, error)
}

func (p ExecutionPlan) Validate() error {
	switch p.Mode {
	case RoutingModeActive:
		if p.RouteOutcome != RouteOutcomeActive || !isBoundDecision(p.Decision) {
			return fmt.Errorf("active execution plan requires a fully bound active decision")
		}
	case RoutingModeLegacy:
		if p.RouteOutcome != RouteOutcomeLegacy || len(normalizedKeys(p.LegacyClients)) == 0 {
			return fmt.Errorf("legacy execution plan requires a legacy route outcome and client candidate")
		}
	default:
		return fmt.Errorf("execution plan routing mode %q is not executable", strings.TrimSpace(string(p.Mode)))
	}
	return nil
}

func isBoundDecision(decision RouteDecision) bool {
	return strings.TrimSpace(string(decision.Capability)) != "" &&
		strings.TrimSpace(string(decision.Operation)) != "" &&
		strings.TrimSpace(decision.ProviderID) != "" &&
		strings.TrimSpace(decision.ModelID) != "" &&
		strings.TrimSpace(decision.RoutingKey) != "" &&
		strings.TrimSpace(decision.CredentialReference) != ""
}
