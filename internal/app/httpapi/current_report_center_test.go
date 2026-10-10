package httpapi

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/httproute"
	reporthttp "task-processor/internal/reportcenter/httpapi"
	"testing"
	"time"
)

func TestReportCenterNativeAdmissionRequiresExactBoundaryAndAllRoutes(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes := reporthttp.Routes(nil)
	all := append(append([]httproute.Descriptor{}, base...), routes...)
	validate := func(r []httproute.Descriptor, on bool) error {
		return validateCurrentApplicationRoutesInternal(r, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{ReportCenter: on})
	}
	require.NoError(t, validate(all, true))
	require.Error(t, validate(all, false))
	require.Error(t, validate(base, true))
	for i := range routes {
		for _, mutate := range []func(*httproute.Descriptor){
			func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyPublic },
			func(r *httproute.Descriptor) {
				r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyContextRead
			},
			func(r *httproute.Descriptor) { r.Permission = "workbench.chat.read" },
			func(r *httproute.Descriptor) { r.Module = "ai-workbench" },
			func(r *httproute.Descriptor) { r.RequestTimeout = 11 * time.Second },
			func(r *httproute.Descriptor) { r.RejectUnreadRequestBody = !r.RejectUnreadRequestBody },
		} {
			changed := append([]httproute.Descriptor{}, all...)
			mutate(&changed[len(base)+i])
			require.Error(t, validate(changed, true))
		}
	}
}
func TestReportCenterOptionRequiresOneIndependentPool(t *testing.T) {
	source, reports := &gorm.DB{}, &gorm.DB{}
	for _, options := range [][]CurrentApplicationOption{{WithReportCenter(nil)}, {WithReportCenter(source)}, {WithReportCenter(reports), WithReportCenter(reports)}, {WithReportCenter(reports), WithProjectCenter(reports)}} {
		var o currentApplicationOptions
		for _, opt := range options {
			opt(&o)
		}
		require.Error(t, validateReportCenterPool(o, source))
	}
	var o currentApplicationOptions
	WithReportCenter(reports)(&o)
	require.NoError(t, validateReportCenterPool(o, source))
}
