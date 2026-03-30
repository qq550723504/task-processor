package httpx

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func writeMetricsResponse(w http.ResponseWriter, stats map[string]any) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	lines := buildPrometheusMetrics(stats)
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
}

func buildPrometheusMetrics(stats map[string]any) []string {
	if len(stats) == 0 {
		return nil
	}

	keys := make([]string, 0, len(stats))
	for key := range stats {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		metricName := "crawler_" + sanitizeMetricName(key)
		switch value := stats[key].(type) {
		case int:
			lines = append(lines, fmt.Sprintf("%s %d", metricName, value))
		case int64:
			lines = append(lines, fmt.Sprintf("%s %d", metricName, value))
		case float64:
			lines = append(lines, fmt.Sprintf("%s %g", metricName, value))
		case float32:
			lines = append(lines, fmt.Sprintf("%s %g", metricName, value))
		case map[string]int:
			lines = appendLabeledIntMetrics(lines, metricName, value)
		case map[string]int64:
			lines = appendLabeledInt64Metrics(lines, metricName, value)
		case map[string]map[string]int64:
			lines = appendNestedInt64Metrics(lines, metricName, value)
		case map[string]any:
			lines = appendLabeledAnyMetrics(lines, metricName, value)
		}
	}
	return lines
}

func appendLabeledIntMetrics(lines []string, metricName string, values map[string]int) []string {
	labels := sortedKeys(values)
	for _, label := range labels {
		lines = append(lines, fmt.Sprintf("%s{key=%q} %d", metricName, label, values[label]))
	}
	return lines
}

func appendLabeledInt64Metrics(lines []string, metricName string, values map[string]int64) []string {
	labels := sortedKeys(values)
	for _, label := range labels {
		lines = append(lines, fmt.Sprintf("%s{key=%q} %d", metricName, label, values[label]))
	}
	return lines
}

func appendLabeledAnyMetrics(lines []string, metricName string, values map[string]any) []string {
	labels := make([]string, 0, len(values))
	for key := range values {
		labels = append(labels, key)
	}
	sort.Strings(labels)
	for _, label := range labels {
		switch v := values[label].(type) {
		case int:
			lines = append(lines, fmt.Sprintf("%s{key=%q} %d", metricName, label, v))
		case int64:
			lines = append(lines, fmt.Sprintf("%s{key=%q} %d", metricName, label, v))
		case float64:
			lines = append(lines, fmt.Sprintf("%s{key=%q} %g", metricName, label, v))
		case float32:
			lines = append(lines, fmt.Sprintf("%s{key=%q} %g", metricName, label, v))
		}
	}
	return lines
}

func appendNestedInt64Metrics(lines []string, metricName string, values map[string]map[string]int64) []string {
	outerKeys := sortedKeys(values)
	for _, outerKey := range outerKeys {
		innerValues := values[outerKey]
		innerKeys := sortedKeys(innerValues)
		for _, innerKey := range innerKeys {
			lines = append(lines, fmt.Sprintf(
				"%s{region=%q,type=%q} %d",
				metricName,
				outerKey,
				innerKey,
				innerValues[innerKey],
			))
		}
	}
	return lines
}

func sanitizeMetricName(input string) string {
	var b strings.Builder
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	result := b.String()
	result = strings.Trim(result, "_")
	result = strings.ReplaceAll(result, "__", "_")
	if result == "" {
		return "unknown"
	}
	return result
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
