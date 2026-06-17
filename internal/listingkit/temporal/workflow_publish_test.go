package temporal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"

	sheintestsuite "go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestPublishWorkflowRunsExpectedPhaseOrder(t *testing.T) {
	t.Parallel()

	env := newPublishWorkflowTestEnvironment()
	registerPublishWorkflowActivityNames(env)

	var phases []string
	prepared := &listingkit.SheinPreparedSubmitPayload{
		TaskID:           "task-1",
		Action:           "publish",
		RequestID:        "req-1",
		NeedsImageUpload: true,
		Snapshot:         &sheinpub.SubmitSnapshot{},
	}
	remoteResult := &listingkit.SheinRemoteSubmitResult{
		TaskID:       "task-1",
		Action:       "publish",
		RequestID:    "req-1",
		SupplierCode: "SUP-1",
		Response:     &sheinpub.SubmissionResponse{Success: true},
		Snapshot:     &sheinpub.SubmitSnapshot{},
	}

	env.OnActivity(activityNameBeginPublishAttempt, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) { phases = append(phases, "validate") })
	env.OnActivity(activityNameValidateReadiness, mock.Anything, mock.Anything).
		Return(nil)
	env.OnActivity(activityNamePrepareProduct, mock.Anything, mock.Anything).
		Return(prepared, nil).
		Run(func(args mock.Arguments) { phases = append(phases, "prepare_product") })
	env.OnActivity(activityNameUploadImages, mock.Anything, mock.Anything).
		Return(prepared, nil).
		Run(func(args mock.Arguments) { phases = append(phases, "upload_images") })
	env.OnActivity(activityNamePreValidate, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) { phases = append(phases, "pre_validate") })
	env.OnActivity(activityNameSubmitRemote, mock.Anything, mock.Anything).
		Return(remoteResult, nil).
		Run(func(args mock.Arguments) { phases = append(phases, "submit_remote") })
	env.OnActivity(activityNamePersistSuccess, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) { phases = append(phases, "persist_result") })

	env.ExecuteWorkflow(PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:      "task-1",
		Platform:    "shein",
		Action:      "publish",
		RequestID:   "req-1",
		RequestedAt: time.Now().UTC(),
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []string{
		"validate",
		"prepare_product",
		"upload_images",
		"pre_validate",
		"submit_remote",
		"persist_result",
	}, phases)
}

func TestPublishWorkflowPersistsFailureOnPreValidateError(t *testing.T) {
	t.Parallel()

	env := newPublishWorkflowTestEnvironment()
	registerPublishWorkflowActivityNames(env)

	prepared := &listingkit.SheinPreparedSubmitPayload{
		TaskID:           "task-1",
		Action:           "publish",
		RequestID:        "req-1",
		NeedsImageUpload: false,
		Snapshot:         &sheinpub.SubmitSnapshot{},
	}
	preValidateErr := errors.New("pre-validate failed")
	var persisted listingkit.SheinPersistSubmitFailureInput

	env.OnActivity(activityNameBeginPublishAttempt, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNameValidateReadiness, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNamePrepareProduct, mock.Anything, mock.Anything).Return(prepared, nil)
	env.OnActivity(activityNamePreValidate, mock.Anything, mock.Anything).Return(preValidateErr)
	env.OnActivity(activityNamePersistFailure, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			persisted = extractPersistFailureInput(t, args)
		})

	env.ExecuteWorkflow(PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:      "task-1",
		Platform:    "shein",
		Action:      "publish",
		RequestID:   "req-1",
		RequestedAt: time.Now().UTC(),
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.Equal(t, sheinpub.SubmissionPhasePreValidate, persisted.Phase)
	require.Equal(t, "task-1", persisted.TaskID)
	require.Equal(t, "req-1", persisted.RequestID)
	require.Contains(t, persisted.ErrorMessage, "pre-validate failed")
	require.NotNil(t, persisted.Snapshot)
}

