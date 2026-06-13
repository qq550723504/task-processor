package model

import (
	"encoding/json"
	"testing"
)

func TestTaskUnmarshalZipcode(t *testing.T) {
	var task Task
	err := json.Unmarshal([]byte(`{"productId":"B001","zipcode":"10001"}`), &task)
	if err != nil {
		t.Fatalf("Unmarshal task: %v", err)
	}
	if task.Zipcode != "10001" {
		t.Fatalf("Zipcode = %q, want 10001", task.Zipcode)
	}
}
