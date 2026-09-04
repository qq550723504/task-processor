package listingkit

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestListingKitResultSemanticFieldNamesRemainUsable(t *testing.T) {
	result := &ListingKitResult{
		SDSDesignResult: &SDSSyncSummary{
			Status: "completed",
		},
	}
	snapshot := &StandardProductSnapshot{
		SDSDesignResult: result.SDSDesignResult,
	}

	if result.SDSDesignResult == nil || result.SDSDesignResult.Status != "completed" {
		t.Fatalf("result sds design result = %+v", result.SDSDesignResult)
	}
	if snapshot.SDSDesignResult == nil || snapshot.SDSDesignResult != result.SDSDesignResult {
		t.Fatalf("snapshot sds design result = %+v", snapshot.SDSDesignResult)
	}
}

func TestListingKitResultJSONIncludesLegacyAndSemanticSDSFields(t *testing.T) {
	result := &ListingKitResult{
		SDSSync:         &SDSSyncSummary{Status: "completed"},
		SDSDesignResult: &SDSSyncSummary{Status: "completed"},
		StandardProductSnapshot: &StandardProductSnapshot{
			SDSSync:         &SDSSyncSummary{Status: "completed"},
			SDSDesignResult: &SDSSyncSummary{Status: "completed"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(data)
	for _, key := range []string{
		`"sds_sync"`,
		`"sds_design_result"`,
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("json output missing %s: %s", key, text)
		}
	}
}

func TestListingKitResultMarshalJSONDoesNotMutateReceiverOrSharedSemanticGraph(t *testing.T) {
	resultLegacy := &SDSSyncSummary{Status: "legacy-result"}
	resultSemantic := &SDSSyncSummary{Status: "semantic-result"}
	snapshotLegacy := &SDSSyncSummary{Status: "legacy-snapshot"}
	resultPod := semanticSerializationPodFixture(" result ")
	snapshotPod := semanticSerializationPodFixture(" snapshot ")
	resultPodBefore := *resultPod
	resultPodBefore.History = append([]PodExecutionAuditEvent(nil), resultPod.History...)
	snapshotPodBefore := *snapshotPod
	snapshotPodBefore.History = append([]PodExecutionAuditEvent(nil), snapshotPod.History...)
	sheinDraft := &sheinpub.RequestDraft{SpuName: "legacy-only draft"}
	sheinPackage := &sheinpub.Package{RequestDraft: sheinDraft}
	snapshot := &StandardProductSnapshot{
		SDSSync:      snapshotLegacy,
		PodExecution: snapshotPod,
	}
	result := &ListingKitResult{
		SDSSync:                 resultLegacy,
		SDSDesignResult:         resultSemantic,
		PodExecution:            resultPod,
		StandardProductSnapshot: snapshot,
		Shein:                   sheinPackage,
	}

	if _, err := json.Marshal(result); err != nil {
		t.Fatal(err)
	}

	if result.SDSSync != resultLegacy || result.SDSDesignResult != resultSemantic {
		t.Fatal("marshal rewrote distinct result SDS semantic aliases")
	}
	if result.PodExecution != resultPod || !reflect.DeepEqual(*resultPod, resultPodBefore) {
		t.Fatalf("marshal mutated result PodExecution: got %#v, want %#v", resultPod, resultPodBefore)
	}
	if result.StandardProductSnapshot != snapshot || snapshot.SDSSync != snapshotLegacy || snapshot.SDSDesignResult != nil {
		t.Fatal("marshal rewrote snapshot SDS semantic aliases or snapshot pointer")
	}
	if snapshot.PodExecution != snapshotPod || !reflect.DeepEqual(*snapshotPod, snapshotPodBefore) {
		t.Fatalf("marshal mutated snapshot PodExecution: got %#v, want %#v", snapshotPod, snapshotPodBefore)
	}
	if result.Shein != sheinPackage || sheinPackage.RequestDraft != sheinDraft || sheinPackage.DraftPayload != nil {
		t.Fatal("marshal mutated nested SHEIN semantic aliases or package pointer")
	}
}

func TestStandardProductSnapshotMarshalJSONDoesNotMutateEitherAliasDirectionOrPodGraph(t *testing.T) {
	tests := []struct {
		name     string
		legacy   *SDSSyncSummary
		semantic *SDSSyncSummary
	}{
		{name: "legacy only", legacy: &SDSSyncSummary{Status: "legacy"}},
		{name: "semantic only", semantic: &SDSSyncSummary{Status: "semantic"}},
		{name: "distinct aliases", legacy: &SDSSyncSummary{Status: "legacy"}, semantic: &SDSSyncSummary{Status: "semantic"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := semanticSerializationPodFixture(" snapshot ")
			podBefore := *pod
			podBefore.History = append([]PodExecutionAuditEvent(nil), pod.History...)
			snapshot := &StandardProductSnapshot{SDSSync: tt.legacy, SDSDesignResult: tt.semantic, PodExecution: pod}

			if _, err := json.Marshal(snapshot); err != nil {
				t.Fatal(err)
			}

			if snapshot.SDSSync != tt.legacy || snapshot.SDSDesignResult != tt.semantic {
				t.Fatal("marshal rewrote snapshot SDS semantic aliases")
			}
			if snapshot.PodExecution != pod || !reflect.DeepEqual(*pod, podBefore) {
				t.Fatalf("marshal mutated snapshot PodExecution: got %#v, want %#v", pod, podBefore)
			}
		})
	}
}

func TestSemanticSerializationReferenceFieldSchemaIsExplicit(t *testing.T) {
	assertSemanticSerializationReferenceFields(t, reflect.TypeOf(ListingKitResult{}), []string{
		"ReviewReasons", "Platforms", "PodExecution", "StandardProductSnapshot", "CatalogProduct",
		"ApprovedAssetInventory", "ApprovedAssetInventories", "CanonicalProduct", "SDSSync", "SDSDesignResult", "Amazon", "Shein",
		"SheinStoreResolution", "Temu", "Walmart", "Summary", "Revision", "RevisionHistory", "ChildTasks",
		"WorkflowStages", "WorkflowIssues",
	})
	assertSemanticSerializationReferenceFields(t, reflect.TypeOf(StandardProductSnapshot{}), []string{
		"CatalogProduct", "ApprovedAssetInventory", "ApprovedAssetInventories", "PodExecution", "SDSSync", "SDSDesignResult", "Summary",
		"ChildTasks", "WorkflowStages", "WorkflowIssues",
	})
}

func assertSemanticSerializationReferenceFields(t *testing.T, typ reflect.Type, want []string) {
	t.Helper()
	var got []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		switch field.Type.Kind() {
		case reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			got = append(got, field.Name)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s reference fields changed: got %v, want %v; review MarshalJSON purity and update the wire clone or this explicit read-only classification", typ.Name(), got, want)
	}
}

func semanticSerializationPodFixture(detail string) *PodExecutionSummary {
	return &PodExecutionSummary{
		Provider:       " SDS ",
		DependencyMode: " OPTIONAL ",
		Status:         " SUCCEEDED ",
		DecisionSource: " fixture ",
		History: []PodExecutionAuditEvent{{
			Kind: " STATUS_TRANSITION ", Code: " code ", Detail: detail, Provider: " SDS ", ToStatus: " SUCCEEDED ",
		}},
	}
}
