package productenrich

import (
	"fmt"
	"strings"

	"task-processor/internal/prompt"
)

type resolvedScorerPrompt struct {
	Key     string
	Source  string
	Version string
	Text    string
}

func normalizeScorerPromptKey(promptRef string, defaultKey string) string {
	normalized := strings.TrimSpace(promptRef)
	if normalized == "" {
		return defaultKey
	}
	replacer := strings.NewReplacer("/", ".", "-", "_")
	return replacer.Replace(normalized)
}

func resolveScorerPrompt(promptRef string, defaultKey string, vars map[string]any, fallback string) resolvedScorerPrompt {
	key := normalizeScorerPromptKey(promptRef, defaultKey)
	resolved := resolvedScorerPrompt{
		Key:     key,
		Source:  "fallback",
		Version: "default",
		Text:    fallback,
	}

	if prompt.GlobalRegistry == nil || !scorerPromptKeyExists(prompt.GlobalRegistry, key) {
		return resolved
	}

	rendered, err := prompt.GlobalRegistry.Render(key, vars, fallback)
	if err != nil {
		return resolved
	}
	if strings.TrimSpace(rendered) == "" {
		return resolved
	}

	resolved.Source = "registry"
	resolved.Text = rendered
	return resolved
}

func scorerPromptKeyExists(registry prompt.PromptRegistry, key string) bool {
	if registry == nil {
		return false
	}
	for _, candidate := range registry.Keys() {
		if candidate == key {
			return true
		}
	}
	return false
}

func resolveTextScoringPrompt(text string, baseScore float64) resolvedScorerPrompt {
	return resolveScorerPrompt(
		prompt.KProductEnrichLlmScorerTextScoring,
		prompt.KProductEnrichLlmScorerTextScoring,
		map[string]any{
			"Text":      text,
			"BaseScore": fmt.Sprintf("%.1f", baseScore),
		},
		fmt.Sprintf(`你是一个专业的产品描述质量评估专家。请对以下产品描述文本进行质量评分（0-100分）。

评分维度：
1. 信息完整性（30分）：是否包含产品名称、类别、主要特性、规格参数等关键信息
2. 描述清晰度（25分）：表达是否清晰、逻辑是否连贯、是否易于理解
3. 专业性（25分）：是否使用准确的专业术语、是否符合行业标准
4. 吸引力（20分）：是否能吸引潜在买家、是否突出产品优势

产品描述文本：
%s

参考评分（基于文本长度）：%.1f 分

评分标准：
- 90-100分：优秀，信息完整、表达专业、极具吸引力
- 80-89分：良好，信息较完整、表达清晰、有一定吸引力
- 70-79分：中等，基本信息完整、表达尚可
- 60-69分：及格，信息不够完整或表达不够清晰
- 0-59分：不及格，信息严重缺失或表达混乱

请以 JSON 格式返回评分结果：
{
  "score": 85,
  "reason": "简要说明评分理由（50字以内）",
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["不足1", "不足2"]
}

只返回 JSON，不要其他内容。`, text, baseScore),
	)
}

func resolveImageScoringPrompt(baseScore float64) resolvedScorerPrompt {
	return resolveScorerPrompt(
		prompt.KProductEnrichLlmScorerImageScoring,
		prompt.KProductEnrichLlmScorerImageScoring,
		map[string]any{
			"BaseScore": fmt.Sprintf("%.1f", baseScore),
		},
		fmt.Sprintf(`你是一个专业的产品图片质量评估专家。请对这张产品图片进行质量评分（0-100分）。

评分维度：
1. 清晰度（30分）：图片是否清晰、分辨率是否足够、是否有模糊或噪点
2. 专业性（25分）：拍摄角度、光线、背景是否专业、是否符合电商标准
3. 信息完整性（25分）：是否能清楚展示产品细节、是否有遮挡或缺失
4. 吸引力（20分）：构图是否美观、色彩是否协调、是否能吸引买家

参考评分（基于图片数量）：%.1f 分

评分标准：
- 90-100分：优秀，清晰专业、细节完整、极具吸引力
- 80-89分：良好，清晰度好、较专业、有吸引力
- 70-79分：中等，基本清晰、一般专业
- 60-69分：及格，清晰度或专业性不足
- 0-59分：不及格，模糊不清或严重不专业

请以 JSON 格式返回评分结果：
{
  "score": 85,
  "reason": "简要说明评分理由（50字以内）",
  "strengths": ["优点1", "优点2"],
  "weaknesses": ["不足1", "不足2"]
}

只返回 JSON，不要其他内容。`, baseScore),
	)
}
