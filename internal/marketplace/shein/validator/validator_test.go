package validator

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	workspace "task-processor/internal/marketplace/shein/workspace"
	contract "task-processor/internal/marketplace/validator"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func request(pkg *sheinpub.Package) contract.Request[*sheinpub.Package] {
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	return contract.Request[*sheinpub.Package]{Target: contract.Target{Marketplace: "shein"}, Action: contract.Publish, RuleVersion: RuleVersion, Snapshot: contract.Snapshot{Revision: "package-1", ExpectedRevision: "package-1", EvaluatedAt: now, ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)}, Input: pkg}
}

func readyPackage() *sheinpub.Package {
	productTypeID, valueID := 901, 9001
	return &sheinpub.Package{
		CategoryID: 3001, ProductTypeID: &productTypeID,
		CategoryResolution:      &sheinpub.CategoryResolution{Status: "resolved", CategoryID: 3001},
		AttributeResolution:     &sheinpub.AttributeResolution{Status: "resolved", ResolvedCount: 1},
		ResolvedAttributes:      []sheinpub.ResolvedAttribute{{Name: "material", AttributeID: 7001}},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{Status: "resolved", PrimaryAttributeID: 1001184},
		DraftPayload:            &sheinpub.RequestDraft{ImageInfo: &sheinpub.ImageDraft{MainImage: "https://example.test/main.jpg"}, SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "SKC-1", SaleAttribute: &sheinpub.ResolvedSaleAttribute{AttributeID: 1001184, AttributeValueID: &valueID}, SKUList: []sheinpub.SKUDraft{{SupplierSKU: "SKU-1", BasePrice: "22.50", SitePriceList: []sheinpub.SitePrice{{SubSite: "us", BasePrice: "22.50"}}}}}}},
		PreviewPayload:          &sheinproduct.Product{},
		FinalSubmissionDraft:    &sheinpub.FinalDraft{Confirmed: true, MainImageURL: "https://example.test/main.jpg", SubmitMode: "publish", FinalImageOrder: []string{"https://example.test/main.jpg"}},
	}
}

