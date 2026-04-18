package listingkit

func applyGenerationPanelResourceRecoveryPresentation(item *GenerationPanelResourceDescriptor) {
	if item == nil {
		return
	}
	profile := generationRecoveryProfileForHint(item.RecoveryHint)
	item.RecoverySeverity = profile.Severity
	item.RecoveryUrgency = profile.Urgency
	item.RecoveryCTAKind = profile.CTAKind
}
