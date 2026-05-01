package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := flag.String("addr", "http://127.0.0.1:8085", "product-listing-api 地址")
	text := flag.String("text", "", "辅助商品描述文本")
	productURL := flag.String("product-url", "", "商品链接")
	imageURL := flag.String("image-url", "", "商品图片链接")
	country := flag.String("country", "US", "国家")
	language := flag.String("language", "en", "语言")
	poll := flag.Int("poll", 2, "轮询间隔（秒）")
	timeout := flag.Int("timeout", 600, "轮询超时（秒）")
	flag.Parse()

	if *productURL == "" && !(*imageURL != "" && *text != "") {
		fmt.Fprintln(os.Stderr, "需要提供 -product-url，或者同时提供 -image-url 和 -text，否则接口会返回 400")
		os.Exit(1)
	}

	payload := map[string]any{
		"marketplace": "amazon",
		"country":     *country,
		"language":    *language,
		"text":        *text,
		"product_url": *productURL,
		"options": map[string]any{
			"process_images":    true,
			"publish_images":    false,
			"strict_validation": false,
		},
	}
	if *imageURL != "" {
		payload["image_urls"] = []string{*imageURL}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal failed: %v\n", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("%s/api/v1/amazon/listings/generate", *addr)
	fmt.Printf("POST %s\n", url)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		fmt.Fprintf(os.Stderr, "POST request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "unexpected status %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "decode response failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("generate response: %v\n", result)

	taskID, ok := result["task_id"].(string)
	if !ok || taskID == "" {
		fmt.Fprintln(os.Stderr, "task_id is missing")
		os.Exit(1)
	}

	checkURL := fmt.Sprintf("%s/api/v1/amazon/listings/tasks/%s", *addr, taskID)
	fmt.Printf("轮询任务状态: %s\n", checkURL)

	deadline := time.Now().Add(time.Duration(*timeout) * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(checkURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GET status failed: %v\n", err)
			os.Exit(1)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "status API error %d: %s\n", resp.StatusCode, string(body))
			os.Exit(1)
		}

		var task map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "decode task status failed: %v\n", err)
			os.Exit(1)
		}
		resp.Body.Close()

		status, _ := task["status"].(string)
		fmt.Printf("[%ds] status=%s\n", int(time.Since(deadline.Add(-time.Duration(*timeout)*time.Second)).Seconds()), status)

		if status == "completed" || status == "failed" || status == "rejected" || status == "needs_review" {
			taskJSON, _ := json.MarshalIndent(task, "", "  ")
			fmt.Println("任务终态：", status)
			fmt.Println(string(taskJSON))
			os.Exit(0)
		}

		time.Sleep(time.Duration(*poll) * time.Second)
	}

	fmt.Fprintf(os.Stderr, "超时 %d 秒，任务未进入终态\n", *timeout)
	resp, err = http.Get(checkURL)
	if err == nil && resp != nil {
		var final map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&final)
		resp.Body.Close()
		fmt.Println("最新任务状态：", final)
	}
	os.Exit(2)
}
