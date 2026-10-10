package operationscockpit

import (
	"math"
	"strconv"
)

func MaintenanceMetadata(head GoalHead, scope Scope, access Access, scopeValid bool) (HeadMetadata, error) {
	if !head.CanMaintain(scope, access) {
		return HeadMetadata{}, ErrForbidden
	}
	return HeadMetadata{GoalID: head.ID, Revision: strconv.FormatInt(head.Revision, 10), ScopeValid: scopeValid, CanReconfigure: true}, nil
}

// AuthorizeGoalCommand validates the resource identity and CAS precondition.
// The persistence boundary must repeat it under the head lock using current
// server authorization and validate every new-scope Store in that transaction.
func AuthorizeGoalCommand(head *GoalHead, scope Scope, access Access, operation string, expected int64) error {
	if !scope.Valid() || !access.GoalsRead {
		return ErrForbidden
	}
	if operation == "create" {
		if !access.GoalsCreate {
			return ErrForbidden
		}
		if head != nil || expected != 0 {
			return ErrRevision
		}
		return nil
	}
	if operation != "update" && operation != "restore" {
		return ErrInvalid
	}
	if head == nil {
		return ErrNotFound
	}
	if !head.CanMaintain(scope, access) {
		return ErrForbidden
	}
	if expected <= 0 || head.Revision != expected || head.Revision == math.MaxInt64 {
		return ErrRevision
	}
	return nil
}
