package currentapplication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/agent"
	coreconfig "task-processor/internal/core/config"
	governed "task-processor/internal/integration/aicapability/einomodel"
	"task-processor/internal/integration/openai"
)

// TitleCredentialProvision is a private operator input, never an HTTP or Agent
// request. WriterDatabase is a separate, restricted connection to ProductAgentDB.
type TitleCredentialProvision struct {
	Action         string         `json:"action"`
	Consumer       string         `json:"consumer"`
	OrganizationID string         `json:"organizationId"`
	ClientName     string         `json:"clientName"`
	APIKey         string         `json:"apiKey,omitempty"`
	BaseURL        string         `json:"baseURL,omitempty"`
	Model          string         `json:"model,omitempty"`
	APIStyle       string         `json:"apiStyle,omitempty"`
	TimeoutSecond  int            `json:"timeoutSecond,omitempty"`
	WriterDatabase DatabaseConfig `json:"writerDatabase"`
}

type TitleCredentialProvisionResult struct {
	OrganizationID         string `json:"organizationId"`
	ProviderID             string `json:"providerId,omitempty"`
	Enabled                bool   `json:"enabled"`
	CredentialVersion      string `json:"credentialVersion,omitempty"`
	EndpointIdentityDigest string `json:"endpointIdentityDigest,omitempty"`
}