func TestPublishWorkflowPersistsFailureDetailsOnSubmitRemoteError(t *testing.T) {
	t.Parallel()

	env := newPublishWorkflowTestEnvironment()
	registerPublishWorkflowActivityNames(env)

	snapshot := &sheinpub.SubmitSnapshot{SupplierCode: "SUP-1"}
	prepared := &listingkit.SheinPreparedSubmitPayload{
		TaskID:           "task-1",
		Action:           "publish",
		RequestID:        "req-1",
		NeedsImageUpload: false,
		Snapshot:         snapshot,
	}
	remoteResponse := &sheinpub.SubmissionResponse{
		Code:    "4001",
		Message: "remote rejected",
	}
	remoteErr := sdktemporal.NewNonRetryableApplicationError(
		"remote rejected",
		listingkit.SheinSubmitRemoteActivityErrorType,
		nil,
		listingkit.SheinSubmitRemoteActivityErrorDetails{
			ErrorMessage: "remote rejected",
			SupplierCode: "SUP-1",
			Response:     remoteResponse,
			Snapshot:     snapshot,
		},
	)
	var persisted listingkit.SheinPersistSubmitFailureInput

	env.OnActivity(activityNameBeginPublishAttempt, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNameValidateReadiness, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNamePrepareProduct, mock.Anything, mock.Anything).Return(prepared, nil)
	env.OnActivity(activityNamePreValidate, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNameSubmitRemote, mock.Anything, mock.Anything).
		Return((*listingkit.SheinRemoteSubmitResult)(nil), remoteErr)
	env.OnActivity(activityNamePersistFailure, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			persisted = extractPersistFailureInput(t, args)
		})

	env.ExecuteWorkflow(PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:      "task-1",
		Platform:    "shein",
		Action:      "publish",
		RequestID:   "req-1",
		RequestedAt: time.Now().UTC(),
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.Equal(t, sheinpub.SubmissionPhaseSubmitRemote, persisted.Phase)
	require.Equal(t, "SUP-1", persisted.SupplierCode)
	require.Equal(t, remoteResponse, persisted.Response)
	require.Equal(t, snapshot, persisted.Snapshot)
	require.Contains(t, persisted.ErrorMessage, "remote rejected")
}

func TestPublishWorkflowQueryReportsCurrentPhaseWhileRunning(t *testing.T) {
	t.Parallel()

	env := newPublishWorkflowTestEnvironment()
	registerPublishWorkflowActivityNames(env)

	prepared := &listingkit.SheinPreparedSubmitPayload{
		TaskID:           "task-1",
		Action:           "publish",
		RequestID:        "req-1",
		NeedsImageUpload: false,
	}
	remoteResult := &listingkit.SheinRemoteSubmitResult{
		TaskID:       "task-1",
		Action:       "publish",
		RequestID:    "req-1",
		SupplierCode: "SUP-1",
		Response:     &sheinpub.SubmissionResponse{Success: true},
	}
	var queried SheinPublishStateQueryResult

	env.OnActivity(activityNameBeginPublishAttempt, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNameValidateReadiness, mock.Anything, mock.Anything).After(time.Minute).Return(nil)
	env.OnActivity(activityNamePrepareProduct, mock.Anything, mock.Anything).Return(prepared, nil)
	env.OnActivity(activityNamePreValidate, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNameSubmitRemote, mock.Anything, mock.Anything).Return(remoteResult, nil)
	env.OnActivity(activityNamePersistSuccess, mock.Anything, mock.Anything).Return(nil)

	env.RegisterDelayedCallback(func() {
		resp, err := env.QueryWorkflow(SheinPublishQueryCurrentState)
		require.NoError(t, err)
		require.NoError(t, resp.Get(&queried))
	}, time.Second)

	env.ExecuteWorkflow(PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:      "task-1",
		Platform:    "shein",
		Action:      "publish",
		RequestID:   "req-1",
		RequestedAt: time.Now().UTC(),
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, sheinpub.SubmissionPhaseValidate, queried.CurrentPhase)
	require.True(t, queried.WorkflowRunning)
	require.Equal(t, "task-1", queried.TaskID)
	require.Equal(t, "req-1", queried.RequestID)
}

