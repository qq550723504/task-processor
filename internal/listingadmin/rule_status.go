package listingadmin

const (
	// RuleStatusEnabled is the persisted status for a rule that participates in runtime validation.
	RuleStatusEnabled int16 = 0
	// RuleStatusDisabled is the persisted status for a rule that must not participate in runtime validation.
	RuleStatusDisabled int16 = 1
)

func IsRuleStatusEnabled(status int16) bool {
	return status == RuleStatusEnabled
}
