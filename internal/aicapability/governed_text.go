package aicapability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrTextProfile = errors.New("invalid governed text profile")
var ErrTextEnvelope = errors.New("invalid governed text envelope")

// ModelProfile is a non-secret, immutable use-time snapshot of the approved
// organization route and all limits that affect dispatch or settlement.
type ModelProfile struct {
	ClientName              string
	ProviderID              string
	AdapterKind             string
	ModelID                 string
	EndpointIdentityDigest  string
	CredentialVersion       string
	RoutingPolicyVersion    string
	AdapterPolicyVersion    string
	PromptVersion           string
	OutputSchemaVersion     string
	UsageMappingVersion     string
	CostPricingVersion      string
	PointTariff             ModelPointTariff
	Currency                string
	InputMicrosPerMillion   int64
	OutputMicrosPerMillion  int64
	MaximumPromptTokens     int64
	MaximumCompletionTokens int64
	MaximumInputBytes       int
	MaximumOutputBytes      int
	DeadlineBound           time.Duration
}

func validTextFact(value string, max int) bool {
	if value == "" || len(value) > max || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (p ModelProfile) Validate() error {
	for _, value := range []string{
		p.ClientName, p.ProviderID, p.ModelID, p.EndpointIdentityDigest,
		p.CredentialVersion, p.RoutingPolicyVersion, p.AdapterPolicyVersion,
		p.PromptVersion, p.OutputSchemaVersion, p.UsageMappingVersion, p.CostPricingVersion,
	} {
		if !validTextFact(value, 128) {
			return ErrTextProfile
		}
	}
	if p.AdapterKind != "openai-compatible" && p.AdapterKind != "claude-native" {
		return ErrTextProfile
	}
	if len(p.Currency) != 3 || !p.PointTariff.Valid() ||
		p.InputMicrosPerMillion <= 0 || p.InputMicrosPerMillion > 1e12 ||
		p.OutputMicrosPerMillion <= 0 || p.OutputMicrosPerMillion > 1e12 ||
		p.MaximumPromptTokens < 1 || p.MaximumPromptTokens > 1048576 ||
		p.MaximumCompletionTokens < 1 || p.MaximumCompletionTokens > 65536 ||
		p.MaximumInputBytes < 1 || p.MaximumInputBytes > 1<<20 ||
		p.MaximumOutputBytes < 1 || p.MaximumOutputBytes > 1<<20 ||
		p.DeadlineBound <= 0 || p.DeadlineBound > 2*time.Minute {
		return ErrTextProfile
	}
	if _, err := p.PointTariff.Points(p.MaximumPromptTokens, p.MaximumCompletionTokens); err != nil {
		return ErrTextProfile
	}
	if _, err := p.MaximumCost(); err != nil {
		return ErrTextProfile
	}
	return nil
}

func (p ModelProfile) Digest() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", ErrTextProfile
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (p ModelProfile) MaximumCost() (int64, error) {
	return p.CostFor(p.MaximumPromptTokens, p.MaximumCompletionTokens)
}

func (p ModelProfile) CostFor(promptTokens, completionTokens int64) (int64, error) {
	if promptTokens < 0 || completionTokens < 0 || promptTokens > p.MaximumPromptTokens ||
		completionTokens > p.MaximumCompletionTokens ||
		p.InputMicrosPerMillion <= 0 || p.OutputMicrosPerMillion <= 0 {
		return 0, ErrTextProfile
	}
	cost := new(big.Int).Mul(big.NewInt(promptTokens), big.NewInt(p.InputMicrosPerMillion))
	cost.Add(cost, new(big.Int).Mul(big.NewInt(completionTokens), big.NewInt(p.OutputMicrosPerMillion)))
	cost.Add(cost, big.NewInt(999999))
	cost.Div(cost, big.NewInt(1000000))
	if !cost.IsInt64() {
		return 0, ErrTextProfile
	}
	return cost.Int64(), nil
}

type TextInputIdentity struct {
	OrganizationID string
	ActorID        string
	MemberID       string
	Operation      Operation
	InvocationID   string
	AgentRunID     string
	BusinessTaskID string
	System         string
	Prompt         string
	Profile        ModelProfile
}

type TextQuote struct {
	InputHash         string
	ProfileDigest     string
	MaximumTokens     int64
	MaximumCostMicros int64
	Currency          string
}

// QuoteText is side-effect-free. A byte-level upper bound includes a fixed
// allowance for SDK framing; the transport checks the final serialized wire.
func QuoteText(input TextInputIdentity) (TextQuote, error) {
	if err := input.Profile.Validate(); err != nil {
		return TextQuote{}, err
	}
	if !validTextFact(input.OrganizationID, 128) ||
		!validTextFact(input.ActorID, 128) || !validTextFact(input.MemberID, 128) ||
		(input.Operation != OperationAIWorkbenchChatPlan && input.Operation != OperationProductAgentDecision) ||
		input.System == "" || input.Prompt == "" ||
		!utf8.ValidString(input.System) || !utf8.ValidString(input.Prompt) {
		return TextQuote{}, ErrTextEnvelope
	}
	if input.Operation == OperationAIWorkbenchChatPlan {
		if input.AgentRunID != "" || input.BusinessTaskID != "" {
			return TextQuote{}, ErrTextEnvelope
		}
	} else if !validTextFact(input.AgentRunID, 128) {
		return TextQuote{}, ErrTextEnvelope
	}
	profileDigest, _ := input.Profile.Digest()
	envelope := struct {
		OrganizationID string
		ActorID        string
		MemberID       string
		Operation      Operation
		AgentRunID     string
		BusinessTaskID string
		ProfileDigest  string
		System         string
		Prompt         string
	}{input.OrganizationID, input.ActorID, input.MemberID, input.Operation,
		input.AgentRunID, input.BusinessTaskID, profileDigest, input.System, input.Prompt}
	wire, err := json.Marshal(envelope)
	if err != nil || len(wire)+1024 > input.Profile.MaximumInputBytes ||
		int64(len(wire)+1024) > input.Profile.MaximumPromptTokens {
		return TextQuote{}, ErrTextEnvelope
	}
	sum := sha256.Sum256(wire)
	promptBound := int64(len(wire) + 1024)
	cost, err := input.Profile.CostFor(promptBound, input.Profile.MaximumCompletionTokens)
	if err != nil {
		return TextQuote{}, err
	}
	return TextQuote{InputHash: hex.EncodeToString(sum[:]), ProfileDigest: profileDigest,
		MaximumTokens:     promptBound + input.Profile.MaximumCompletionTokens,
		MaximumCostMicros: cost, Currency: input.Profile.Currency}, nil
}
