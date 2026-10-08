package httpapi

import (
	"errors"
	"strings"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	memberhttp "task-processor/internal/organization/membership/httpapi"
	"time"
)

var currentMembershipRoutes = []currentApplicationRoute{
	{Method: "GET", Path: "/api/v1/account/roles"},
	{Method: "POST", Path: "/api/v1/account/roles"},
	{Method: "POST", Path: "/api/v1/account/roles/:role_id/permissions"},
	{Method: "GET", Path: "/api/v1/account/members"},
	{Method: "GET", Path: "/api/v1/account/members/summary"},
	{Method: "GET", Path: "/api/v1/account/members/:member_id"},
	{Method: "POST", Path: "/api/v1/account/members/:member_id/role"},
	{Method: "POST", Path: "/api/v1/account/members/:member_id/remove"},
	{Method: "GET", Path: "/api/v1/account/member-operations/:operation_id"},
	{Method: "GET", Path: "/api/v1/account/member-operations"},
	{Method: "POST", Path: "/api/v1/account/member-operations/:operation_id/verify"},
	{Method: "GET", Path: "/api/v1/account/member-invitations"},
	{Method: "POST", Path: "/api/v1/account/member-invitations"},
	{Method: "GET", Path: "/api/v1/account/member-invitations/summary"},
	{Method: "GET", Path: "/api/v1/account/member-invitations/:invitation_id"},
	{Method: "POST", Path: "/api/v1/account/member-invitations/:invitation_id/cancel"},
	{Method: "POST", Path: "/api/v1/account/member-invitations/:invitation_id/resend"},
	{Method: "GET", Path: "/api/v1/account/invitations/:invitation_id"},
	{Method: "POST", Path: "/api/v1/account/invitations/:invitation_id/accept"},
	{Method: "POST", Path: "/api/v1/account/invitations/:invitation_id/decline"},
}

func validateMembershipDescriptors(routes []httproute.Descriptor) error {
	for _, want := range currentMembershipRoutes {
		found := false
		permission := authz.PermissionWorkbenchOrganizationMemberManage
		if (strings.HasPrefix(want.Path, "/api/v1/account/members") || want.Path == "/api/v1/account/roles") && want.Method == "GET" || want.Path == "/api/v1/account/member-invitations/summary" {
			permission = authz.PermissionWorkbenchOrganizationMemberRead
		}
		policy := httproute.OrganizationAccessPolicyLiveWrite
		recipient := strings.HasPrefix(want.Path, "/api/v1/account/invitations/")
		if recipient {
			policy = httproute.OrganizationAccessPolicyNone
			permission = ""
		}
		for _, got := range routes {
			if got.Path != want.Path || got.Method != want.Method {
				continue
			}
			found = true
			if got.Module != memberhttp.ModuleName || got.Permission != permission || got.AuthPolicy != httproute.AuthPolicyCurrentIdentity || got.OrganizationAccessPolicy != policy || (got.OrganizationTargetResolver == nil) != recipient || !got.RejectUnreadRequestBody || got.RequestTimeout != 15*time.Second || got.Handler == nil {
				return errors.New("membership route authorization contract mismatch")
			}
		}
		if !found {
			return errors.New("membership route missing")
		}
	}
	return nil
}
