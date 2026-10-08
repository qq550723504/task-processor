package membership

import "context"

type NoticeOperationReader interface {
	ListNoticeOperations(context.Context, OperationScope, string, int) ([]Operation, string, error)
}

func (c *Commands) NotificationFacts(ctx context.Context, after string, limit int) ([]Operation, string, error) {
	fresh, i, e := c.current(ctx)
	if e != nil {
		return nil, "", e
	}
	r, ok := c.store.(NoticeOperationReader)
	if !ok {
		return nil, "", ErrUnavailable
	}
	scope := OperationScope{ProjectID: c.service.projectID, OrganizationID: i.EffectiveOrganizationID, ActorID: i.UserID}
	rows, next, e := r.ListNoticeOperations(fresh, scope, after, limit)
	if e != nil {
		return nil, "", e
	}
	_, again, e := c.current(ctx)
	if e != nil {
		return nil, "", e
	}
	if again.UserID != i.UserID || again.EffectiveOrganizationID != i.EffectiveOrganizationID {
		return nil, "", ErrPermission
	}
	return rows, next, nil
}
