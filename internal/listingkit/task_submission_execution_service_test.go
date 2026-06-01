package listingkit

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestTaskSubmissionExecutionServiceBuildSheinSubmitProductAPIUsesResolvedStoreID(t *testing.T) {
	t.Parallel()

	var lastStoreID int64
	var builderCtx context.Context
	exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api:         &stubSheinProductAPI{},
			lastStoreID: &lastStoreID,
			lastCtx:     &builderCtx,
		},
		resolveSheinStoreID: func(_ context.Context, _ *Task) (int64, error) {
			return 903, nil
		},
	})
	task := &Task{
		TenantID: "373211199677923496",
		UserID:   "user-submit",
		Request:  &GenerateRequest{SheinStoreID: 903},
	}

	api, err := exec.buildSheinSubmitProductAPI(context.Background(), task)
	if err != nil {
		t.Fatalf("buildSheinSubmitProductAPI() error = %v", err)
	}
	if api == nil {
		t.Fatal("expected product api")
	}
	if lastStoreID != 903 {
		t.Fatalf("builder store id = %d, want 903", lastStoreID)
	}
	identity := openaiclient.IdentityFromContext(builderCtx)
	if identity.TenantID != task.TenantID {
		t.Fatalf("builder context tenant id = %q, want %q", identity.TenantID, task.TenantID)
	}
	if identity.UserID != task.UserID {
		t.Fatalf("builder context user id = %q, want %q", identity.UserID, task.UserID)
	}
}

func TestTaskSubmissionExecutionServiceExecuteSheinSubmitRemoteRoutesByAction(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{SupplierCode: "SKU-1"}
	cases := []struct {
		name       string
		action     string
		api        stubSheinProductAPI
		wantCalled string
		wantCode   string
		wantMsg    string
	}{
		{
			name:   "publish",
			action: "publish",
			api: stubSheinProductAPI{
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "publish ok"},
			},
			wantCalled: "publish",
			wantCode:   "0",
			wantMsg:    "publish ok",
		},
		{
			name:   "save draft",
			action: "save_draft",
			api: stubSheinProductAPI{
				saveResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "draft ok"},
			},
			wantCalled: "save_draft",
			wantCode:   "0",
			wantMsg:    "draft ok",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := ""
			api := tc.api
			api.publishHook = func(*sheinproduct.Product) { called = "publish" }
			api.saveHook = func(*sheinproduct.Product) { called = "save_draft" }
			exec := newTaskSubmissionExecutionService(taskSubmissionExecutionServiceConfig{})

			response, err := exec.executeSheinSubmitRemote(api, tc.action, product)
			if err != nil {
				t.Fatalf("executeSheinSubmitRemote() error = %v", err)
			}
			if response == nil {
				t.Fatal("executeSheinSubmitRemote() response = nil, want summary")
			}
			if response.Code != tc.wantCode || response.Message != tc.wantMsg {
				t.Fatalf("executeSheinSubmitRemote() response = %+v, want code=%q msg=%q", response, tc.wantCode, tc.wantMsg)
			}
			if called != tc.wantCalled {
				t.Fatalf("called api method = %q, want %q", called, tc.wantCalled)
			}
		})
	}
}
