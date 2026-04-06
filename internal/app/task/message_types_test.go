package task

import "testing"

func TestNewSuccessDataIncludesSourceAndTargetPlatform(t *testing.T) {
	successData := NewSuccessData("shein", "amazon", "B012345678", 177)

	if successData.Platform != "shein" {
		t.Fatalf("expected legacy platform to stay as target platform, got %q", successData.Platform)
	}
	if successData.TargetPlatform != "shein" {
		t.Fatalf("expected target platform to be shein, got %q", successData.TargetPlatform)
	}
	if successData.SourcePlatform != "amazon" {
		t.Fatalf("expected source platform to be amazon, got %q", successData.SourcePlatform)
	}

	data := successData.ToMap()
	if data["platform"] != "shein" {
		t.Fatalf("expected map platform to be shein, got %#v", data["platform"])
	}
	if data["target_platform"] != "shein" {
		t.Fatalf("expected map target_platform to be shein, got %#v", data["target_platform"])
	}
	if data["source_platform"] != "amazon" {
		t.Fatalf("expected map source_platform to be amazon, got %#v", data["source_platform"])
	}
}
