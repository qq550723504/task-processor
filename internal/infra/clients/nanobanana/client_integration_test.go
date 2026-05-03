package nanobanana

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestClientGenerateImageConcurrentLive(t *testing.T) {
	if os.Getenv("NANOBANANA_LIVE_TEST") == "" {
		t.Skip("set NANOBANANA_LIVE_TEST=1 to run live nanobanana integration test")
	}

	client := NewClient(Config{
		APIKey:       os.Getenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY"),
		Model:        os.Getenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL"),
		SubmitURL:    os.Getenv("TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL"),
		PollInterval: time.Second,
		Timeout:      180 * time.Second,
	})

	prompts := []string{
		"Flat POD artwork: minimalist black cat silhouette with moon and stars.",
		"Flat POD artwork: retro camping badge with pine forest and sunset.",
		"Flat POD artwork: cute capybara drinking coffee in clean vector style.",
		"Flat POD artwork: botanical wildflowers wreath in boho style.",
		"Flat POD artwork: funny skeleton reading a book in vintage line art.",
	}

	var wg sync.WaitGroup
	for idx, promptText := range prompts {
		idx, promptText := idx, promptText
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			resp, err := client.GenerateImage(context.Background(), &openaiclient.ImageGenerateRequest{
				Model:          client.GetDefaultModel(),
				Prompt:         promptText,
				Size:           "1024x1024",
				ResponseFormat: "b64_json",
				N:              1,
			})
			if err != nil {
				t.Errorf("request %d failed after %s: %v", idx+1, time.Since(start).Round(time.Millisecond), err)
				return
			}
			if resp == nil || len(resp.Data) == 0 {
				t.Errorf("request %d returned no image data after %s", idx+1, time.Since(start).Round(time.Millisecond))
				return
			}
			t.Logf("request %d succeeded after %s", idx+1, time.Since(start).Round(time.Millisecond))
		}()
	}
	wg.Wait()
}
