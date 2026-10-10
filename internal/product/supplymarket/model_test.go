package supplymarket

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/product/catalog"
)

func TestManualEvaluationRequiresOfflineCooperationBeforeApproval(t *testing.T) {
	stage, err := NextStage("selected", Submitted, Evaluation{Action: "evaluate", Note: "正在人工评估"})
	if err != nil || stage != Evaluating {
		t.Fatalf("cannot start manual review: %v %s", err, stage)
	}
	if _, err = NextStage("selected", Evaluating, Evaluation{Action: "approve", Note: "优化及资质已核查"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("approved without cooperation: %v", err)
	}
	stage, err = NextStage("selected", Evaluating, Evaluation{Action: "request_supplement", Note: "请补充生产能力"})
	if err != nil || stage != SupplementRequired {
		t.Fatalf("bad supplement request: %v", err)
	}
	if _, err = NextStage("selected", stage, Evaluation{Action: "approve", Note: "ok", CooperationConfirmed: true}); !errors.Is(err, ErrConflict) {
		t.Fatalf("approved while waiting supplement: %v", err)
	}
	stage, err = NextStage("selected", stage, Evaluation{Action: "supplement", Note: "已补充"})
	if err != nil || stage != Evaluating {
		t.Fatalf("bad supplement: %v", err)
	}
	stage, err = NextStage("selected", stage, Evaluation{Action: "approve", Note: "线下合作确认", CooperationConfirmed: true})
	if err != nil || stage != Approved {
		t.Fatalf("cannot approve: %v", err)
	}
	if _, err = NextStage("selected", stage, Evaluation{Action: "evaluate", Note: "覆盖历史"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("terminal application overwritten: %v", err)
	}
}

func TestConnectionConfirmationDoesNotPublishProduct(t *testing.T) {
	if _, err := NextStage("connection", Evaluating, Evaluation{Action: "approve", Note: "ok", CooperationConfirmed: true}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("connection became market release: %v", err)
	}
	stage, err := NextStage("connection", Evaluating, Evaluation{Action: "confirm_plan", Note: "线下对接方案已确认"})
	if err != nil || stage != PlanConfirmed {
		t.Fatalf("cannot confirm plan: %v", err)
	}
}

func TestPublicProjectionCannotDiscloseSourceTracesOrPrivateURL(t *testing.T) {
	snapshot := catalog.ProductSnapshot{Title: "own item", Description: "public description", Sources: []catalog.SourceRecord{{URL: "https://private.example/token", Notes: []string{"PRIVATE_EVIDENCE"}}}, Attributes: []catalog.Attribute{{Name: "material", Value: "cotton", Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Detail: "PRIVATE_TRACE"}}}}}, Images: []catalog.Image{{URL: "https://cdn.example/art.png", Trace: catalog.Trace{Sources: []catalog.SourceRecord{{Notes: []string{"PRIVATE_IMAGE_TRACE"}}}}}}, Variants: []catalog.Variant{{SourceID: "sku-1", Title: "blue", SKU: "SKU-1", Price: &catalog.Price{Currency: "CNY", Amount: 7, CostPrice: 123}, Stock: 10}}}
	projection, err := PublicProjection(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(projection)
	if strings.Contains(string(encoded), "PRIVATE") || strings.Contains(string(encoded), "private.example") || strings.Contains(string(encoded), "cost_price") || strings.Contains(string(encoded), "123") {
		t.Fatalf("private canonical fact leaked: %s", encoded)
	}
	snapshot.Images[0].URL = "https://cdn.example/art.png?X-Amz-Signature=private-token"
	if _, err := PublicProjection(snapshot); !errors.Is(err, ErrInvalid) {
		t.Fatalf("signed private source image published: %v", err)
	}
}

func TestSupplyDeclarationMustStateStockOrCapacity(t *testing.T) {
	if (SupplyDeclaration{MinimumQuantity: 1, LeadDays: 1, Province: "广东", City: "深圳"}).Validate() == nil {
		t.Fatal("missing actual supply declaration accepted")
	}
	if err := (SupplyDeclaration{Capacity: "每周生产100件", MinimumQuantity: 1, LeadDays: 7, Province: "广东", City: "深圳"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
