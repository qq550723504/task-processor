package agent

import "time"

type Limits struct {
	Steps, ModelCalls  int
	Tokens, CostMicros int64
	Currency           string
	Runtime            time.Duration
}

func (l Limits) Valid() bool {
	return l.Steps > 0 && l.ModelCalls > 0 && l.Tokens > 0 && l.CostMicros > 0 &&
		l.Runtime > 0 && len(l.Currency) == 3 && l.Currency[0] >= 'A' && l.Currency[0] <= 'Z' &&
		l.Currency[1] >= 'A' && l.Currency[1] <= 'Z' && l.Currency[2] >= 'A' && l.Currency[2] <= 'Z'
}

type Usage struct {
	Steps, ModelCalls  int
	Tokens, CostMicros int64
}

func (u *Usage) Step(l Limits) StopReason {
	if u.Steps >= l.Steps {
		return StopSteps
	}
	u.Steps++
	return ""
}

// Reserve uses subtraction to avoid overflow and changes nothing on refusal.
func (u *Usage) Reserve(l Limits, q Quote) StopReason {
	if u.Steps >= l.Steps {
		return StopSteps
	}
	if u.ModelCalls >= l.ModelCalls {
		return StopModelCalls
	}
	if !q.Known || q.Tokens <= 0 || q.CostMicros < 0 || q.Currency != l.Currency {
		return StopUsageUnknown
	}
	if q.Tokens > l.Tokens-u.Tokens {
		return StopTokens
	}
	if q.CostMicros > l.CostMicros-u.CostMicros {
		return StopCost
	}
	u.Steps++
	u.ModelCalls++
	u.Tokens += q.Tokens
	u.CostMicros += q.CostMicros
	return ""
}

func (u *Usage) Settle(q Quote, observed ObservedUsage) StopReason {
	if !observed.Known || observed.Currency != q.Currency || observed.Tokens < 0 ||
		observed.CostMicros < 0 || observed.Tokens > q.Tokens || observed.CostMicros > q.CostMicros {
		return StopUsageUnknown
	}
	u.Tokens -= q.Tokens - observed.Tokens
	u.CostMicros -= q.CostMicros - observed.CostMicros
	return ""
}
