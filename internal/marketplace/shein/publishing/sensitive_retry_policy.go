package publishing

// ShouldRetrySensitiveWordSubmit reports whether a failed submit can be retried after sensitive-word cleanup.
func ShouldRetrySensitiveWordSubmit(action string, hasResponse, hasResponseError bool, validationNoteCount int, hasExecutor bool) bool {
	return action == "publish" &&
		hasResponse &&
		hasResponseError &&
		validationNoteCount > 0 &&
		hasExecutor
}
