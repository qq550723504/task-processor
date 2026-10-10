package operationscockpit

import (
	"math/big"
	"sort"
	"time"
)

// Rational keeps derived ratios exact even when their products exceed int64 or
// JavaScript's safe integer range. The denominator is always strictly positive.
type Rational struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}

func rationalDifference(current, previous int64) *Rational {
	difference := new(big.Int).Sub(big.NewInt(current), big.NewInt(previous))
	return &Rational{Numerator: difference.String(), Denominator: big.NewInt(previous).String()}
}

type Totals struct {
	Revenue     int64     `json:"revenue"`
	Refunds     int64     `json:"refunds"`
	Procurement int64     `json:"procurement"`
	Logistics   int64     `json:"logistics"`
	Platform    int64     `json:"platform"`
	Advertising int64     `json:"advertising"`
	Other       int64     `json:"other"`
	NetRevenue  int64     `json:"netRevenue"`
	NetProfit   int64     `json:"netProfit"`
	Margin      *Rational `json:"margin"`
}

func checkedAdd(a, b int64) (int64, error) {
	if a < -MaxSafeInteger || a > MaxSafeInteger || b < -MaxSafeInteger || b > MaxSafeInteger || b > 0 && a > MaxSafeInteger-b || b < 0 && a < -MaxSafeInteger-b {
		return 0, ErrAmountRange
	}
	return a + b, nil
}

func totalValues(values [7]int64) (Totals, error) {
	for _, v := range values {
		if v < 0 || v > MaxSafeInteger {
			return Totals{}, ErrAmountRange
		}
	}
	t := Totals{Revenue: values[0], Refunds: values[1], Procurement: values[2], Logistics: values[3], Platform: values[4], Advertising: values[5], Other: values[6]}
	var err error
	t.NetRevenue, err = checkedAdd(t.Revenue, -t.Refunds)
	if err != nil {
		return Totals{}, err
	}
	t.NetProfit = t.NetRevenue
	for _, cost := range values[2:] {
		t.NetProfit, err = checkedAdd(t.NetProfit, -cost)
		if err != nil {
			return Totals{}, err
		}
	}
	if t.NetRevenue > 0 {
		t.Margin = &Rational{big.NewInt(t.NetProfit).String(), big.NewInt(t.NetRevenue).String()}
	}
	return t, nil
}

func (a Amounts) Totals() (Totals, error) {
	if !a.Valid() {
		return Totals{}, ErrInvalid
	}
	return totalValues(a.values())
}

func (t Totals) Add(other Totals) (Totals, error) {
	a := [7]int64{t.Revenue, t.Refunds, t.Procurement, t.Logistics, t.Platform, t.Advertising, t.Other}
	b := [7]int64{other.Revenue, other.Refunds, other.Procurement, other.Logistics, other.Platform, other.Advertising, other.Other}
	for i := range a {
		var err error
		a[i], err = checkedAdd(a[i], b[i])
		if err != nil {
			return Totals{}, err
		}
	}
	return totalValues(a)
}

type StoreAggregate struct {
	StoreID  string           `json:"storeId"`
	Period   Period           `json:"period"`
	Complete bool             `json:"complete"`
	Totals   Totals           `json:"totals"`
	Gaps     []Period         `json:"gaps"`
	Excluded []RecordEvidence `json:"excluded"`
	Records  []RecordEvidence `json:"records"`
}

