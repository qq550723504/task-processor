package listingkit

import "strings"

func resolveTemporalRequestPlatform(req *ExecuteGenerationActionRequest) string {
	if req != nil && req.Target != nil {
		if req.Target.QueueQuery != nil {
			if platform := normalizeTemporalRequestPlatform(req.Target.QueueQuery.Platform); platform != "" {
				return platform
			}
		}
		if req.Target.NavigationTarget != nil {
			if req.Target.NavigationTarget.QueueQuery != nil {
				if platform := normalizeTemporalRequestPlatform(req.Target.NavigationTarget.QueueQuery.Platform); platform != "" {
					return platform
				}
			}
			if req.Target.NavigationTarget.SessionQuery != nil {
				if platform := normalizeTemporalRequestPlatform(req.Target.NavigationTarget.SessionQuery.Platform); platform != "" {
					return platform
				}
			}
			if req.Target.NavigationTarget.PreviewQuery != nil {
				if platform := normalizeTemporalRequestPlatform(req.Target.NavigationTarget.PreviewQuery.Platform); platform != "" {
					return platform
				}
			}
			if req.Target.NavigationTarget.ActionTarget != nil && req.Target.NavigationTarget.ActionTarget.QueueQuery != nil {
				if platform := normalizeTemporalRequestPlatform(req.Target.NavigationTarget.ActionTarget.QueueQuery.Platform); platform != "" {
					return platform
				}
			}
			if req.Target.NavigationTarget.Descriptor != nil {
				for _, read := range req.Target.NavigationTarget.Descriptor.FollowUpReads {
					if read.Query != nil {
						if platform := normalizeTemporalRequestPlatform(read.Query.Platform); platform != "" {
							return platform
						}
					}
				}
			}
			if req.Target.NavigationTarget.ActionTarget != nil {
				if platform := resolveTemporalRequestPlatform(&ExecuteGenerationActionRequest{Target: req.Target.NavigationTarget.ActionTarget}); platform != "" {
					return platform
				}
			}
		}
		if req.Target.NavigationTarget != nil && req.Target.NavigationTarget.ActionTarget != nil && req.Target.NavigationTarget.ActionTarget.QueueQuery != nil {
			if platform := normalizeTemporalRequestPlatform(req.Target.NavigationTarget.ActionTarget.QueueQuery.Platform); platform != "" {
				return platform
			}
		}
	}
	return "shein"
}

func normalizeTemporalRequestPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}