func TestPublishWorkflowReturnsPersistFailureErrorWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	env := newPublishWorkflowTestEnvironment()
	registerPublishWorkflowActivityNames(env)

	prepared := &listingkit.SheinPreparedSubmitPayload{
		TaskID:    "task-1",
		Action:    "publish",
		RequestID: "req-1",
	}
	env.OnActivity(activityNameBeginPublishAttempt, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNameValidateReadiness, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityNamePrepareProduct, mock.Anything, mock.Anything).Return(prepared, nil)
	env.OnActivity(activityNameUploadImages, mock.Anything, mock.Anything).Return(prepared, nil)
	env.OnActivity(activityNamePreValidate, mock.Anything, mock.Anything).Return(errors.New("pre-validate failed"))
	env.OnActivity(activityNamePersistFailure, mock.Anything, mock.Anything).Return(
		sdktemporal.NewNonRetryableApplicationError("persist failed", "persist_failure", nil),
	)

	env.ExecuteWorkflow(PublishWorkflow, SheinPublishWorkflowInput{
		TaskID:      "task-1",
		Platform:    "shein",
		Action:      "publish",
		RequestID:   "req-1",
		RequestedAt: time.Now().UTC(),
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "persist workflow failure after")
	require.Contains(t, env.GetWorkflowError().Error(), "pre-validate failed")
	require.Contains(t, env.GetWorkflowError().Error(), "persist failed")
}

func extractPersistFailureInput(t *testing.T, args mock.Arguments) listingkit.SheinPersistSubmitFailureInput {
	t.Helper()
	for _, arg := range args {
		switch value := arg.(type) {
		case listingkit.SheinPersistSubmitFailureInput:
			return value
		case *listingkit.SheinPersistSubmitFailureInput:
			if value != nil {
				return *value
			}
		}
	}
	t.Fatalf("persist failure input not found in mock args: %#v", []interface{}(args))
	return listingkit.SheinPersistSubmitFailureInput{}
}

type activityRegistrar interface {
	RegisterActivityWithOptions(a interface{}, options sdkactivity.RegisterOptions)
}

func newPublishWorkflowTestEnvironment() *sheintestsuite.TestWorkflowEnvironment {
	var suite sheintestsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	return env
}

func registerPublishWorkflowActivityNames(env activityRegistrar) {
	env.RegisterActivityWithOptions(func(ctx context.Context, in SheinPublishWorkflowInput) error {
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNameBeginPublishAttempt})
	env.RegisterActivityWithOptions(func(ctx context.Context, in SheinPublishWorkflowInput) error {
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNameValidateReadiness})
	env.RegisterActivityWithOptions(func(ctx context.Context, in SheinPublishWorkflowInput) (*listingkit.SheinPreparedSubmitPayload, error) {
		return nil, nil
	}, sdkactivity.RegisterOptions{Name: activityNamePrepareProduct})
	env.RegisterActivityWithOptions(func(ctx context.Context, in *listingkit.SheinPreparedSubmitPayload) (*listingkit.SheinPreparedSubmitPayload, error) {
		return nil, nil
	}, sdkactivity.RegisterOptions{Name: activityNameUploadImages})
	env.RegisterActivityWithOptions(func(ctx context.Context, in *listingkit.SheinPreparedSubmitPayload) error {
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNamePreValidate})
	env.RegisterActivityWithOptions(func(ctx context.Context, in *listingkit.SheinPreparedSubmitPayload) (*listingkit.SheinRemoteSubmitResult, error) {
		return nil, nil
	}, sdkactivity.RegisterOptions{Name: activityNameSubmitRemote})
	env.RegisterActivityWithOptions(func(ctx context.Context, in listingkit.SheinPersistSubmitSuccessInput) error {
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNamePersistSuccess})
	env.RegisterActivityWithOptions(func(ctx context.Context, in *listingkit.SheinPersistSubmitFailureInput) error {
		return nil
	}, sdkactivity.RegisterOptions{Name: activityNamePersistFailure})
}
