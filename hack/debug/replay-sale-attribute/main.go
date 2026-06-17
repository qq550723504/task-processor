package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type debugFileData struct {
	TaskID       string `json:"task_id"`
	ProductID    string `json:"product_id"`
	SystemPrompt string `json:"system_prompt"`
	UserPrompt   string `json:"user_prompt"`
	Response     string `json:"response"`
	Error        string `json:"error"`
	TokensUsed   int    `json:"tokens_used"`
	Model        string `json:"model"`
	FinishReason string `json:"finish_reason"`
	IsTruncated  bool   `json:"is_truncated"`
}

type replayOutput struct {
	DebugFile         string `json:"debug_file"`
	TaskID            string `json:"task_id"`
	ProductID         string `json:"product_id"`
	RequestModel      string `json:"request_model"`
	RequestBaseURL    string `json:"request_base_url"`
	ResponseModel     string `json:"response_model"`
	FinishReason      string `json:"finish_reason"`
	TokensUsed        int    `json:"tokens_used"`
	ResponseLength    int    `json:"response_length"`
	LikelyTruncated   bool   `json:"likely_truncated"`
	JSONValid         bool   `json:"json_valid"`
	ParseError        string `json:"parse_error,omitempty"`
	ExpectedVariants  int    `json:"expected_variants,omitempty"`
	ActualVariants    int    `json:"actual_variants,omitempty"`
	VariantMatch      bool   `json:"variant_match"`
	OriginalModel     string `json:"original_model,omitempty"`
	OriginalTokens    int    `json:"original_tokens,omitempty"`
	OriginalTruncated bool   `json:"original_is_truncated"`
	Response          string `json:"response"`
}

type replayAnalysis struct {
	JSONValid            bool
	ParseError           string
	ExpectedVariantCount int
	VariantCount         int
	VariantCountMatches  bool
}

type replayPayload struct {
	Variants []json.RawMessage `json:"variants"`
}

