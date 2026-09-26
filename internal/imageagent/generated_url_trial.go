package imageagent

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// IsolatedTrialGeneratedURLPolicy is a narrow exception for server-generated
// artifacts in the loopback-only local trial. It does not authorize source
// images, arbitrary provider URLs, or network downloads.
type IsolatedTrialGeneratedURLPolicy struct{ base string }

var trialBucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func NewIsolatedTrialGeneratedURLPolicy(base, bucket string) (*IsolatedTrialGeneratedURLPolicy, error) {
	if !trialBucketPattern.MatchString(bucket) || strings.Contains(bucket, "..") {
		return nil, ErrValidation
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Hostname() != "localhost" || parsed.Port() == "" || parsed.User != nil || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.ForceQuery || parsed.Opaque != "" || parsed.Path != "/image-agent-assets/"+bucket || parsed.String() != base {
		return nil, ErrValidation
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != parsed.Port() {
		return nil, ErrValidation
	}
	return &IsolatedTrialGeneratedURLPolicy{base: base}, nil
}

func (p *IsolatedTrialGeneratedURLPolicy) exactURL(key, raw string) error {
	if p == nil || key == "" || raw != p.base+"/"+key {
		return ErrValidation
	}
	return nil
}

// ValidateStaged admits only the staged identity for the exact recovered slot
// attempt and asset index. Callers must obtain ref from RecoverSlotArtifacts.
func (p *IsolatedTrialGeneratedURLPolicy) ValidateStaged(input SlotExecutionInput, ref StagedAssetRef, index int, raw string) error {
	if index < 0 || input.PlanRevision <= 0 || input.Attempt <= 0 || ValidateArtifactKeyIdentifier(input.TenantID) != nil || ValidateArtifactKeyIdentifier(input.RunID) != nil || ValidateArtifactKeyIdentifier(input.Slot.ID) != nil {
		return ErrValidation
	}
	ownerKey, err := ArtifactOwnerKey(input.UserID)
	if err != nil {
		return err
	}
	if _, err := NormalizeStagingManifest(StagingManifest{Assets: []StagedAssetRef{ref}}); err != nil {
		return err
	}
	extension, ok := publishedArtifactExtensions[ref.ContentType]
	if !ok {
		return ErrValidation
	}
	expected := fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/%d/%d-%s.%s", input.TenantID, ownerKey, input.RunID, input.PlanRevision, input.Slot.ID, input.Attempt, index, ref.SHA256, extension)
	if ref.ObjectKey != expected {
		return ErrValidation
	}
	return p.exactURL(ref.ObjectKey, raw)
}

// ValidatePublished retains the existing exact slot/key proof before applying
// the same fixed local URL rule to a published candidate.
func (p *IsolatedTrialGeneratedURLPolicy) ValidatePublished(input SlotExecutionInput, asset DurableAssetIdentity, index int, raw string) error {
	if err := ValidatePublishedAssetIdentityForSlot(input, asset, index); err != nil {
		return err
	}
	return p.exactURL(asset.ObjectKey, raw)
}

// ResolvePublishedAssetURL never trusts a URL from the run projection. The
// durable identity and configured resolver are checked before projection to a
// browser URL or commitment to Product Asset.
func ResolvePublishedAssetURL(input SlotExecutionInput, asset DurableAssetIdentity, index int, resolver DurableAssetPublicURLResolver, trial *IsolatedTrialGeneratedURLPolicy) (string, error) {
	if resolver == nil {
		return "", ErrValidation
	}
	if err := ValidatePublishedAssetIdentityForSlot(input, asset, index); err != nil {
		return "", err
	}
	url := resolver.PublicURL(asset.ObjectKey)
	if trial != nil {
		if err := trial.ValidatePublished(input, asset, index, url); err != nil {
			return "", err
		}
		return url, nil
	}
	return ValidateSafeImageURL(url)
}
