package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/httproute"
)

func agentAndPointRoutes(agent, points bool) []httproute.Descriptor {
	var routes []httproute.Descriptor
	for _, route := range currentWorkbenchApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	// Keep billing first: its early discovery must not skip later auth checks.
	for _, route := range currentCommercialBillingApplicationRoutes {
		routes = append(routes, httproute.Descriptor{Method: route.Method, Path: route.Path})
	}
	if agent {
		routes = append(routes, productAgentRoutes(nil)...)
		for _, route := range productReviewRoutes(nil, nil) {
			if route.Method != http.MethodPost || route.Path != "/api/product/text-proposals" {
				routes = append(routes, route)
			}
		}
	}
	if points {
		routes = append(routes, (memberPointLimitModule{}).routes()...)
	}
	return routes
}

func TestCurrentApplicationAgentAndMemberPointRoutesAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		agent, points bool
	}{{"neither", false, false}, {"agent_only", true, false}, {"points_only", false, true}, {"both", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			validate := func(routes []httproute.Descriptor) error {
				return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{ProductAgent: tc.agent, MemberPoints: tc.points})
			}
			require.NoError(t, validate(agentAndPointRoutes(tc.agent, tc.points)))
			require.Error(t, validate(agentAndPointRoutes(!tc.agent, tc.points)), "Agent routes must match only the Agent flag")
			require.Error(t, validate(agentAndPointRoutes(tc.agent, !tc.points)), "point routes must match only the point flag")
		})
	}
}

func TestCurrentApplicationAgentAndPointPermissionsSurviveComposition(t *testing.T) {
	routes := agentAndPointRoutes(true, true)
	validate := func(routes []httproute.Descriptor) error {
		return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{ProductAgent: true, MemberPoints: true})
	}
	require.NoError(t, validate(routes))
	for i, route := range routes {
		if !strings.HasPrefix(route.Path, productAgentBase) && !strings.HasPrefix(route.Path, memberPointLimitBase) {
			continue
		}
		for name, mutate := range map[string]func(*httproute.Descriptor){
			"module":     func(d *httproute.Descriptor) { d.Module = "wrong" },
			"auth":       func(d *httproute.Descriptor) { d.AuthPolicy = httproute.AuthPolicyPublic },
			"permission": func(d *httproute.Descriptor) { d.Permission = "" },
			"live": func(d *httproute.Descriptor) {
				d.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyCachedRead
			},
			"timeout": func(d *httproute.Descriptor) { d.RequestTimeout = 0 },
		} {
			t.Run(route.Method+" "+route.Path+"/"+name, func(t *testing.T) {
				changed := append([]httproute.Descriptor(nil), routes...)
				mutate(&changed[i])
				require.Error(t, validate(changed))
			})
		}
	}
}
