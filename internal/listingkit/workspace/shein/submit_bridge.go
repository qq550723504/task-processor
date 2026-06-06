package shein

import sheinworkspace "task-processor/internal/workspace/shein"

func BuildSubmitChecklist[R any, H any](readiness *SubmitReadiness[R, H]) *SubmitChecklist[R, H] {
	return sheinworkspace.BuildSubmitChecklist(readiness, sheinworkspace.SubmitChecklistGroupForKey)
}
