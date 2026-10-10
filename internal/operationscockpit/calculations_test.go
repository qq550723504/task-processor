package operationscockpit

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const testStore = "a26e6326-ec15-4b47-8bcc-e78e3b7c7860"
const testGoal = "beaa2f90-019b-410e-b684-7384c7021219"

func testRecord(id, start, end string, revenue, refund, cost int64) Record {
	return Record{ID: id, StoreID: testStore, Revision: 1, Period: Period{start, end}, Amounts: Amounts{Revenue: revenue, Refunds: refund, Procurement: cost}}
}

func TestRefundsAndCostsMayProduceLoss(t *testing.T) {
	totals, err := (Amounts{Revenue: 100, Refunds: 200, Procurement: 30, Advertising: 20}).Totals()
	if err != nil || totals.NetRevenue != -100 || totals.NetProfit != -150 || totals.Margin != nil {
		t.Fatalf("refunds/loss must be preserved and margin undefined: %+v, %v", totals, err)
	}
	if _, err := (Amounts{Revenue: MaxFieldAmount + 1}).Totals(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded input accepted: %v", err)
	}
}

func TestFinancialAggregationRejectsUnsafeIntegers(t *testing.T) {
	_, err := (Totals{Revenue: MaxSafeInteger}).Add(Totals{Revenue: 1})
	if !errors.Is(err, ErrAmountRange) {
		t.Fatalf("unsafe aggregate must fail, got %v", err)
	}
	_, err = (Amounts{Revenue: -1}).Totals()
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative input accepted: %v", err)
	}
}

func TestCoverageExcludesCrossBoundaryInsteadOfProrating(t *testing.T) {
	query := Period{"2026-09-01", "2026-09-07"}
	month := testRecord("7b6e9c5b-e08f-4f15-bd01-88fe8f1e7550", "2026-09-01", "2026-09-30", 30000, 0, 1000)
	aggregate, err := Aggregate(query, []Record{month})
	if err != nil || aggregate.Complete || aggregate.Totals.NetProfit != 0 || len(aggregate.Excluded) != 1 || len(aggregate.Gaps) != 1 || aggregate.Gaps[0] != query {
		t.Fatalf("monthly fact must not become weekly profit: %+v, %v", aggregate, err)
	}
}

func TestCoverageRequiresEveryDayAndExplicitZeroIsComplete(t *testing.T) {
	query := Period{"2026-09-01", "2026-09-03"}
	records := []Record{
		testRecord("efb7e0c6-66f8-4159-a9a0-d3a1ace0b415", "2026-09-01", "2026-09-01", 0, 0, 0),
		testRecord("1cbf5ae0-eeff-4021-b784-91cad8c7888d", "2026-09-03", "2026-09-03", 100, 0, 30),
	}
	aggregate, err := Aggregate(query, records)
	if err != nil || aggregate.Complete || len(aggregate.Gaps) != 1 || aggregate.Gaps[0] != (Period{"2026-09-02", "2026-09-02"}) || aggregate.Totals.NetProfit != 70 {
		t.Fatalf("partial facts must retain gap and partial amount: %+v, %v", aggregate, err)
	}
	records = append(records, testRecord("a78f0a11-15b2-4f7c-aa4a-0659c55e5751", "2026-09-02", "2026-09-02", 0, 0, 0))
	aggregate, err = Aggregate(query, records)
	if err != nil || !aggregate.Complete || len(aggregate.Gaps) != 0 {
		t.Fatalf("explicit zero fact must cover its day: %+v, %v", aggregate, err)
	}
	records = append(records, testRecord("84a36fd8-6a0f-41f9-9ca1-d04c9c75384b", "2026-09-01", "2026-09-02", 100, 0, 0))
	if _, err := Aggregate(query, records); !errors.Is(err, ErrOverlap) {
		t.Fatalf("overlapping current facts accepted: %v", err)
	}
}

func TestGoalThresholdsUseExactNaturalDayRatio(t *testing.T) {
	now := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC) // UTC+8: Sep 4; completed through Sep 3.
	goal := GoalConfig{StoreIDs: []string{testStore}, Frequency: "week", Period: Period{"2026-08-31", "2026-09-06"}, Profit: 700, NormalBPS: 9000, AttentionBPS: 7000}
	period := Period{"2026-08-31", "2026-09-03"}
	for _, tc := range []struct {
		profit int64
		state  string
	}{{360, "normal"}, {359, "attention"}, {280, "attention"}, {279, "abnormal"}, {-1, "abnormal"}} {
		agg, err := Aggregate(period, []Record{testRecord("ac414aef-8297-477a-8e2e-bf32710c1617", period.Start, period.End, 1000, 0, 1000-tc.profit)})
		if err != nil {
			t.Fatal(err)
		}
		result, err := EvaluateGoal(goal, now, map[string]StoreAggregate{testStore: agg})
		if err != nil || result.State != tc.state || result.ExpectedProfit == nil || result.ExpectedProfit.Numerator != "2800" || result.ExpectedProfit.Denominator != "7" {
			t.Fatalf("profit %d: %+v %v", tc.profit, result, err)
		}
	}
	result, err := EvaluateGoal(goal, now, map[string]StoreAggregate{})
	if err != nil || result.State != "pending_data" || result.Completion != nil {
		t.Fatalf("missing store cannot count as zero or healthy: %+v %v", result, err)
	}
}

