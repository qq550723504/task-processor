package imageagent

import "task-processor/internal/agentconfig"

func (c ImageSlotClosure) Valid(attempt int) bool {
	if c.Kind == "not_dispatched" {
		return attempt == 0 && c == (ImageSlotClosure{Kind: "not_dispatched"})
	}
	return attempt > 0 && agentconfig.ImageDigest(c.IntentID) && agentconfig.ImageDigest(c.Fingerprint) && agentconfig.ImageDigest(c.SettlementProofDigest) && (c.Kind == "settled" && c.Points > 0 || c.Kind == "no_generation" && c.Points == 0)
}

func (q ImageGenerationQuote) MatchesGenerationIntent(intent GenerationIntent) bool {
	return q.Valid() && q.Provider == intent.Provider && q.Model == intent.Model && q.Protocol == intent.Protocol && q.Resolution == intent.Resolution && q.Quality == intent.Quality && q.PriceVersion == intent.PriceVersion && q.Points == intent.Points && q.RouteReference == intent.RouteReference && q.CredentialReference == intent.CredentialReference && q.ConfigurationVersion == intent.ConfigurationVersion
}

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
