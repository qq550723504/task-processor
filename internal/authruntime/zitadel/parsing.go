package zitadel

import (
	"encoding/json"
	"sort"
	"strings"
)

func ParseRoles(data []byte) []string {
	return parseRoles(data, "")
}

func ParseRolesForProject(data []byte, projectID string) []string {
	return parseRoles(data, strings.TrimSpace(projectID))
}

func parseRoles(data []byte, projectID string) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	roles := make([]string, 0, 4)
	add := func(value string) {
		role := strings.TrimSpace(value)
		if role == "" {
			return
		}
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}

	keys := make([]string, 0, 4)
	if projectID != "" {
		keys = append(keys, "urn:zitadel:iam:org:project:"+projectID+":roles")
	}
	keys = append(keys, "urn:zitadel:iam:org:project:roles", "roles", "role")
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}

		var claimValue any
		if err := json.Unmarshal(value, &claimValue); err == nil {
			addRoleClaimValue(claimValue, add)
		}
	}

	return roles
}

func addRoleClaimValue(value any, add func(string)) {
	switch typed := value.(type) {
	case string:
		for _, role := range strings.Split(typed, ",") {
			add(role)
		}
	case []any:
		for _, item := range typed {
			addRoleClaimValue(item, add)
		}
	case map[string]any:
		roles := make([]string, 0, len(typed))
		for role := range typed {
			roles = append(roles, role)
		}
		sort.Strings(roles)
		for _, role := range roles {
			add(role)
		}
	}
}

func StringSliceToSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func valueInSet(value string, set map[string]struct{}) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	_, ok := set[trimmed]
	return ok
}