func Aggregate(query Period, records []Record) (StoreAggregate, error) {
	result := StoreAggregate{Period: query, Gaps: []Period{}, Excluded: []RecordEvidence{}, Records: []RecordEvidence{}}
	if !query.Valid() {
		return result, ErrInvalid
	}
	ordered := append([]Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Period.Start < ordered[j].Period.Start })
	covered := make([]bool, query.Days())
	start, _ := parseDate(query.Start)
	previousEnd := ""
	for _, record := range ordered {
		if !record.Period.Valid() || !UUID(record.ID) || !UUID(record.StoreID) || record.Revision <= 0 || !record.Amounts.Valid() {
			return result, ErrInvalid
		}
		if !query.Intersects(record.Period) {
			continue
		}
		if previousEnd >= record.Period.Start {
			return result, ErrOverlap
		}
		previousEnd = record.Period.End
		if result.StoreID != "" && result.StoreID != record.StoreID {
			return result, ErrInvalid
		}
		result.StoreID = record.StoreID
		evidence := RecordEvidence{ID: record.ID, Revision: record.Revision, Period: record.Period}
		if !query.Contains(record.Period) {
			result.Excluded = append(result.Excluded, evidence)
			continue
		}
		totals, err := record.Amounts.Totals()
		if err != nil {
			return result, err
		}
		result.Totals, err = result.Totals.Add(totals)
		if err != nil {
			return result, err
		}
		result.Records = append(result.Records, evidence)
		from, _ := parseDate(record.Period.Start)
		offset := int(from.Sub(start) / (24 * time.Hour))
		for i := offset; i < offset+record.Period.Days(); i++ {
			covered[i] = true
		}
	}
	for i := 0; i < len(covered); {
		if covered[i] {
			i++
			continue
		}
		from := i
		for i < len(covered) && !covered[i] {
			i++
		}
		result.Gaps = append(result.Gaps, Period{start.AddDate(0, 0, from).Format(time.DateOnly), start.AddDate(0, 0, i-1).Format(time.DateOnly)})
	}
	result.Complete = len(result.Gaps) == 0 && len(result.Excluded) == 0
	return result, nil
}

type GoalEvaluation struct {
	State          string    `json:"state"`
	Through        string    `json:"through"`
	Totals         Totals    `json:"totals"`
	ExpectedProfit *Rational `json:"expectedProfit"`
	Completion     *Rational `json:"completion"`
}

func EvaluateGoal(goal GoalConfig, now time.Time, stores map[string]StoreAggregate) (GoalEvaluation, error) {
	result := GoalEvaluation{State: "pending_data"}
	if !goal.Valid() {
		return result, ErrInvalid
	}
	today := Today(now)
	if goal.Period.Start > today {
		result.State = "not_started"
		return result, nil
	}
	date, _ := parseDate(today)
	through := date.AddDate(0, 0, -1).Format(time.DateOnly)
	if through > goal.Period.End {
		through = goal.Period.End
	}
	if through < goal.Period.Start {
		return result, nil
	}
	result.Through = through
	period := Period{goal.Period.Start, through}
	for _, id := range goal.StoreIDs {
		aggregate, ok := stores[id]
		if !ok || !aggregate.Complete || aggregate.Period != period || aggregate.StoreID != id {
			return result, nil
		}
		var err error
		result.Totals, err = result.Totals.Add(aggregate.Totals)
		if err != nil {
			return GoalEvaluation{}, err
		}
	}
	totalDays, elapsed := int64(goal.Period.Days()), int64(period.Days())
	profit := big.NewInt(goal.Profit)
	expectedNumerator := new(big.Int).Mul(profit, big.NewInt(elapsed))
	result.ExpectedProfit = &Rational{expectedNumerator.String(), big.NewInt(totalDays).String()}
	completionNumerator := new(big.Int).Mul(big.NewInt(result.Totals.NetProfit), big.NewInt(totalDays))
	result.Completion = &Rational{completionNumerator.String(), expectedNumerator.String()}
	left := new(big.Int).Mul(completionNumerator, big.NewInt(10000))
	result.State = "abnormal"
	if left.Cmp(new(big.Int).Mul(expectedNumerator, big.NewInt(goal.NormalBPS))) >= 0 {
		result.State = "normal"
	} else if left.Cmp(new(big.Int).Mul(expectedNumerator, big.NewInt(goal.AttentionBPS))) >= 0 {
		result.State = "attention"
	}
	if goal.MinimumMarginBPS != nil && result.Totals.NetRevenue > 0 && result.State == "normal" {
		actualMargin := new(big.Int).Mul(big.NewInt(result.Totals.NetProfit), big.NewInt(10000))
		minimum := new(big.Int).Mul(big.NewInt(result.Totals.NetRevenue), big.NewInt(*goal.MinimumMarginBPS))
		if actualMargin.Cmp(minimum) < 0 {
			result.State = "attention"
		}
	}
	return result, nil
}
