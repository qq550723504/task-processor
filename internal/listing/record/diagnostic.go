package record

import (
	"context"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"
	contract "task-processor/internal/marketplace/validator"
)

// DiagnosticEvaluator is the existing v2 computation seam, not a new rule owner.
type DiagnosticEvaluator interface {
	Validate(contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error)
}

type DiagnosticService struct {
	reader                      Reader
	evaluator                   DiagnosticEvaluator
	auth                        Authorizer
	ruleVersion, bindingVersion string
}

func NewDiagnosticService(reader Reader, evaluator DiagnosticEvaluator, auth Authorizer, ruleVersion, bindingVersion string) (*DiagnosticService, error) {
	if reader == nil || evaluator == nil || auth == nil || ruleVersion == "" || bindingVersion == "" {
		return nil, ErrUnavailable
	}
	return &DiagnosticService{reader, evaluator, auth, ruleVersion, bindingVersion}, nil
}

func (s *DiagnosticService) Diagnose(ctx context.Context, id string, action contract.Action, expected string) (contract.DiagnosticResult, error) {
	if err := ctx.Err(); err != nil {
		return contract.DiagnosticResult{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	actor := listingtask.Actor{TenantID: identity.EffectiveOrganizationID, UserID: identity.UserID, Roles: identity.Roles}
	if !ok || identity.TenantID != actor.TenantID || listingtask.ValidateActor(actor) != nil || !time.Now().Before(identity.TokenExpiresAt) || !s.auth.Authorize(actor.UserID, actor.Roles, authz.PermissionListingKitAdminRead) {
		return contract.DiagnosticResult{}, ErrForbidden
	}
	stored, err := s.reader.ReadOfflinePackage(ctx, actor, id)
	if ctx.Err() != nil {
		return contract.DiagnosticResult{}, ctx.Err()
	}
	if err != nil {
		return contract.DiagnosticResult{}, err
	}
	result, err := s.evaluator.Validate(contract.BoundRequest[[]byte]{Input: stored.Payload, Target: contract.Target{Marketplace: "shein"}, Action: action, RuleVersion: s.ruleVersion, BindingVersion: s.bindingVersion, ExpectedDigest: expected, ReadAt: stored.ReadAt, EvaluatedAt: time.Now().UTC(), Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated}})
	if ctx.Err() != nil {
		return contract.DiagnosticResult{}, ctx.Err()
	}
	if err != nil {
		return contract.DiagnosticResult{}, err
	}
	return result, nil
}
