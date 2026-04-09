package browser

import (
	"testing"

	sharedbrowser "task-processor/internal/crawler/shared/browser"
)

func TestValidateInstanceRequiresActiveContext(t *testing.T) {
	im := &InstanceManager{}
	instance := &BrowserInstance{
		ID: 1,
		Manager: &BrowserManager{
			Manager: sharedbrowser.NewManager(&sharedbrowser.BrowserConfig{}),
		},
	}

	if im.ValidateInstance(instance) {
		t.Fatal("期望没有活动 context 的实例校验失败")
	}
}

func TestGetInstanceInfoReportsContextInsteadOfPage(t *testing.T) {
	im := &InstanceManager{}
	info := im.GetInstanceInfo(&BrowserInstance{ID: 7})

	if info["manager_exists"] != false {
		t.Fatalf("manager_exists 应为 false，实际: %v", info["manager_exists"])
	}
	if info["context_exists"] != false {
		t.Fatalf("context_exists 应为 false，实际: %v", info["context_exists"])
	}
	if _, ok := info["page_exists"]; ok {
		t.Fatal("实例信息不应再暴露 page_exists")
	}
}

func TestGetInstanceInfoNilInstance(t *testing.T) {
	im := &InstanceManager{}
	info := im.GetInstanceInfo(nil)

	if info["valid"] != false {
		t.Fatalf("nil 实例的 valid 应为 false，实际: %v", info["valid"])
	}
	if info["error"] != "instance is nil" {
		t.Fatalf("nil 实例的 error 不符合预期，实际: %v", info["error"])
	}
}
