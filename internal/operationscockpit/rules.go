package operationscockpit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

// Rule is a current, evidence-based projection. It has no acknowledged,
// processing or recovered lifecycle and never creates an AI or external task.
type Rule struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Level         string         `json:"level"`
	Title         string         `json:"title"`
	Reason        string         `json:"reason"`
	Advice        string         `json:"advice"`
	ActionPath    string         `json:"actionPath"`
	ActionLabel   string         `json:"actionLabel"`
	Source        string         `json:"source"`
	CapturedAt    time.Time      `json:"capturedAt"`
	StoreID       string         `json:"storeId,omitempty"`
	Period        Period         `json:"period"`
	Profit        *int64         `json:"profit,omitempty"`
	GoalRevision  string         `json:"goalRevision,omitempty"`
	RecordCount   int            `json:"recordCount,omitempty"`
	GapCount      int            `json:"gapCount,omitempty"`
	ExcludedCount int            `json:"excludedCount,omitempty"`
	RecordDigest  string         `json:"recordDigest,omitempty"`
	GoalBasis     *GoalRuleBasis `json:"goalBasis,omitempty"`
}

type GoalRuleBasis struct {
	Target           int64          `json:"target"`
	NormalBPS        int64          `json:"normalBps"`
	AttentionBPS     int64          `json:"attentionBps"`
	MinimumMarginBPS *int64         `json:"minimumMarginBps"`
	Evaluation       GoalEvaluation `json:"evaluation"`
}

func Rules(snapshot Snapshot) ([]Rule, error) {
	result := []Rule{}
	if !snapshot.Period.Valid() {
		return result, ErrInvalid
	}
	for id, aggregate := range snapshot.Stores {
		if !UUID(id) || aggregate.StoreID != id || aggregate.Period != snapshot.Period {
			return nil, ErrInvalid
		}
		rule := Rule{Source: "manual", CapturedAt: snapshot.CapturedAt, StoreID: id, Period: snapshot.Period, ActionPath: "/workbench/overview/stores?storeId=" + id + "&startDate=" + snapshot.Period.Start + "&endDate=" + snapshot.Period.End, ActionLabel: "查看经营明细", RecordCount: len(aggregate.Records)}
		if !aggregate.Complete {
			rule.Kind = "data_missing"
			rule.Level = "data"
			rule.Title = "经营数据不完整"
			rule.Reason = "所选周期存在未覆盖日期或跨越边界的记录，尚不能评价完整利润。"
			rule.Advice = "查看区间缺口，补录已完成期间，或选择与现有录入记录一致的周期。"
			rule.GapCount = len(aggregate.Gaps)
			rule.ExcludedCount = len(aggregate.Excluded)
		} else if aggregate.Totals.NetProfit < 0 {
			rule.Kind = "period_loss"
			rule.Level = "attention"
			rule.Title = "本期录入净利润为负"
			rule.Reason = "完整覆盖所选周期的人工录入收入、退款与成本计算得到亏损。"
			rule.Advice = "查看经营明细，核对本期收入、退款和各项成本，再由人处理经营问题。"
			profit := aggregate.Totals.NetProfit
			rule.Profit = &profit
		} else {
			continue
		}
		rule.ID = rule.Kind + ":" + id + ":" + snapshot.Period.Start + ":" + snapshot.Period.End
		evidence, err := json.Marshal(struct {
			Records, Excluded []RecordEvidence
			Gaps              []Period
		}{aggregate.Records, aggregate.Excluded, aggregate.Gaps})
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(evidence)
		rule.RecordDigest = hex.EncodeToString(digest[:])
		result = append(result, rule)
	}
	if snapshot.Goal != nil && snapshot.Evaluation != nil && (snapshot.Evaluation.State == "attention" || snapshot.Evaluation.State == "abnormal") {
		goal := snapshot.Goal
		evaluation := snapshot.Evaluation
		if !UUID(goal.ID) || goal.Revision <= 0 || !goal.Config.Valid() {
			return nil, ErrInvalid
		}
		level := "attention"
		if evaluation.State == "abnormal" {
			level = "critical"
		}
		profit := evaluation.Totals.NetProfit
		basis := &GoalRuleBasis{Target: goal.Config.Profit, NormalBPS: goal.Config.NormalBPS, AttentionBPS: goal.Config.AttentionBPS, MinimumMarginBPS: goal.Config.MinimumMarginBPS, Evaluation: *evaluation}
		result = append(result, Rule{ID: "goal_assessment:" + goal.ID + ":" + strconv.FormatInt(goal.Revision, 10) + ":" + evaluation.Through, Kind: "goal_assessment", Level: level, Title: "目标评判需关注", Reason: "完整经营数据下，利润进度或已启用的利润率底线未达到目标评判阈值。", Advice: "查看目标评判依据及收入成本明细，由创建人或具备管理权限的人维护目标。", ActionPath: "/workbench/overview/goals", ActionLabel: "查看目标依据", Source: "manual", CapturedAt: snapshot.CapturedAt, Period: Period{Start: goal.Config.Period.Start, End: evaluation.Through}, Profit: &profit, GoalRevision: strconv.FormatInt(goal.Revision, 10), GoalBasis: basis})
	}
	rank := map[string]int{"critical": 0, "attention": 1, "data": 2}
	sort.Slice(result, func(i, j int) bool {
		if rank[result[i].Level] != rank[result[j].Level] {
			return rank[result[i].Level] < rank[result[j].Level]
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}
