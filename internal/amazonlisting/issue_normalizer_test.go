package amazonlisting

import (
	"testing"

	amazonapi "task-processor/internal/amazon/api"
)

func TestNormalizeListingIssues(t *testing.T) {
	resp := &amazonapi.ListingResponse{
		SKU:    "SKU-1",
		Status: "INVALID",
		Issues: []struct {
			Code     string `json:"code"`
			Message  string `json:"message"`
			Severity string `json:"severity"`
		}{
			{Code: "9001", Message: "Missing required attribute brand", Severity: "ERROR"},
			{Code: "9002", Message: "Item title too long", Severity: "ERROR"},
		},
	}

	issues := normalizeListingIssues(resp)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Type != "missing_brand" {
		t.Fatalf("unexpected first issue type: %s", issues[0].Type)
	}
	if issues[1].Type != "title_too_long" {
		t.Fatalf("unexpected second issue type: %s", issues[1].Type)
	}
	if issues[0].OperatorAdvice == "" || issues[0].OperatorAction == "" {
		t.Fatalf("expected operator advice and action")
	}
	if issues[0].OperatorAction != OperatorActionFillBrand {
		t.Fatalf("expected normalized operator action enum, got %s", issues[0].OperatorAction)
	}
}

func TestSummarizeAmazonIssuesIncludesManualAdvices(t *testing.T) {
	summary := summarizeAmazonIssues([]AmazonIssue{
		{
			Type:           "unknown",
			Message:        "Restricted product compliance approval required",
			IsBlocking:     true,
			Retryable:      false,
			OperatorAdvice: "该商品可能涉及限制类目或合规审批，需要人工确认资质、证书或审核要求。",
			OperatorAction: OperatorActionCheckCompliance,
		},
	})

	if summary.ManualCount != 1 {
		t.Fatalf("expected one manual issue")
	}
	if len(summary.ManualAdvices) != 1 {
		t.Fatalf("expected one manual advice")
	}
	if summary.ManualAdvices[0] == "" {
		t.Fatalf("expected advice text")
	}
	if len(summary.ManualActions) != 1 || summary.ManualActions[0] != OperatorActionCheckCompliance {
		t.Fatalf("expected manual action enum")
	}
	if summary.ActionCounts[OperatorActionCheckCompliance] != 1 {
		t.Fatalf("expected action count for compliance")
	}
}
