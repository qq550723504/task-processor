package operationscockpit

import "time"

type Query struct {
	Module   string
	Period   Period
	StoreIDs []string
}

func (q Query) Valid() bool {
	if !q.Period.Valid() || len(q.StoreIDs) > 500 {
		return false
	}
	if q.Module != "goals" && q.Module != "stores" && q.Module != "alerts" && q.Module != "advice" {
		return false
	}
	seen := map[string]bool{}
	for _, id := range q.StoreIDs {
		if !UUID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	_, err := PreviousPeriod(q.Period)
	return err == nil
}

func (a Access) Allows(module string) bool {
	switch module {
	case "goals":
		return a.GoalsRead
	case "stores":
		return a.StoresRead
	case "alerts":
		return a.AlertsRead
	case "advice":
		return a.AdviceRead
	default:
		return false
	}
}

func PreviousPeriod(period Period) (Period, error) {
	if !period.Valid() {
		return Period{}, ErrInvalid
	}
	start, _ := parseDate(period.Start)
	previous := Period{Start: start.AddDate(0, 0, -period.Days()).Format(time.DateOnly), End: start.AddDate(0, 0, -1).Format(time.DateOnly)}
	if !previous.Valid() {
		return Period{}, ErrInvalid
	}
	return previous, nil
}

func GoalThrough(goal GoalConfig, now time.Time) (Period, bool) {
	if !goal.Valid() {
		return Period{}, false
	}
	date, _ := parseDate(Today(now))
	through := date.AddDate(0, 0, -1).Format(time.DateOnly)
	if through > goal.Period.End {
		through = goal.Period.End
	}
	if through < goal.Period.Start {
		return Period{}, false
	}
	return Period{Start: goal.Period.Start, End: through}, true
}

// Snapshot is internal use-case data. HTTP consumers build bounded page views;
// they must not publish every Store's record evidence as a single giant result.
type Snapshot struct {
	CapturedAt      time.Time
	Today           string
	Period          Period
	Stores          map[string]StoreAggregate
	Previous        map[string]StoreAggregate
	Goal            *GoalVersion
	GoalStores      map[string]StoreAggregate
	Evaluation      *GoalEvaluation
	Head            *HeadMetadata
	GoalUnavailable bool
}

func Growth(current, previous StoreAggregate) *Rational {
	expected, err := PreviousPeriod(current.Period)
	if err != nil || previous.Period != expected || current.StoreID != previous.StoreID || !UUID(current.StoreID) || !current.Complete || !previous.Complete || previous.Totals.NetProfit <= 0 {
		return nil
	}
	return rationalDifference(current.Totals.NetProfit, previous.Totals.NetProfit)
}