func TestValidateOfflineSHEIN(t *testing.T) {
	result, err := (Validator{}).Validate(request(readyPackage()))
	if err != nil || !result.Ready || result.Status != contract.Ready || result.Scope != "shein.offline_package" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	pkg := readyPackage()
	pkg.ReviewNotes = []string{"需要人工复核"}
	result, err = (Validator{}).Validate(request(pkg))
	if err != nil || result.Status != contract.ReadyWithWarnings || len(result.Warnings) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestValidateActionAndBusinessFailures(t *testing.T) {
	for _, action := range []contract.Action{contract.SaveDraft, contract.Publish} {
		t.Run(string(action), func(t *testing.T) {
			pkg := readyPackage()
			pkg.FinalSubmissionDraft.Confirmed = false
			input := request(pkg)
			input.Action = action
			result, err := (Validator{}).Validate(input)
			if err != nil {
				t.Fatal(err)
			}
			if hasRule(result.Blockers, "final_review") != (action == contract.Publish) || result.ReadinessBlockersAllowed != (action == contract.SaveDraft) {
				t.Fatalf("action policy drift: %+v", result)
			}
			input.Input = &sheinpub.Package{}
			result, err = (Validator{}).Validate(input)
			if err != nil || result.Ready || result.Status != contract.Blocked || !hasRule(result.Blockers, "images") {
				t.Fatalf("blockers suppressed: %+v %v", result, err)
			}
		})
	}
}

func TestValidateRequiredOptionalAndPreparedPayload(t *testing.T) {
	for _, required := range []bool{false, true} {
		pkg := readyPackage()
		pkg.AttributeResolution.PendingAttributeCandidates = []sheinpub.PendingAttributeCandidate{{Name: "Material", Required: required, Important: true}}
		result, err := (Validator{}).Validate(request(pkg))
		if err != nil || hasRule(result.Blockers, "attributes") != required {
			t.Fatalf("required=%v result=%+v err=%v", required, result, err)
		}
	}
	pkg := readyPackage()
	pkg.DraftPayload.SKCList = nil
	pkg.PreviewPayload.SKCList = []sheinproduct.SKC{{}}
	result, err := (Validator{}).Validate(request(pkg))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, check := range result.Blockers {
		if check.Rule == "variants" {
			count++
			if check.Code != "sku_invalid" || check.Category != "sku" || len(check.Paths) == 0 || check.Guidance == "" {
				t.Fatalf("lost semantics: %+v", check)
			}
		}
	}
	if count != 2 {
		t.Fatalf("variants checks=%d, want 2: %+v", count, result)
	}
}

func TestValidateRejectsInvalidRequestWithZeroResult(t *testing.T) {
	cases := []struct {
		name   string
		change func(*contract.Request[*sheinpub.Package])
		code   contract.ErrorCode
	}{
		{"nil", func(r *contract.Request[*sheinpub.Package]) { r.Input = nil }, contract.InvalidInput},
		{"target", func(r *contract.Request[*sheinpub.Package]) { r.Target.Marketplace = "temu" }, contract.UnsupportedTarget},
		{"site", func(r *contract.Request[*sheinpub.Package]) { r.Target.Site = "us" }, contract.UnsupportedTarget},
		{"preview", func(r *contract.Request[*sheinpub.Package]) { r.Action = contract.Preview }, contract.UnsupportedAction},
		{"empty action", func(r *contract.Request[*sheinpub.Package]) { r.Action = "" }, contract.UnsupportedAction},
		{"version", func(r *contract.Request[*sheinpub.Package]) { r.RuleVersion = "future" }, contract.UnsupportedVersion},
		{"stale", func(r *contract.Request[*sheinpub.Package]) { r.Snapshot.ExpectedRevision = "package-2" }, contract.StaleInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := request(readyPackage())
			tc.change(&r)
			result, err := (Validator{}).Validate(r)
			var typed *contract.Error
			if !errors.As(err, &typed) || typed.Code != tc.code || !reflect.DeepEqual(result, contract.Result{}) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestValidateDeterministicConcurrentAndDoesNotNormalizeCaller(t *testing.T) {
	pkg := readyPackage()
	r := request(pkg)
	baseline, err := (Validator{}).Validate(r)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Go(func() {
			result, err := (Validator{}).Validate(r)
			if err != nil || !reflect.DeepEqual(result, baseline) {
				t.Errorf("unstable result=%+v err=%v", result, err)
			}
		})
	}
	group.Wait()
	if pkg.RequestDraft != nil || pkg.PreviewProduct != nil || pkg.FinalDraft != nil {
		t.Fatal("validator normalized caller aliases")
	}
	if !reflect.DeepEqual(pkg, readyPackage()) {
		t.Fatal("validator mutated caller facts")
	}
}

func TestValidateReusesWorkspacePayloadRules(t *testing.T) {
	pkg := readyPackage()
	pkg.DraftPayload.ImageInfo = nil
	pkg.FinalSubmissionDraft = nil
	result, err := (Validator{}).Validate(request(pkg))
	if err != nil {
		t.Fatal(err)
	}
	cloned, err := sheinpub.ClonePackageForPersistence(pkg)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range workspace.BuildSubmitPayloadReadinessChecks(cloned, "publish") {
		found := false
		for _, actual := range result.Checks {
			if actual.Rule == expected.Key && actual.Message == expected.Message {
				found = true
				if (actual.Status == contract.CheckReady) != expected.OK || actual.Guidance != expected.SuggestedAction || actual.Code != expected.Taxonomy.BlockerKey {
					t.Fatalf("rule drift %s: %+v", expected.Key, actual)
				}
			}
		}
		if !found {
			t.Fatalf("missing shared check %s", expected.Key)
		}
	}
}

func hasRule(checks []contract.Check, rule string) bool {
	for _, check := range checks {
		if check.Rule == rule {
			return true
		}
	}
	return false
}

type forbiddenRandomReader struct{}

func (forbiddenRandomReader) Read([]byte) (int, error) {
	return 0, errors.New("validator must not read random entropy")
}

// UUID's reader is process-global. This package deliberately has no parallel
// tests; the hook is restored before any other test can run.
func TestValidateDoesNotReadRandomEntropy(t *testing.T) {
	uuid.SetRand(forbiddenRandomReader{})
	defer uuid.SetRand(nil)
	result, err := (Validator{}).Validate(request(readyPackage()))
	if err != nil || !result.Ready {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestValidateCopyFailureReturnsTypedErrorAndZeroResult(t *testing.T) {
	pkg := readyPackage()
	pkg.PreviewPayload.SKCList = []sheinproduct.SKC{{SKUS: []sheinproduct.SKU{{Weight: math.NaN()}}}}
	result, err := (Validator{}).Validate(request(pkg))
	var typed *contract.Error
	if !errors.As(err, &typed) || typed.Code != contract.EvaluationFailed || !reflect.DeepEqual(result, contract.Result{}) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestValidateDoesNotMutateNestedPayloadDuringPreparation(t *testing.T) {
	fixture := func() *sheinpub.Package {
		pkg := readyPackage()
		stock := 7
		pkg.PreviewPayload.SKCList = []sheinproduct.SKC{{SKUS: []sheinproduct.SKU{{StockCount: &stock, SupplierSKU: "sku"}}}}
		return pkg
	}
	pkg := fixture()
	_, err := (Validator{}).Validate(request(pkg))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pkg, fixture()) {
		t.Fatal("submission preparation mutated nested input")
	}
}
