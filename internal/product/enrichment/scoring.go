package enrichment

func scoreCandidate(candidate Candidate, policy PolicySnapshot) QualityScore {
	quality := QualityScore{}
	if len(candidate.Changes) > 0 {
		backed := 0
		for _, change := range candidate.Changes {
			if len(change.EvidenceIDs) > 0 {
				backed++
			}
		}
		quality.EvidenceCoverage = float64(backed) / float64(len(candidate.Changes)) * 100
	}

	if len(policy.RequiredFields) == 0 {
		quality.RequiredFieldCoverage = 100
		quality.Overall = quality.EvidenceCoverage
		return quality
	}
	changed := make(map[string]struct{}, len(candidate.Changes))
	for _, change := range candidate.Changes {
		changed[change.Field] = struct{}{}
	}
	present := 0
	for _, field := range policy.RequiredFields {
		if _, ok := changed[field]; ok {
			present++
		}
	}
	quality.RequiredFieldCoverage = float64(present) / float64(len(policy.RequiredFields)) * 100
	quality.Overall = (quality.EvidenceCoverage + quality.RequiredFieldCoverage) / 2
	return quality
}
