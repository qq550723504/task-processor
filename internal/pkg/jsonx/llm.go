package jsonx

import "strings"

// CleanLLMResponse 清理 LLM 响应中可能包含的 markdown 代码块包裹。
// 处理 ```json ... ``` 和 ``` ... ``` 两种格式。
func CleanLLMResponse(s string) string {
	s = strings.TrimSpace(s)
	if after, found := strings.CutPrefix(s, "```json"); found {
		s = after
	} else if after, found := strings.CutPrefix(s, "```"); found {
		s = after
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
