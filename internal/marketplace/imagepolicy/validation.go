package imagepolicy

import "math"

const (
	maxPolicyIdentifierBytes = 64
	maxPolicyVersionBytes    = 128
	maxPoliciesPerSet        = 4096
	maxPolicySetBytes        = 4 << 20
)

func addPolicyBytes(used *int, policy Policy) bool {
	values := [...]string{
		policy.Key.Marketplace,
		policy.Key.Country,
		policy.Key.Family,
		policy.Key.SceneCategory,
		policy.SceneDefaults.SceneCategory,
		policy.SceneDefaults.SceneStyle,
		policy.SceneDefaults.BackgroundTone,
		policy.SceneDefaults.Composition,
		policy.SceneDefaults.PropsLevel,
		policy.SceneDefaults.AudienceHint,
		policy.SceneDefaults.CustomSceneHint,
		policy.SceneDefaults.SlotRole,
		policy.SceneDefaults.SlotBrief,
	}
	for _, value := range values {
		if len(value) > maxPolicySetBytes-*used {
			return false
		}
		*used += len(value)
	}
	for _, referenceID := range policy.SceneDefaults.StyleReferenceIDs {
		if len(referenceID) > maxPolicySetBytes-*used {
			return false
		}
		*used += len(referenceID)
	}
	return true
}

func validPolicyVersion(value string) bool {
	if len(value) == 0 || len(value) > maxPolicyVersionBytes || !isLowerASCIILetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !isLowerASCIILetter(character) && !isASCIIDigit(character) && character != '-' && character != '_' && character != '.' && character != '/' {
			return false
		}
	}
	last := value[len(value)-1]
	return isLowerASCIILetter(last) || isASCIIDigit(last)
}

func validPolicyKey(key PolicyKey) bool {
	return validIdentifier(key.Marketplace) &&
		validCountry(key.Country) &&
		validIdentifier(key.Family) &&
		validIdentifier(key.SceneCategory)
}

func validThresholds(thresholds Thresholds) bool {
	return validUnitInterval(thresholds.MainReview) &&
		validUnitInterval(thresholds.WhiteBackgroundReview) &&
		validUnitInterval(thresholds.WhiteCanvasPenalty)
}

func validUnitInterval(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validCountry(value string) bool {
	return len(value) == 2 && isLowerASCIILetter(value[0]) && isLowerASCIILetter(value[1])
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > maxPolicyIdentifierBytes || !isLowerASCIILetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !isLowerASCIILetter(character) && !isASCIIDigit(character) && character != '-' && character != '_' {
			return false
		}
	}
	last := value[len(value)-1]
	return isLowerASCIILetter(last) || isASCIIDigit(last)
}

func isLowerASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z'
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}
