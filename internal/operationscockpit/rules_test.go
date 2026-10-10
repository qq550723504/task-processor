package operationscockpit

import (
	"testing"
	"time"
)

func TestRuleAdviceUsesLossAndMissingEvidenceWithoutInventingProfit(t *testing.T) {
	period := Period{"2026-10-01", "2026-10-02"}
	loss, err := Aggregate(period, []Record{testRecord("a7a6958d-7e74-4b03-8a69-4f98c97ca470", period.Start, period.End, 100, 0, 200)})
	if err != nil {
		t.Fatal(err)
	}
	missingID := "cd825a4d-bf56-4900-b5ad-7b244a36be51"
	missing, err := Aggregate(period, nil)
	if err != nil {
		t.Fatal(err)
	}
	missing.StoreID = missingID
	snapshot := Snapshot{CapturedAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), Period: period, Stores: map[string]StoreAggregate{testStore: loss, missingID: missing}}
	rules, err := Rules(snapshot)
	if err != nil || len(rules) != 2 {
		t.Fatalf("rules: %+v %v", rules, err)
	}
	if rules[0].Kind != "period_loss" || rules[0].Profit == nil || *rules[0].Profit != -100 || rules[0].RecordCount != 1 || rules[0].Source != "manual" {
		t.Fatalf("loss evidence: %+v", rules[0])
	}
	if rules[1].Kind != "data_missing" || rules[1].Profit != nil || rules[1].Level != "data" {
		t.Fatalf("missing facts must not imply profitability: %+v", rules[1])
	}
	if rules[0].ActionPath != "/workbench/overview/stores?storeId="+testStore {
		t.Fatal("unsafe action path")
	}
	for _, rule := range rules {
		if rule.ActionLabel == "AI生成方案" || rule.ActionLabel == "创建任务" {
			t.Fatal("unopened AI action")
		}
	}
}

func TestPendingGoalDoesNotGenerateProfitWarning(t *testing.T) {
	period := Period{"2026-10-01", "2026-10-02"}
	goal := GoalVersion{ID: testGoal, Revision: 1, Config: GoalConfig{StoreIDs: []string{testStore}, Frequency: "month", Period: Period{"2026-10-01", "2026-10-31"}, Profit: 1000, NormalBPS: 9000, AttentionBPS: 7000}}
	snapshot := Snapshot{Period: period, Stores: map[string]StoreAggregate{}, Goal: &goal, Evaluation: &GoalEvaluation{State: "pending_data"}}
	rules, err := Rules(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatalf("pending data cannot generate target failure: %+v", rules)
	}
	snapshot.Evaluation = &GoalEvaluation{State: "abnormal", Through: "2026-10-02", Totals: Totals{NetProfit: 50}}
	rules, err = Rules(snapshot)
	if err != nil || len(rules) != 1 || rules[0].Kind != "goal_assessment" || rules[0].GoalRevision != "1" {
		t.Fatalf("goal evidence: %+v %v", rules, err)
	}
}
