package operationscockpit

import "time"

// Command is constructed with the authenticated server scope. Creator and
// membership identity are never supplied in a mutable goal payload.
type Command struct {
	Scope                       Scope
	Key, Operation, ID, StoreID string
	Expected, SourceRevision    int64
	Fact                        *FactInput
	Goal                        *GoalConfig
}

func (c Command) Valid(now time.Time) bool {
	if !c.Scope.Valid() || !UUID(c.Key) || !UUID(c.ID) || c.Expected < 0 {
		return false
	}
	switch c.Operation {
	case "fact_create", "fact_update":
		return UUID(c.StoreID) && c.Fact != nil && c.Fact.Valid(now) && c.Goal == nil && c.SourceRevision == 0 && ((c.Operation == "fact_create" && c.Expected == 0) || (c.Operation == "fact_update" && c.Expected > 0))
	case "goal_create", "goal_update":
		return c.StoreID == "" && c.Fact == nil && c.Goal != nil && c.Goal.Valid() && c.SourceRevision == 0 && ((c.Operation == "goal_create" && c.Expected == 0) || (c.Operation == "goal_update" && c.Expected > 0))
	case "goal_restore":
		return c.StoreID == "" && c.Fact == nil && c.Goal == nil && c.Expected > 0 && c.SourceRevision > 0
	default:
		return false
	}
}

type Receipt struct {
	CommandID   string    `json:"commandId"`
	Operation   string    `json:"operation"`
	ID          string    `json:"id"`
	Revision    string    `json:"revision"`
	CommittedAt time.Time `json:"committedAt"`
}

type GoalVersion struct {
	ID        string     `json:"id"`
	CreatorID string     `json:"creatorId"`
	Revision  int64      `json:"revision,string"`
	Config    GoalConfig `json:"config"`
	UpdatedBy string     `json:"updatedBy"`
	UpdatedAt time.Time  `json:"updatedAt"`
}
