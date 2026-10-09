package inviteflow

import (
	"context"
	"strings"
)

type NoticeQuery struct {
	OrganizationID, Contact, RecipientID, After string
	Limit                                       int
}
type NoticeReader interface {
	ListNoticeInvitations(context.Context, NoticeQuery) ([]Invitation, string, error)
}

// NotificationFacts is pure: it does not call reconcile, write grants, deliver
// mail or transition accepting/expired invitations.
func (s *Service) NotificationFacts(ctx context.Context, recipient bool, after string, limit int) ([]Invitation, string, error) {
	if s == nil || s.Store == nil || limit < 1 || limit > 100 {
		return nil, "", ErrUnavailable
	}
	r, ok := s.Store.(NoticeReader)
	if !ok {
		return nil, "", ErrUnavailable
	}
	q := NoticeQuery{After: after, Limit: limit}
	if recipient {
		i, e := s.principal(ctx)
		if e != nil {
			return nil, "", e
		}
		if s.ReadSelf == nil {
			return nil, "", ErrUnavailable
		}
		self, e := s.ReadSelf(ctx)
		if e != nil {
			return nil, "", ErrUnavailable
		}
		if self.UserID != i.UserID || self.Email == nil || self.EmailVerified == nil || !*self.EmailVerified {
			return nil, "", ErrPermission
		}
		q.Contact = strings.ToLower(strings.TrimSpace(*self.Email))
		q.RecipientID = i.UserID
	} else {
		if s.Manager == nil {
			return nil, "", ErrUnavailable
		}
		i, e := s.Manager(ctx, "")
		if e != nil {
			return nil, "", e
		}
		q.OrganizationID = i.EffectiveOrganizationID
	}
	rows, next, e := r.ListNoticeInvitations(ctx, q)
	if e != nil {
		return nil, "", e
	}
	for j, inv := range rows {
		if inv.ProjectID != s.ProjectID || !recipient && inv.OrganizationID != q.OrganizationID || recipient && (inv.Contact != q.Contact || inv.RecipientID != "" && inv.RecipientID != q.RecipientID) {
			return nil, "", ErrUnavailable
		}
		rows[j] = s.visible(inv)
	}
	return rows, next, nil
}
