package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/productenrich"
	"task-processor/internal/prompt"
)

const defaultImagePrompt = `Analyze this product image and extract the following attributes in JSON format:
{
  "color": "the main color of the product",
  "material": "the material the product is made of",
  "scene": "the scene or context where the product is shown",
  "usage": "the intended use or purpose of the product"
}

Only return the JSON object, no additional text.`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	imageURL := os.Args[1]
	promptText := loadPrompt()
	if len(os.Args) >= 3 {
		promptText = os.Args[2]
	}

	cfg := config.LoadConfig()
	llmManager, err := productenrich.NewLLMManagerAdapter(cfg.OpenAI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create llm manager: %v\n", err)
		os.Exit(1)
	}

	client, clientName, err := getVisionClient(llmManager)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get analyze-image client: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	start := time.Now()
	rawResponse, err := client.AnalyzeImage(ctx, imageURL, promptText)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analyze image failed after %.1fs: %v\n", elapsed.Seconds(), err)
		os.Exit(1)
	}

	cleaned := jsonx.CleanLLMResponse(rawResponse)
	var attributes productenrich.ImageAttributes
	parseErr := json.Unmarshal([]byte(cleaned), &attributes)

	fmt.Println("=== AnalyzeImage Test Tool ===")
	fmt.Printf("client: %s\n", clientName)
	fmt.Printf("image: %s\n", imageURL)
	fmt.Printf("duration: %.1fs\n", elapsed.Seconds())
	fmt.Println()
	fmt.Println("prompt:")
	fmt.Println(promptText)
	fmt.Println()
	fmt.Println("raw_response:")
	fmt.Println(rawResponse)
	fmt.Println()
	fmt.Println("cleaned_response:")
	fmt.Println(cleaned)
	fmt.Println()

	if parseErr != nil {
		fmt.Printf("json_parse: failed: %v\n", parseErr)
		return
	}

	formatted, err := json.MarshalIndent(attributes, "", "  ")
	if err != nil {
		fmt.Printf("json_format: failed: %v\n", err)
		return
	}

	fmt.Println("parsed_attributes:")
	fmt.Println(string(formatted))
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run ./cmd/test-analyzeimage <image_url>")
	fmt.Println("  go run ./cmd/test-analyzeimage <image_url> <prompt>")
}

func loadPrompt() string {
	logger := corelogger.GetGlobalLogManager().GetRawLogger()
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	promptsDir := filepath.Join(projectRoot, "prompts")

	if err := prompt.InitGlobal(context.Background(), promptsDir, false, nil); err != nil {
		logger.WithError(err).Warn("failed to initialize prompt registry, using built-in prompt")
		return defaultImagePrompt
	}

	if prompt.GlobalRegistry == nil {
		return defaultImagePrompt
	}

	return prompt.GlobalRegistry.Get(prompt.KProductEnrichUnderstandingAnalyzeImage, defaultImagePrompt)
}

func getVisionClient(llmManager productenrich.LLMManager) (productenrich.LLMClient, string, error) {
	client, err := llmManager.GetClient("vision")
	if err == nil && client != nil {
		return client, "vision", nil
	}

	client, fallbackErr := llmManager.GetClient("default")
	if fallbackErr != nil || client == nil {
		if err != nil {
			return nil, "", fmt.Errorf("vision client error: %w; default client error: %v", err, fallbackErr)
		}
		return nil, "", fmt.Errorf("default client unavailable: %w", fallbackErr)
	}

	return client, "default", nil
}
