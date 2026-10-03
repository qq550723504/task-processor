package einomodel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"task-processor/internal/aicapability"
	"task-processor/internal/integration/openai"
)

type OrganizationCredentialReader interface {
	GetCredential(context.Context, string, string, string) (*openai.AIClientCredential, error)
}

type RouteKey struct {
	OrganizationID string
	Operation      aicapability.Operation
}

// RoutePolicy is trusted deployment configuration for one organization and
// one capability. The admitted version is bound after restricted provisioning.
type RoutePolicy struct {
	Profile                        aicapability.ModelProfile
	AdmittedCredentialVersion      string
	AdmittedEndpointIdentityDigest string
}

type OrganizationRouteResolver struct {
	credentials OrganizationCredentialReader
	policies    map[RouteKey]RoutePolicy
}

// CredentialVersion is a non-secret row/version identity. The credential
// writer changes UpdatedAt on rotation; no key material enters the version.
func CredentialVersion(row openai.AIClientCredential) string {
	identity := fmt.Sprintf("v1|%d|%d|%s|%s|%s|%s|%d", row.ID, row.UpdatedAt.UTC().UnixNano(),
		row.ClientName, row.BaseURL, row.Model, row.APIStyle, row.TimeoutSecond)
	sum := sha256.Sum256([]byte(identity))
	return "org-credential:v1:" + hex.EncodeToString(sum[:])
}

func NewOrganizationRouteResolver(credentials OrganizationCredentialReader, policies map[RouteKey]RoutePolicy) (*OrganizationRouteResolver, error) {
	if credentials == nil || len(policies) == 0 || len(policies) > 128 {
		return nil, ErrInvalid
	}
	frozen := make(map[RouteKey]RoutePolicy, len(policies))
	for key, policy := range policies {
		if key.OrganizationID == "" || len(key.OrganizationID) > 128 ||
			(key.Operation != aicapability.OperationAIWorkbenchChatPlan && key.Operation != aicapability.OperationProductAgentDecision) ||
			policy.AdmittedCredentialVersion == "" || policy.AdmittedEndpointIdentityDigest == "" {
			return nil, ErrInvalid
		}
		policy.Profile.CredentialVersion = policy.AdmittedCredentialVersion
		policy.Profile.EndpointIdentityDigest = policy.AdmittedEndpointIdentityDigest
		if policy.Profile.Validate() != nil {
			return nil, ErrInvalid
		}
		frozen[key] = policy
	}
	return &OrganizationRouteResolver{credentials: credentials, policies: frozen}, nil
}

func (r *OrganizationRouteResolver) Resolve(ctx context.Context, input aicapability.TextInputIdentity) (QualifiedRoute, error) {
	if r == nil || r.credentials == nil || ctx == nil || ctx.Err() != nil {
		return QualifiedRoute{}, ErrNotDispatched
	}
	policy, ok := r.policies[RouteKey{OrganizationID: input.OrganizationID, Operation: input.Operation}]
	if !ok {
		return QualifiedRoute{}, ErrNotDispatched
	}
	// The exact organization row is selected. A member row, process default or
	// another capability's policy cannot be borrowed when this row is missing.
	row, err := r.credentials.GetCredential(ctx, input.OrganizationID, "", policy.Profile.ClientName)
	if err != nil || row == nil || !row.Enabled || row.UserID != "" || row.TenantID != input.OrganizationID ||
		row.ClientName != policy.Profile.ClientName || strings.TrimSpace(row.APIKey) == "" ||
		row.Model != policy.Profile.ModelID || CredentialVersion(*row) != policy.AdmittedCredentialVersion ||
		endpointDigest(row.BaseURL) != policy.AdmittedEndpointIdentityDigest ||
		row.TimeoutSecond < 1 || row.TimeoutSecond > 120 ||
		int64(row.TimeoutSecond)*int64(1e9) < int64(policy.Profile.DeadlineBound) {
		return QualifiedRoute{}, ErrNotDispatched
	}
	style := strings.ToLower(strings.TrimSpace(row.APIStyle))
	switch AdapterKind(policy.Profile.AdapterKind) {
	case AdapterOpenAICompatible:
		if style != "openai" && style != "openai-compatible" && style != "grsai" {
			return QualifiedRoute{}, ErrNotDispatched
		}
	case AdapterClaudeNative:
		if style != "claude-native" {
			return QualifiedRoute{}, ErrNotDispatched
		}
	default:
		return QualifiedRoute{}, ErrNotDispatched
	}
	route := QualifiedRoute{Profile: policy.Profile, Endpoint: row.BaseURL, APIKey: row.APIKey}
	if !sameRoute(route, route) {
		return QualifiedRoute{}, ErrNotDispatched
	}
	return route, nil
}

var _ interface {
	Resolve(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error)
} = (*OrganizationRouteResolver)(nil)