func LoadTitleCredentialProvision(path string) (TitleCredentialProvision, error) {
	var input TitleCredentialProvision
	if !filepath.IsAbs(path) {
		return input, errors.New("title credential input path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return input, errors.New("private title credential input must be a regular file")
	}
	if runtime.GOOS == "windows" {
		if coreconfig.VerifyPrivateFiles(context.Background(), []string{path}) != nil {
			return input, errors.New("title credential input must be private")
		}
	} else if info.Mode().Perm()&0o077 != 0 {
		return input, errors.New("title credential input must be private")
	}
	file, err := os.Open(path)
	if err != nil {
		return input, errors.New("open title credential input failed")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (16<<10)+1))
	if err != nil || len(data) > 16<<10 || validateJSONShape(data) != nil {
		return input, errors.New("title credential input is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || ensureJSONEOF(decoder) != nil {
		return TitleCredentialProvision{}, errors.New("title credential input is invalid")
	}
	return input, nil
}

// ValidateTitleCredentialProvision checks the target before any database open.
func ValidateTitleCredentialProvision(cfg *Config, input TitleCredentialProvision) error {
	if cfg == nil || cfg.ProductAgent == nil || !agent.ValidID(input.OrganizationID) || !agent.ValidID(input.ClientName) {
		return errors.New("title credential target is invalid")
	}
	if err := input.WriterDatabase.validate("title credential writer database"); err != nil {
		return errors.New("title credential writer database is invalid")
	}
	target := cfg.ProductAgent.Database
	if input.WriterDatabase.Host != target.Host || input.WriterDatabase.Port != target.Port || input.WriterDatabase.Database != target.Database || input.WriterDatabase.User == target.User {
		return errors.New("title credential writer must be separate and target ProductAgentDB")
	}
	allowed := false
	for _, id := range cfg.ProductAgent.AllowedOrganizationIDs {
		allowed = allowed || id == input.OrganizationID
	}
	if !allowed {
		return errors.New("title credential organization is not admitted")
	}
	var policy governed.RoutePolicy
	var found bool
	switch input.Consumer {
	case "title":
		policy, found = cfg.ProductAgent.TextPolicies[input.OrganizationID]
	case "planning":
		if cfg.AIWorkbench != nil {
			policy, found = cfg.AIWorkbench.PlanningTextPolicies[input.OrganizationID]
		}
	}
	if !found || policy.Profile.ClientName != input.ClientName {
		return errors.New("title credential client is not admitted")
	}
	if input.Action == "disable" {
		if input.APIKey != "" || input.BaseURL != "" || input.Model != "" || input.APIStyle != "" || input.TimeoutSecond != 0 {
			return errors.New("disable input must not include provider values")
		}
		return nil
	}
	if input.Action != "upsert" {
		return errors.New("title credential action is unsupported")
	}
	endpointDigest := governed.EndpointIdentityDigest(input.BaseURL)
	if !governed.ValidEndpoint(input.BaseURL) ||
		(policy.AdmittedEndpointIdentityDigest != "" && endpointDigest != policy.AdmittedEndpointIdentityDigest) ||
		!validProvisionPolicy(cfg, input.Consumer, policy, endpointDigest) || input.Model != policy.Profile.ModelID ||
		(input.APIStyle != policy.Profile.AdapterKind && !(policy.Profile.AdapterKind == "openai-compatible" && (input.APIStyle == "openai" || input.APIStyle == "grsai"))) ||
		strings.TrimSpace(input.APIKey) == "" || len(input.APIKey) > 4096 || input.TimeoutSecond < 1 || input.TimeoutSecond > 120 ||
		time.Duration(input.TimeoutSecond)*time.Second < policy.Profile.DeadlineBound {
		return errors.New("title credential does not match the operator admission profile")
	}
	return nil
}

func validProvisionPolicy(cfg *Config, consumer string, policy governed.RoutePolicy, endpointDigest string) bool {
	profile := policy.ShapeProfile()
	profile.EndpointIdentityDigest = endpointDigest
	if profile.Validate() != nil || profile.Currency != cfg.ProductAgent.Currency ||
		profile.MaximumPromptTokens+profile.MaximumCompletionTokens > cfg.ProductAgent.Tokens {
		return false
	}
	maximumCost, err := profile.MaximumCost()
	if err != nil || maximumCost > cfg.ProductAgent.CostMicros {
		return false
	}
	switch consumer {
	case "title":
		return profile.PromptVersion == "product-title-agent-v1" && profile.OutputSchemaVersion == "product-title-action-v1"
	case "planning":
		return profile.PromptVersion == "ai-workbench-chat-plan-v1" && profile.OutputSchemaVersion == "ai-workbench-plan-decision-v1" &&
			profile.MaximumCompletionTokens <= 4096 && profile.MaximumInputBytes <= 128<<10 && profile.MaximumOutputBytes <= 16<<10
	default:
		return false
	}
}

// SaveTitleCredential uses the credential owner's existing write method. The
// serving role is never used and the DB session suppresses secret-bearing SQL.
func SaveTitleCredential(ctx context.Context, cfg *Config, input TitleCredentialProvision, db *gorm.DB) (TitleCredentialProvisionResult, error) {
	var result TitleCredentialProvisionResult
	if ctx == nil || ctx.Err() != nil || db == nil || ValidateTitleCredentialProvision(cfg, input) != nil {
		return result, errors.New("title credential provision is unavailable")
	}
	safeDB := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	err := safeDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := openai.NewGormCredentialResolver(tx)
		credential := openai.AIClientCredential{TenantID: input.OrganizationID, ClientName: input.ClientName, Enabled: input.Action == "upsert"}
		if input.Action == "disable" {
			previous, err := store.GetCredential(ctx, input.OrganizationID, "", input.ClientName)
			if err != nil || previous == nil {
				return errors.New("organization credential unavailable")
			}
			credential = *previous
			credential.Enabled = false
		} else {
			credential.APIKey, credential.BaseURL, credential.Model, credential.APIStyle, credential.TimeoutSecond = input.APIKey, input.BaseURL, input.Model, input.APIStyle, input.TimeoutSecond
		}
		credential.UserID = ""
		if err := store.SaveCredential(ctx, credential); err != nil {
			return err
		}
		result.OrganizationID, result.Enabled = input.OrganizationID, input.Action == "upsert"
		if !result.Enabled {
			return nil
		}
		stored, err := store.GetCredential(ctx, input.OrganizationID, "", input.ClientName)
		if err != nil || stored == nil || !stored.Enabled || stored.UserID != "" || stored.TenantID != input.OrganizationID ||
			stored.BaseURL != input.BaseURL || stored.Model != input.Model || stored.APIStyle != input.APIStyle {
			return errors.New("title route resolution failed")
		}
		if input.Consumer == "planning" {
			result.ProviderID = cfg.AIWorkbench.PlanningTextPolicies[input.OrganizationID].Profile.ProviderID
		} else {
			result.ProviderID = cfg.ProductAgent.TextPolicies[input.OrganizationID].Profile.ProviderID
		}
		result.CredentialVersion = governed.CredentialVersion(*stored)
		result.EndpointIdentityDigest = governed.EndpointIdentityDigest(stored.BaseURL)
		return nil
	})
	if err != nil {
		return TitleCredentialProvisionResult{}, errors.New("title credential transaction failed")
	}
	return result, nil
}
