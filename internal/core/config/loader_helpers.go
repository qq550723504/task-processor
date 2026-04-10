package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

func getDuration(v *viper.Viper, key string, defaultSeconds int) time.Duration {
	seconds := v.GetInt(key)
	if seconds == 0 {
		seconds = defaultSeconds
	}
	return time.Duration(seconds) * time.Second
}

func getInt64Slice(v *viper.Viper, key string) []int64 {
	intSlice := v.GetIntSlice(key)
	if len(intSlice) == 0 {
		raw := strings.TrimSpace(v.GetString(key))
		if raw == "" {
			return nil
		}

		parts := strings.Split(raw, ",")
		result := make([]int64, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			value, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil
			}
			result = append(result, value)
		}
		return result
	}

	result := make([]int64, len(intSlice))
	for i, value := range intSlice {
		result[i] = int64(value)
	}
	return result
}

func getIntSlice(v *viper.Viper, key string) []int {
	intSlice := v.GetIntSlice(key)
	if len(intSlice) > 0 {
		return intSlice
	}

	raw := strings.TrimSpace(v.GetString(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		result = append(result, value)
	}
	return result
}

func getStringSlice(v *viper.Viper, key string) []string {
	stringSlice := v.GetStringSlice(key)
	if len(stringSlice) > 0 {
		result := make([]string, 0, len(stringSlice))
		for _, item := range stringSlice {
			item = strings.TrimSpace(strings.Trim(item, ","))
			if item == "" {
				continue
			}
			result = append(result, item)
		}
		return result
	}

	raw := strings.TrimSpace(v.GetString(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func getStringFromMap(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromMap(m map[string]any, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}
