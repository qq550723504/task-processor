package commercetool

import (
	"errors"
	"strings"
)

const maxAIInvocationIDBytes = 128

func validatedAIInvocationID(definition Definition, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if definition.Usage.Owner != UsageOwnerAICapability || definition.Risk != RiskPropose {
		return "", errors.New("AI invocation ID is not applicable to this tool")
	}
	if !isSafeAIInvocationID(value) {
		return "", errors.New("AI invocation ID is invalid")
	}

	return value, nil
}

func isSafeAIInvocationID(value string) bool {
	if len(value) == 0 || len(value) > maxAIInvocationIDBytes || value != strings.TrimSpace(value) {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if isASCIIAlphanumeric(character) {
			continue
		}
		if index == 0 || !isSafeAIInvocationIDSeparator(character) {
			return false
		}
	}

	return true
}

func isASCIIAlphanumeric(character byte) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9'
}

func isSafeAIInvocationIDSeparator(character byte) bool {
	switch character {
	case '.', '_', ':', '+', '-':
		return true
	default:
		return false
	}
}
