package sdspod

import (
	"fmt"
	"strings"
)

const SupportedBaselineVersion = 1

const (
	BaselinePayloadPresent = "present"
	BaselinePayloadMissing = "missing"
	BaselinePayloadInvalid = "invalid"
	BaselinePayloadEmpty   = "empty"
)

type BaselineSnapshot struct {
	CacheStatus          string
	Version              int
	PayloadState         string
	ValidationStatus     string
	ValidationReasonCode string
	ValidationReason     string
}

type BaselineDecision struct {
	Reusable         bool
	Status           string
	CacheStatus      string
	ValidationStatus string
	ReasonCode       string
	Reason           string
}

func EvaluateBaseline(snapshot BaselineSnapshot) BaselineDecision {
	cacheStatus := strings.ToLower(strings.TrimSpace(snapshot.CacheStatus))
	validationStatus := normalizeBaselineValidationStatus(snapshot.ValidationStatus)
	decision := BaselineDecision{CacheStatus: cacheStatus, ValidationStatus: validationStatus}
	if cacheStatus != "baseline_cached" && cacheStatus != "ready" {
		decision.Status = "missing"
		decision.ReasonCode = "cache_unavailable"
		decision.Reason = fmt.Sprintf("Baseline cache is not usable for grouped SDS create (status: %s).", firstNonEmptyBaseline(cacheStatus, "unknown"))
		return decision
	}
	if snapshot.Version != SupportedBaselineVersion {
		decision.Status = "failed"
		decision.ReasonCode = "cache_version_unsupported"
		decision.Reason = fmt.Sprintf("Baseline cache version %d is not supported.", snapshot.Version)
		return decision
	}
	switch strings.ToLower(strings.TrimSpace(snapshot.PayloadState)) {
	case BaselinePayloadMissing:
		decision.Status, decision.ReasonCode, decision.Reason = "failed", "cache_payload_missing", "Baseline cache entry is marked baseline_cached but missing canonical payload."
		return decision
	case BaselinePayloadInvalid:
		decision.Status, decision.ReasonCode, decision.Reason = "failed", "cache_payload_invalid", "Baseline cache payload is invalid."
		return decision
	case BaselinePayloadEmpty:
		decision.Status, decision.ReasonCode, decision.Reason = "failed", "cache_payload_empty", "Baseline cache entry resolved to an empty canonical product."
		return decision
	}
	if validationStatus == "ready" || cacheStatus == "ready" && validationStatus == "unknown" {
		decision.Reusable = true
		decision.Status = "ready"
		return decision
	}
	decision.Status = baselineOverallStatus(validationStatus)
	decision.ReasonCode = firstNonEmptyBaseline(snapshot.ValidationReasonCode, "validation_not_ready")
	decision.Reason = firstNonEmptyBaseline(snapshot.ValidationReason, "SDS baseline validation is not ready.")
	return decision
}

func normalizeBaselineValidationStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ready", "blocked", "failed":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func baselineOverallStatus(validationStatus string) string {
	switch validationStatus {
	case "ready":
		return "ready"
	case "blocked":
		return "blocked"
	case "failed":
		return "failed"
	default:
		return "baseline_cached"
	}
}

func firstNonEmptyBaseline(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
