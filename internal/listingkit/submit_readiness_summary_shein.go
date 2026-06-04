package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/workspace/shein"
)

type sheinSubmitReadinessSummaryShape struct {
	blockingLabel       string
	warningLabel        string
	prependFirstBlocker bool
}

func shapeSheinSubmitReadinessSummary(
	readiness *SheinSubmitReadiness,
	shape sheinSubmitReadinessSummaryShape,
) *SheinSubmitReadiness {
	if readiness == nil {
		return nil
	}
	if len(readiness.BlockingItems) > 0 {
		if shape.prependFirstBlocker {
			if message := strings.TrimSpace(readiness.BlockingItems[0].Message); message != "" {
				readiness.Summary = append([]string{message}, readiness.Summary...)
			}
		}
		if label := strings.TrimSpace(shape.blockingLabel); label != "" {
			readiness.Summary = append(readiness.Summary, label+sheinworkspace.JoinReadinessLabels(readiness.BlockingItems, "、"))
		}
	}
	if len(readiness.WarningItems) > 0 {
		if label := strings.TrimSpace(shape.warningLabel); label != "" {
			readiness.Summary = append(readiness.Summary, label+sheinworkspace.JoinReadinessLabels(readiness.WarningItems, "、"))
		}
	}
	readiness.Summary = uniqueStrings(readiness.Summary)
	return readiness
}
