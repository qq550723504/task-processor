package shein

import generationtopics "task-processor/internal/shein/generationtopics"

const (
	sheinGenerationPolicyMaxDirectives = generationtopics.SheinGenerationPolicyMaxDirectives
	sheinGenerationPolicyMaxChars      = generationtopics.SheinGenerationPolicyMaxChars
)

type GenerationTopicDefinition = generationtopics.Definition

type GenerationTopicResolution = generationtopics.Resolution

var sheinGenerationTopicDefinitions = generationtopics.SheinGenerationTopicDefinitions()

func ResolveSheinGenerationTopics(topicKeys []string) GenerationTopicResolution {
	return generationtopics.ResolveSheinTopics(topicKeys)
}

func ResolveSheinGenerationTopicKeys(topicKeys []string) ([]GenerationTopicDefinition, []string) {
	return generationtopics.ResolveSheinTopicKeys(topicKeys)
}

func buildSheinGenerationPolicySummary(topicKeys []string) string {
	return generationtopics.BuildSheinPolicySummary(topicKeys)
}

func assembleSheinGenerationPolicySummary(definitions []GenerationTopicDefinition, maxDirectives int, maxChars int) string {
	return generationtopics.AssembleSheinPolicySummary(definitions, maxDirectives, maxChars)
}

func normalizeGenerationTopicKey(topicKey string) string {
	return generationtopics.NormalizeKey(topicKey)
}
