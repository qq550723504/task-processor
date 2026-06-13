package httpapi

import sheinpub "task-processor/internal/publishing/shein"

func configureSheinSubmitPrepRuntime(repositories *builtRepositories) {
	if repositories == nil {
		return
	}
	sheinpub.ConfigureSubmitPrepRepositories(
		repositories.sensitiveWordRepository,
		repositories.generationTopicPolicyRepository,
		repositories.generationTopicOverrideRepository,
	)
	sheinpub.SetGenerationTopicPolicyRepository(repositories.generationTopicPolicyRepository)
	sheinpub.SetGenerationTopicOverrideRepository(repositories.generationTopicOverrideRepository)
}
