package temporal

import (
	"context"
	"encoding/base64"
	"fmt"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/shared/aiidentity"

	"go.temporal.io/sdk/workflow"
)

const (
	WorkerWireModeOrganization WorkerWireMode = "organization-v1"
	OrganizationTaskQueue                     = "image-agent-organization-v1"
	organizationWorkflowName                  = "ImageAgentOrganizationWorkflowV1"
)

func (a *Activities) reconcileGenerationBeforeExecution(ctx context.Context, runID string, identity imageagent.ExecutionIdentity, revision int64, slotID string, attempt int) error {
	if a.generationRecovery == nil {
		return nil
	}
	if err := imageagent.ValidateOrganizationExecution(identity, runID); err != nil {
		return err
	}
	if identity.MemberID == "" || revision <= 0 || slotID == "" || attempt <= 0 {
		return imageagent.ErrIdentityRequired
	}
	// Only the fixed fact owner is consulted here. This internal seam has no
	// reserve, provider, artifact download or approval capability. Revocation
	// cannot erase already incurred consumption; ordinary execution below
	// still requires the complete live authorization and catalog boundary.
	finalization, cancel := providerFinalizationContext(ctx)
	defer cancel()
	_, err := a.generationRecovery.ReconcileExistingGeneration(finalization, imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID}, PlanRevision: revision, SlotID: slotID, Attempt: attempt})
	return err
}

func (a *Activities) restoreExecutionIdentity(ctx context.Context, runID string, identity imageagent.ExecutionIdentity) (context.Context, error) {
	if a.executionAuthorizer == nil {
		return restoreActivityIdentity(ctx, identity)
	}
	if err := imageagent.ValidateOrganizationExecution(identity, runID); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projection, err := a.repository.GetProjection(ctx, imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID})
	if err != nil {
		return nil, err
	}
	run := projection.Run
	if run.ScopeProtocol != identity.ScopeProtocol || run.TenantID != identity.TenantID || run.UserID != identity.UserID || run.MemberID != identity.MemberID || run.ID != runID || run.BusinessTaskID != identity.BusinessTaskID {
		return nil, imageagent.ErrIdentityRequired
	}
	if err := a.executionAuthorizer.AuthorizeExecution(ctx, identity); err != nil {
		return nil, fmt.Errorf("current organization authorization: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: run.TenantID, EffectiveOrganizationID: run.TenantID, EffectiveMemberID: run.MemberID, UserID: run.UserID})
	return aiidentity.WithIdentity(ctx, aiidentity.Identity{AgentRunID: run.ID, TenantID: run.TenantID, UserID: run.UserID, BusinessTaskID: run.BusinessTaskID, TraceID: identity.TraceID}), nil
}

func validateWorkflowScope(ctx workflow.Context, identity imageagent.ExecutionIdentity, runID string) error {
	scoped := workflow.GetInfo(ctx).TaskQueueName == OrganizationTaskQueue
	if scoped {
		return imageagent.ValidateOrganizationExecution(identity, runID)
	}
	if identity.ScopeProtocol != "" || identity.RunID != "" {
		return imageagent.ErrIdentityRequired
	}
	return nil
}

// The catalog is immutable run-owned input, not an authority supplied by a
// deserialized Activity argument. Compare the complete snapshot before I/O.
func (a *Activities) validateOrganizationCatalog(ctx context.Context, identity imageagent.ExecutionIdentity, runID string, catalog imageagent.AssetCatalog) error {
	if a.executionAuthorizer == nil {
		return nil
	}
	persisted, err := a.repository.GetAssetCatalog(ctx, imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID})
	if err != nil {
		return err
	}
	validated, err := imageagent.NormalizeAssetCatalog(catalog)
	if err != nil {
		return err
	}
	// PostgreSQL timestamps have lower precision than JSON workflow payloads;
	// the canonical content hash and version bind every execution input field.
	if persisted.Manifest.Version != validated.Manifest.Version || persisted.Manifest.Hash != validated.Manifest.Hash {
		return fmt.Errorf("organization catalog does not match persisted snapshot: %w", imageagent.ErrIdentityRequired)
	}
	return nil
}

func NewOrganizationClient(client sdkWorkflowClient) *Client {
	return &Client{client: client, organizationScope: true}
}
func (c *Client) validateIdentityScope(identity imageagent.ExecutionIdentity) error {
	if c == nil {
		return imageagent.ErrIdentityRequired
	}
	if c.organizationScope {
		if identity.ScopeProtocol != imageagent.OrganizationScopeProtocol {
			return imageagent.ErrIdentityRequired
		}
	} else if identity.ScopeProtocol != "" || identity.RunID != "" {
		return imageagent.ErrIdentityRequired
	}
	return nil
}

func (c *Client) validateCommandScope(identity imageagent.ExecutionIdentity, runID string) error {
	if err := c.validateIdentityScope(identity); err != nil {
		return err
	}
	if c.organizationScope {
		return imageagent.ValidateOrganizationExecution(identity, runID)
	}
	return validateCommandIdentity(identity, runID)
}
func (c *Client) taskQueue() string {
	if c.organizationScope {
		return OrganizationTaskQueue
	}
	return TaskQueueV3
}
func scopedWorkflowID(identity imageagent.ExecutionIdentity, runID string) string {
	id := WorkflowID(identity.TenantID, identity.UserID, runID)
	if identity.ScopeProtocol == imageagent.OrganizationScopeProtocol {
		encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
		return "organization-v1:" + encode(identity.TenantID) + ":" + encode(identity.UserID) + ":" + encode(runID)
	}
	return id
}
func (c *Client) workflowName() string {
	if c.organizationScope {
		return organizationWorkflowName
	}
	return workflowNameImageAgent
}

func validateActivityMode(activities *Activities, mode WorkerWireMode) error {
	if (mode == WorkerWireModeOrganization) != (activities.executionAuthorizer != nil) {
		return fmt.Errorf("activity organization authorization mode mismatch")
	}
	return nil
}
