package listingkit

import "strings"

func looksLikeStudioSubmitRequestToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) < 6 || len(token) > 9 || !strings.HasPrefix(token, "R") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func looksLikeStudioSubmitTaskToken(token string) bool {
	token = strings.TrimSpace(strings.ToUpper(token))
	if len(token) != 9 || !strings.HasPrefix(token, "T") {
		return false
	}
	for _, r := range token[1:] {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func resolveStudioSubmitStyleSuffix(task *Task) string {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return ""
	}
	if value := firstNonEmptyString(
		sheinStudioStyleID(task.Request.Options.SheinStudio),
		task.Request.Options.SDS.StyleID,
	); strings.TrimSpace(value) != "" {
		return value
	}
	return deriveStudioSubmitStyleSuffix(
		task.Request.Text,
		task.Request.Options.SDS.ProductEnglishName,
		task.Request.Options.SDS.ProductName,
	)
}

func sheinStudioStyleID(options *SheinStudioOptions) string {
	if options == nil {
		return ""
	}
	return options.StyleID
}

func deriveStudioSubmitStyleSuffix(values ...string) string {
	stopwords := map[string]bool{
		"THE": true, "AND": true, "FOR": true, "WITH": true, "FROM": true,
		"FRESH": true, "SDS": true, "TASK": true, "PUBLIC": true, "IMAGE": true,
		"RETRY": true, "TEST": true, "DEFAULT": true, "DESIGN": true,
	}
	tokens := make([]string, 0, 8)
	for _, value := range values {
		for _, token := range tokenizeStudioStyleSuffixWords(value) {
			if stopwords[token] {
				continue
			}
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	shortToken := ""
	longToken := ""
	for _, token := range tokens {
		if shortToken == "" && len(token) >= 2 && len(token) <= 3 {
			shortToken = token
		}
		if len(token) > len(longToken) {
			longToken = token
		}
	}
	if shortToken != "" && longToken != "" && !strings.EqualFold(shortToken, longToken) {
		return normalizeStyleIDSuffix(shortToken + longToken)
	}
	var builder strings.Builder
	for _, token := range tokens {
		builder.WriteString(token)
		if builder.Len() >= 8 {
			break
		}
	}
	return normalizeStyleIDSuffix(builder.String())
}

func tokenizeStudioStyleSuffixWords(value string) []string {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return nil
	}
	tokens := make([]string, 0, 8)
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			current.WriteRune(r)
		default:
			flush()
		}
	}
	flush()
	return tokens
}

func studioSubmitTaskDiscriminator(taskID string) string {
	taskID = strings.TrimSpace(strings.ToUpper(taskID))
	if taskID == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("T")
	for _, r := range taskID {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 9 {
			break
		}
	}
	if b.Len() <= 1 {
		return ""
	}
	return b.String()
}

func studioSubmitRequestDiscriminator(requestID string) string {
	requestID = strings.TrimSpace(strings.ToUpper(requestID))
	if requestID == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("R")
	for _, r := range requestID {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if b.Len() >= 9 {
			break
		}
	}
	if b.Len() <= 1 {
		return ""
	}
	return b.String()
}

func combineStudioSubmitDiscriminators(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "-")
}
