package supplychainapp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"task-processor/internal/storecenter"
)

type operationModulePolicy struct {
	modules     []string
	unavailable bool
}

func (p operationModulePolicy) RoleModules(_ context.Context, org string, _ []string) (map[string][]string, error) {
	if p.unavailable {
		return nil, authz.ErrRolePolicyUnavailable
	}
	return map[string][]string{authz.EnterpriseRoleKey(org, 1): p.modules}, nil
}

type operationStoreCheck struct {
	storecenter.ProductExecutionReader
	err error
}

func (s operationStoreCheck) ReadProductExecution(context.Context, storecenter.ProductExecutionSubject, string, storecenter.ProductExecutionAuthorizer, time.Time) (storecenter.ProductExecutionMaterial, error) {
	return storecenter.ProductExecutionMaterial{}, s.err
}

type unusedOperationOwner struct{}

func (unusedOperationOwner) AuthorizeOwner(context.Context) (collection.AuthorizedOwner, error) {
	return collection.AuthorizedOwner{}, collection.ErrUnavailable
}

type activityAuthorizationRepo struct {
	preparation.OperationRepository
	op    preparation.Operation
	item  preparation.OperationItem
	stops int
}

func (r *activityAuthorizationRepo) ReadExecutionOperation(context.Context, string, string) (preparation.Operation, error) {
	return r.op, nil
}
func (r *activityAuthorizationRepo) StopOperation(ctx context.Context, proof preparation.OperationStopAccess, _ string) (preparation.OperationItem, error) {
	_, err := proof.Read(ctx)
	if err != nil {
		return preparation.OperationItem{}, err
	}
	r.stops++
	r.item.Status = preparation.ItemDenied
	return r.item, nil
}
func (r *activityAuthorizationRepo) ListOperationItems(context.Context, collection.Scope, string, collection.Query) (collection.Page[preparation.OperationItem], error) {
	return collection.Page[preparation.OperationItem]{Items: []preparation.OperationItem{r.item}}, nil
}

type authorizationOptimizer struct{ calls int }

func (o *authorizationOptimizer) Optimize(context.Context, preparation.Operation, preparation.OperationItem, string) (string, error) {
	o.calls++
	return uuid.NewString(), nil
}

func TestOperationActivitiesStopOnActualAgentOrStoreDenialAndRetryOutages(t *testing.T) {
	for _, tc := range []struct {
		name, action      string
		modules           []string
		policyUnavailable bool
		storeError        error
		stopped           bool
	}{
		{"Agent grant revoked while Supply remains", preparation.OperationOptimize, []string{"supply-mine"}, false, nil, true},
		{"Agent policy unavailable", preparation.OperationOptimize, []string{"supply-mine", "acquisition"}, true, nil, false},
		{"individual Store revoked", preparation.OperationUpload, []string{"supply-mine"}, false, storecenter.ErrNotFound, true},
		{"Store unavailable", preparation.OperationUpload, []string{"supply-mine"}, false, storecenter.ErrDependencyUnavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			permissions, err := authz.NewListingKitAuthorizer(nil, nil)
			require.NoError(t, err)
			permissions.SetRolePolicyReader(operationModulePolicy{tc.modules, tc.policyUnavailable})
			iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"pagination":{"totalResult":"1"},"authorizations":[{"id":"member-a","project":{"id":"project-a"},"organization":{"id":"org-a"},"user":{"id":"actor-a"},"state":"STATE_ACTIVE","roles":[{"key":%q}]}]}`, authz.EnterpriseRoleKey("org-a", 1))
			}))
			defer iam.Close()
			authority := OrganizationExecutionAuthorizer{Client: zitadel.NewAuthorizationClient(iam.URL, iam.Client()), ServiceToken: func(context.Context) (string, error) { return "fixture", nil }, ProjectID: "project-a", Permissions: permissions}
			repo := &activityAuthorizationRepo{op: preparation.Operation{ID: uuid.NewString(), Owner: collection.Scope{"org-a", "actor-a", "member-a"}, Input: preparation.OperationInput{PreparationID: uuid.NewString(), ExpectedRevision: 1, StoreID: uuid.NewString(), Action: tc.action}}, item: preparation.OperationItem{SourceID: uuid.NewString(), Status: preparation.ItemPending}}
			if tc.action == preparation.OperationOptimize {
				repo.op.Input.TitleTemplateID = uuid.NewString()
				repo.op.Input.TitleTemplateRevision = "1"
				repo.op.Input.TitleQuoteHash = collection.Digest("confirmed quote")
			}
			service, err := preparation.NewOperationService(&preparation.Service{}, unusedOperationOwner{}, repo)
			require.NoError(t, err)
			_, err = service.WithExecution(OperationExecutionAuthorization{OrganizationExecutionAuthorizer: authority, Stores: operationStoreCheck{err: tc.storeError}})
			require.NoError(t, err)
			optimizer := &authorizationOptimizer{}
			activities := OperationActivities{Operations: service, Repository: repo, Optimizer: optimizer}
			in := OperationExecution{OrganizationID: "org-a", OperationID: repo.op.ID}
			page, err := activities.List(ctx, in)
			if tc.stopped {
				require.NoError(t, err)
				require.True(t, page.Cancelled)
				require.Equal(t, 1, repo.stops)
			} else {
				require.ErrorIs(t, err, preparation.ErrUnavailable)
				require.Zero(t, repo.stops)
			}
			repo.item.Status = preparation.ItemPending
			repo.stops = 0
			item, err := activities.Process(ctx, in, repo.item.SourceID)
			if tc.stopped {
				require.NoError(t, err)
				require.Equal(t, preparation.ItemDenied, item.Status)
				require.Equal(t, 1, repo.stops)
			} else {
				require.ErrorIs(t, err, preparation.ErrUnavailable)
				require.Zero(t, repo.stops)
			}
			require.Zero(t, optimizer.calls, "neither revoked authority nor unavailable authorization dispatches a model")
		})
	}
}
