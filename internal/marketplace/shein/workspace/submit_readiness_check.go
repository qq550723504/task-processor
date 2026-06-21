package workspace

// BuildSubmitReadinessCheck builds a SHEIN submit readiness check with taxonomy attached.
func BuildSubmitReadinessCheck(
	key string,
	label string,
	ok bool,
	message string,
	fieldPaths []string,
	suggestedAction string,
	warningOnly bool,
) ReadinessCheckSpec {
	return ReadinessCheckSpec{
		Key:             key,
		Label:           label,
		OK:              ok,
		Message:         message,
		FieldPaths:      append([]string(nil), fieldPaths...),
		SuggestedAction: suggestedAction,
		WarningOnly:     warningOnly,
		Taxonomy:        BuildReadinessTaxonomy(key, warningOnly),
	}
}