func TestTodayFutureAndDateLimits(t *testing.T) {
	now := time.Date(2026, 10, 9, 17, 0, 0, 0, time.UTC) // UTC+8: Oct 10.
	if Today(now) != "2026-10-10" {
		t.Fatal("wrong operating timezone")
	}
	for _, tc := range []struct{ date, state string }{{"2026-10-10", "pending_data"}, {"2026-10-11", "not_started"}} {
		goal := GoalConfig{StoreIDs: []string{testStore}, Frequency: "day", Period: Period{tc.date, tc.date}, Profit: 100, NormalBPS: 9000, AttentionBPS: 7000}
		result, err := EvaluateGoal(goal, now, nil)
		if err != nil || result.State != tc.state {
			t.Fatalf("%s: %+v %v", tc.date, result, err)
		}
	}
	if (Period{"2026-02-29", "2026-03-01"}).Valid() || (Period{"2025-01-01", "2026-01-02"}).Valid() {
		t.Fatal("invalid/overlong dates accepted")
	}
	fact := FactInput{Period: Period{"2026-10-10", "2026-10-10"}}
	if fact.Valid(now) {
		t.Fatal("unfinished day accepted as fact")
	}
}

func TestGoalCreationAndMaintenanceAuthority(t *testing.T) {
	head := GoalHead{OrganizationID: "org", ID: testGoal, CreatorID: "creator", Revision: 3}
	for _, tc := range []struct {
		actor, org string
		access     Access
		allowed    bool
	}{
		{"creator", "org", Access{GoalsRead: true}, true},
		{"manager", "org", Access{GoalsRead: true, GoalsManage: true}, true},
		{"other", "org", Access{GoalsRead: true, GoalsCreate: true}, false},
		{"creator", "org", Access{}, false},
		{"creator", "other-org", Access{GoalsRead: true, GoalsManage: true}, false},
	} {
		scope := Scope{OrganizationID: tc.org, ActorID: tc.actor}
		metadata, err := MaintenanceMetadata(head, scope, tc.access, false)
		if (err == nil) != tc.allowed {
			t.Fatalf("actor %s access %+v: %v", tc.actor, tc.access, err)
		}
		if tc.allowed {
			body, _ := json.Marshal(metadata)
			if strings.Contains(string(body), "creator") || strings.Contains(string(body), "store") || metadata.Revision != "3" || metadata.ScopeValid {
				t.Fatalf("unsafe metadata %s", body)
			}
		}
	}
	if err := AuthorizeGoalCommand(&head, Scope{"org", "other"}, Access{GoalsRead: true, GoalsCreate: true}, "create", 0); !errors.Is(err, ErrRevision) {
		t.Fatalf("create overwrites existing target: %v", err)
	}
	if err := AuthorizeGoalCommand(&head, Scope{"org", "creator"}, Access{GoalsRead: true}, "update", 2); !errors.Is(err, ErrRevision) {
		t.Fatalf("stale maintenance accepted: %v", err)
	}
}

func TestGrowthRequiresComparableCompletePeriods(t *testing.T) {
	current := StoreAggregate{StoreID: testStore, Period: Period{"2026-09-08", "2026-09-14"}, Complete: true, Totals: Totals{NetProfit: 150}}
	previous := StoreAggregate{StoreID: testStore, Period: Period{"2026-09-01", "2026-09-07"}, Complete: true, Totals: Totals{NetProfit: 100}}
	if ratio := Growth(current, previous); ratio == nil || ratio.Numerator != "50" || ratio.Denominator != "100" {
		t.Fatalf("wrong growth: %+v", ratio)
	}
	previous.Period = Period{"2026-08-01", "2026-08-07"}
	if Growth(current, previous) != nil {
		t.Fatal("non-adjacent periods accepted")
	}
	previous.Period = Period{"2026-09-01", "2026-09-07"}
	previous.Totals.NetProfit = 0
	if Growth(current, previous) != nil {
		t.Fatal("zero baseline invented growth")
	}
}
