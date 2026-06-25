package listingcontrol

import (
	"time"

	"task-processor/internal/core/config"
)

func controlPlaneConfigStatus(controlCfg config.ListingControlPlaneConfig, platform string) ControlPlaneConfigStatus {
	resolvedPlatform := normalizePlatform(platform)
	quotaTTL := controlCfg.QuotaKeyTTLGrace
	quotaStatus := ControlPlaneConfigFieldStatus{
		Value:     durationStatusValue(quotaTTL),
		Effective: false,
		Status:    "disabled",
	}
	if quotaTTL > 0 {
		quotaStatus.Status = "reserved"
	}
	return ControlPlaneConfigStatus{
		Platform: resolvedPlatform,
		DryRun:   controlCfg.DryRun,
		LeaderLock: ControlPlaneConfigFieldStatus{
			Value:     resolveLeaderLockKey(controlCfg, resolvedPlatform),
			Effective: true,
			Status:    "active",
		},
		LeaderLockTTL: ControlPlaneConfigFieldStatus{
			Value:     durationStatusValue(leaderLockTTL(controlCfg)),
			Effective: true,
			Status:    "active",
		},
		QuotaKeyTTLGrace: quotaStatus,
	}
}

func durationStatusValue(value time.Duration) string {
	if value <= 0 {
		return "0s"
	}
	return value.String()
}
