package httpapi

import (
	"encoding/json"
	"strings"
)

func firstNonEmptyZitadelValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseZitadelRoles(data []byte) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	roles := []string{}
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
	for _, key := range []string{"urn:zitadel:iam:org:project:roles", "roles", "role"} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		var list []string
		if err := json.Unmarshal(value, &list); err == nil {
			for _, role := range list {
				add(role)
			}
			continue
		}
		var single string
		if err := json.Unmarshal(value, &single); err == nil {
			for _, role := range strings.Split(single, ",") {
				add(role)
			}
			continue
		}
		var roleMap map[string]any
		if err := json.Unmarshal(value, &roleMap); err == nil {
			for role := range roleMap {
				add(role)
			}
		}
	}
	return roles
}

func stringSliceToSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func valueInSet(value string, set map[string]struct{}) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	_, ok := set[trimmed]
	return ok
}
