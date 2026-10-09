package imageagent

// This is a view of original generation/settlement facts, never a new resource
// decision. Callers must resolve the original fact through its effect owner.
func ImageGenerationClosure(fact GenerationFact) (*ImageSlotClosure, error) {
	if fact.Validate() != nil || fact.Intent.InputProtocol != ImageSetSchema {
		return nil, ErrRevisionConflict
	}
	if fact.Settlement == (GenerationSettlementReceipt{}) || fact.State != GenerationSucceeded && fact.State != GenerationNoEffect {
		return nil, ErrCommandBlocked
	}
	closure := &ImageSlotClosure{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, SettlementProofDigest: fact.Settlement.ProofDigest}
	if fact.State == GenerationSucceeded {
		closure.Kind, closure.Points = "settled", fact.Intent.Points
	} else {
		closure.Kind = "no_generation"
	}
	return closure, nil
}
