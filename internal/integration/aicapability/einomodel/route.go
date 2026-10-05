package einomodel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"task-processor/internal/aicapability"
	"task-processor/internal/integration/googleinteractions"
	"task-processor/internal/integration/openai"
)

type OrganizationCredentialReader interface {
	GetCredential(context.Context, string, string, string) (*openai.AIClientCredential, error)
}

type RouteKey struct {
	OrganizationID string
	Operation      aicapability.Operation
}

type RouteReadiness string

const (
	RouteAvailable          RouteReadiness = "AVAILABLE"
	RouteNeedsConfiguration RouteReadiness = "NEEDS_CONFIGURATION"
	RouteUnavailable        RouteReadiness = "UNAVAILABLE"
)

// RoutePolicy is trusted deployment configuration for one organization and
// one capability. The admitted version is bound after restricted provisioning.
type RoutePolicy struct {
	Profile                        aicapability.ModelProfile
	AdmittedCredentialVersion      string
	AdmittedEndpointIdentityDigest string
}

// ShapeProfile validates a deployment profile before its first credential
// write. An empty admitted version keeps execution unavailable until the
// operator binds the returned row version in deployment configuration.
func (p RoutePolicy) ShapeProfile() aicapability.ModelProfile {
	profile := p.Profile
	profile.CredentialVersion = p.AdmittedCredentialVersion
	if profile.CredentialVersion == "" {
		profile.CredentialVersion = "unadmitted"
	}
	profile.EndpointIdentityDigest = p.AdmittedEndpointIdentityDigest
	return profile
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

func EndpointIdentityDigest(endpoint string) string { return endpointDigest(endpoint) }

func NewOrganizationRouteResolver(credentials OrganizationCredentialReader, policies map[RouteKey]RoutePolicy) (*OrganizationRouteResolver, error) {
	if credentials == nil || len(policies) == 0 || len(policies) > 128 {
		return nil, ErrInvalid
	}
	frozen := make(map[RouteKey]RoutePolicy, len(policies))
	for key, policy := range policies {
		if key.OrganizationID == "" || len(key.OrganizationID) > 128 ||
			(key.Operation != aicapability.OperationAIWorkbenchChatPlan && key.Operation != aicapability.OperationProductAgentDecision) ||
			policy.AdmittedEndpointIdentityDigest == "" {
			return nil, ErrInvalid
		}
		if !ValidRouteProfile(policy.ShapeProfile(), key.Operation) {
			return nil, ErrInvalid
		}
		frozen[key] = policy
	}
	return &OrganizationRouteResolver{credentials: credentials, policies: frozen}, nil
}

func (r *OrganizationRouteResolver) Resolve(ctx context.Context, input aicapability.TextInputIdentity) (QualifiedRoute, error) {
	route, _, err := r.resolve(ctx, input)
	return route, err
}

// Readiness reads the same scoped policy and credential as Resolve. It is a
// current configuration hint, never execution authorization or provider health.
func (r *OrganizationRouteResolver) Readiness(ctx context.Context, input aicapability.TextInputIdentity) RouteReadiness {
	_, status, _ := r.resolve(ctx, input)
	return status
}

func (r *OrganizationRouteResolver) resolve(ctx context.Context, input aicapability.TextInputIdentity) (QualifiedRoute, RouteReadiness, error) {
	if r == nil || r.credentials == nil || ctx == nil || ctx.Err() != nil {
		return QualifiedRoute{}, RouteUnavailable, ErrNotDispatched
	}
	policy, ok := r.policies[RouteKey{OrganizationID: input.OrganizationID, Operation: input.Operation}]
	if !ok {
		return QualifiedRoute{}, RouteUnavailable, ErrNotDispatched
	}
	if policy.AdmittedCredentialVersion == "" {
		return QualifiedRoute{}, RouteNeedsConfiguration, ErrNotDispatched
	}
	policy.Profile = policy.ShapeProfile()
	// The exact organization row is selected. A member row, process default or
	// another capability's policy cannot be borrowed when this row is missing.
	row, err := r.credentials.GetCredential(ctx, input.OrganizationID, "", policy.Profile.ClientName)
	if err != nil {
		return QualifiedRoute{}, RouteUnavailable, ErrNotDispatched
	}
	if row == nil || !row.Enabled || row.UserID != "" || row.TenantID != input.OrganizationID ||
		row.ClientName != policy.Profile.ClientName || strings.TrimSpace(row.APIKey) == "" ||
		row.Model != policy.Profile.ModelID || CredentialVersion(*row) != policy.AdmittedCredentialVersion ||
		endpointDigest(row.BaseURL) != policy.AdmittedEndpointIdentityDigest ||
		row.TimeoutSecond < 1 || row.TimeoutSecond > 120 ||
		int64(row.TimeoutSecond)*int64(1e9) < int64(policy.Profile.DeadlineBound) {
		return QualifiedRoute{}, RouteNeedsConfiguration, ErrNotDispatched
	}
	style := strings.ToLower(strings.TrimSpace(row.APIStyle))
	switch AdapterKind(policy.Profile.AdapterKind) {
	case AdapterOpenAICompatible:
		if style != "openai" && style != "openai-compatible" && style != "grsai" {
			return QualifiedRoute{}, RouteNeedsConfiguration, ErrNotDispatched
		}
	case AdapterClaudeNative:
		if style != "claude-native" {
			return QualifiedRoute{}, RouteNeedsConfiguration, ErrNotDispatched
		}
	case AdapterGoogleInteractions:
		if style != "google-interactions" || !googleinteractions.ValidEndpoint(row.BaseURL) {
			return QualifiedRoute{}, RouteNeedsConfiguration, ErrNotDispatched
		}
	default:
		return QualifiedRoute{}, RouteUnavailable, ErrNotDispatched
	}
	route := QualifiedRoute{Profile: policy.Profile, Endpoint: row.BaseURL, APIKey: row.APIKey}
	if !sameRoute(route, route) {
		return QualifiedRoute{}, RouteUnavailable, ErrNotDispatched
	}
	return route, RouteAvailable, nil
}

// ValidRouteProfile is static protocol qualification, never user permission.
// Google is qualified only for the current title operation and fixed versions.
func ValidRouteProfile(p aicapability.ModelProfile, operation aicapability.Operation) bool {
	if p.Validate() != nil {
		return false
	}
	if AdapterKind(p.AdapterKind) != AdapterGoogleInteractions {
		return true
	}
	return operation == aicapability.OperationProductAgentDecision && p.ProviderID == "google" && p.ModelID == googleinteractions.Model &&
		p.AdapterPolicyVersion == googleinteractions.AdapterPolicyVersion && p.UsageMappingVersion == googleinteractions.UsageMappingVersion &&
		p.MaximumInputBytes <= 128<<10 && p.MaximumOutputBytes <= 256<<10
}

var _ interface {
	Resolve(context.Context, aicapability.TextInputIdentity) (QualifiedRoute, error)
} = (*OrganizationRouteResolver)(nil)