func main() {
	var (
		debugFile     = flag.String("file", "", "debug JSON file path")
		configFile    = flag.String("config", "config/config-dev.yaml", "config file path")
		modelOverride = flag.String("model", "", "override model")
		baseOverride  = flag.String("base-url", "", "override OpenAI base URL")
		keyOverride   = flag.String("api-key", "", "override OpenAI API key")
		timeoutSec    = flag.Int("timeout", 0, "override OpenAI timeout seconds")
		outputFile    = flag.String("output", "", "optional JSON output file path")
		printResponse = flag.Bool("print-response", false, "print raw response body")
	)
	flag.Parse()

	if strings.TrimSpace(*debugFile) == "" {
		fatalf("missing -file")
	}

	debugData, err := loadDebugFile(*debugFile)
	if err != nil {
		fatalf("load debug file failed: %v", err)
	}
	if strings.TrimSpace(debugData.SystemPrompt) == "" || strings.TrimSpace(debugData.UserPrompt) == "" {
		fatalf("debug file is missing system_prompt or user_prompt")
	}

	cfg, err := config.LoadConfigFromFile(*configFile)
	if err != nil {
		fatalf("load config failed: %v", err)
	}

	clientCfg := cfg.OpenAI.ToClientConfig()
	if strings.TrimSpace(*modelOverride) != "" {
		clientCfg.Model = strings.TrimSpace(*modelOverride)
	}
	if strings.TrimSpace(*baseOverride) != "" {
		clientCfg.BaseURL = strings.TrimSpace(*baseOverride)
	}
	if strings.TrimSpace(*keyOverride) != "" {
		clientCfg.APIKey = strings.TrimSpace(*keyOverride)
	}
	if *timeoutSec > 0 {
		clientCfg.Timeout = time.Duration(*timeoutSec) * time.Second
	}

	client := openaiclient.NewClient(clientCfg)
	if client == nil {
		fatalf("create OpenAI client failed")
	}
	defer func() { _ = client.Close() }()

	req := &openaiclient.ChatCompletionRequest{
		Model: clientCfg.Model,
		Messages: []openaiclient.ChatCompletionMessage{
			{Role: "system", Content: debugData.SystemPrompt},
			{Role: "user", Content: debugData.UserPrompt},
		},
		Temperature:    float32Ptr(0.1),
		Seed:           intPtr(42),
		ResponseFormat: "json_object",
	}

	fmt.Printf("=== Replay Sale Attribute Debug ===\n")
	fmt.Printf("Debug file: %s\n", filepath.Clean(*debugFile))
	fmt.Printf("Task ID: %s\n", debugData.TaskID)
	fmt.Printf("Product ID: %s\n", debugData.ProductID)
	fmt.Printf("Original model: %s\n", debugData.Model)
	fmt.Printf("Original finish reason: %s\n", debugData.FinishReason)
	fmt.Printf("Original tokens: %d\n", debugData.TokensUsed)
	fmt.Printf("Original truncated flag: %v\n", debugData.IsTruncated)
	fmt.Printf("Replay model: %s\n", clientCfg.Model)
	fmt.Printf("Replay base URL: %s\n", clientCfg.BaseURL)
	fmt.Printf("Replay timeout: %s\n", clientCfg.Timeout)
	fmt.Printf("\nSending request...\n\n")

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		fatalf("replay request failed: %v", err)
	}
	if len(resp.Choices) == 0 {
		fatalf("replay request returned no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	likelyTruncated := isLikelyTruncatedResponse(content, resp.Choices[0].FinishReason)
	analysis := analyzeReplayResponse(debugData.UserPrompt, content)

	fmt.Printf("=== Replay Result ===\n")
	fmt.Printf("Response model: %s\n", resp.Model)
	fmt.Printf("Finish reason: %s\n", resp.Choices[0].FinishReason)
	fmt.Printf("Tokens used: prompt=%d completion=%d total=%d\n", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
	fmt.Printf("Response length: %d\n", len(content))
	fmt.Printf("Likely truncated: %v\n", likelyTruncated)
	fmt.Printf("JSON valid: %v\n", analysis.JSONValid)
	if analysis.ParseError != "" {
		fmt.Printf("Parse error: %s\n", analysis.ParseError)
	}
	if analysis.ExpectedVariantCount > 0 {
		fmt.Printf("Variant count: %d/%d\n", analysis.VariantCount, analysis.ExpectedVariantCount)
		fmt.Printf("Variant count matches: %v\n", analysis.VariantCountMatches)
	}

	if *printResponse {
		fmt.Printf("\n=== Raw Response ===\n%s\n", content)
	}

	if strings.TrimSpace(*outputFile) != "" {
		output := replayOutput{
			DebugFile:         filepath.Clean(*debugFile),
			TaskID:            debugData.TaskID,
			ProductID:         debugData.ProductID,
			RequestModel:      clientCfg.Model,
			RequestBaseURL:    clientCfg.BaseURL,
			ResponseModel:     resp.Model,
			FinishReason:      resp.Choices[0].FinishReason,
			TokensUsed:        resp.Usage.TotalTokens,
			ResponseLength:    len(content),
			LikelyTruncated:   likelyTruncated,
			JSONValid:         analysis.JSONValid,
			ParseError:        analysis.ParseError,
			ExpectedVariants:  analysis.ExpectedVariantCount,
			ActualVariants:    analysis.VariantCount,
			VariantMatch:      analysis.VariantCountMatches,
			OriginalModel:     debugData.Model,
			OriginalTokens:    debugData.TokensUsed,
			OriginalTruncated: debugData.IsTruncated,
			Response:          content,
		}
		if err := writeJSON(*outputFile, output); err != nil {
			fatalf("write output failed: %v", err)
		}
		fmt.Printf("Replay output saved: %s\n", filepath.Clean(*outputFile))
	}
}

func loadDebugFile(path string) (*debugFileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var out debugFileData
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func isLikelyTruncatedResponse(content, finishReason string) bool {
	if finishReason == "length" {
		return true
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}

	cleaned := strings.TrimSpace(strings.TrimPrefix(trimmed, "```json"))
	cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "```"))
	cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, "```"))

	if !strings.HasPrefix(cleaned, "{") {
		return false
	}
	if strings.Count(cleaned, "{") > strings.Count(cleaned, "}") {
		return true
	}
	if strings.Count(cleaned, "[") > strings.Count(cleaned, "]") {
		return true
	}

	lastChar := cleaned[len(cleaned)-1]
	switch lastChar {
	case '{', '[', ':', ',':
		return true
	default:
		return false
	}
}

func analyzeReplayResponse(userPrompt, content string) replayAnalysis {
	cleaned := cleanLLMResponse(content)
	expected := expectedVariantCountFromPrompt(userPrompt)

	var payload replayPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return replayAnalysis{
			JSONValid:            false,
			ParseError:           err.Error(),
			ExpectedVariantCount: expected,
		}
	}

	variantCount := len(payload.Variants)
	matches := expected == 0 || expected == variantCount
	return replayAnalysis{
		JSONValid:            true,
		ExpectedVariantCount: expected,
		VariantCount:         variantCount,
		VariantCountMatches:  matches,
	}
}

func cleanLLMResponse(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func expectedVariantCountFromPrompt(userPrompt string) int {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)Generate\s+(\d+)\s+variants`),
		regexp.MustCompile(`(?i)for\s+(\d+)\s+Amazon\s+products`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(userPrompt)
		if len(match) != 2 {
			continue
		}
		n, err := strconv.Atoi(match[1])
		if err == nil {
			return n
		}
	}
	return 0
}

func float32Ptr(v float32) *float32 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
