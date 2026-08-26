package publishing

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatWeight formats a TEMU weight in pounds with API bounds and precision.
func FormatWeight(value string) string {
	return formatNumericString(value, []string{"lb", "磅"}, 0.22, 999.99, "%.2f")
}

// FormatDimension formats a TEMU dimension in inches with API bounds and precision.
func FormatDimension(value string) string {
	return formatNumericString(value, []string{"in", "英寸"}, 3.9, 9999.9, "%.1f")
}

func formatNumericString(value string, units []string, defaultValue, maxValue float64, format string) string {
	defaultString := fmt.Sprintf(format, defaultValue)
	if value == "" {
		return defaultString
	}

	clean := strings.TrimSpace(value)
	for _, unit := range units {
		clean = strings.ReplaceAll(clean, unit, "")
	}
	clean = strings.TrimSpace(clean)

	parsed, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return defaultString
	}
	if parsed <= 0 {
		parsed = defaultValue
	} else if parsed > maxValue {
		parsed = maxValue
	}
	return fmt.Sprintf(format, parsed)
}
