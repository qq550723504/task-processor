package workspace

// SubmitPayloadValidationReadinessInput carries prepared payload validation results for readiness checks.
type SubmitPayloadValidationReadinessInput struct {
	Ready   bool
	Message string
}

// BuildSubmitPayloadValidationReadinessChecks builds readiness checks for prepared payload validation failures.
func BuildSubmitPayloadValidationReadinessChecks(input SubmitPayloadValidationReadinessInput) []ReadinessCheckSpec {
	if input.Ready {
		return nil
	}
	return []ReadinessCheckSpec{
		BuildSubmitReadinessCheck(
			"variants",
			"发布载荷结构",
			false,
			input.Message,
			[]string{"shein.preview_product", "shein.request_draft.skc_list"},
			"确认规格",
			false,
		),
	}
}
