package httpapi

import (
	"errors"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	memberhttp "task-processor/internal/organization/membership/httpapi"
	"time"
)

var currentMembershipRoutes = []currentApplicationRoute{
	{Method: "GET", Path: "/api/v1/account/members"},
	{Method: "GET", Path: "/api/v1/account/members/:member_id"},
	{Method: "POST", Path: "/api/v1/account/members/invitations"},
	{Method: "POST", Path: "/api/v1/account/members/:member_id/role"},
	{Method: "POST", Path: "/api/v1/account/members/:member_id/remove"},
	{Method: "GET", Path: "/api/v1/account/member-operations/:operation_id"},
	{Method: "POST", Path: "/api/v1/account/member-operations/:operation_id/verify"},
}

func validateMembershipDescriptors(routes []httproute.Descriptor) error {
	for i, want := range currentMembershipRoutes {
		found := false
		permission := authz.PermissionWorkbenchOrganizationMemberManage
		if i < 2 {
			permission = authz.PermissionWorkbenchOrganizationMemberRead
		}
		for _, got := range routes {
			if got.Path != want.Path || got.Method != want.Method {
				continue
			}
			found = true
			if got.Module != memberhttp.ModuleName || got.Permission != permission || got.AuthPolicy != httproute.AuthPolicyCurrentIdentity || got.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || got.OrganizationTargetResolver == nil || !got.RejectUnreadRequestBody || got.RequestTimeout != 15*time.Second || got.Handler == nil {
				return errors.New("membership route authorization contract mismatch")
			}
		}
		if !found {
			return errors.New("membership route missing")
		}
	}
	return nil
}
