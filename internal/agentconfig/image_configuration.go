package agentconfig

import (
	"context"
	"encoding/hex"
	"time"

	"task-processor/internal/agent"
)

const ImageSnapshotContext = "product-image-set-v1"

type ImageRunLimits struct {
	Images         int   `json:"images"`
	Points         int64 `json:"points"`
	ElapsedSeconds int64 `json:"elapsedSeconds"`
}

func (l ImageRunLimits) Valid() bool {
	return l.Images > 0 && l.Images <= MaxSetTasks && l.Points > 0 && l.ElapsedSeconds > 0 && l.ElapsedSeconds <= 3600
}

func (l ImageRunLimits) Within(ceiling ImageRunLimits) bool {
	return l.Valid() && ceiling.Valid() && l.Images <= ceiling.Images && l.Points <= ceiling.Points && l.ElapsedSeconds <= ceiling.ElapsedSeconds
}

func ImageDigest(s string) bool {
	decoded, err := hex.DecodeString(s)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == s
}

// Image configuration deliberately carries no title Request or Knowledge.
// Parameters live in the exact immutable template revision; metadata is bounded.
type ImageStartCommand struct {
	Scope                                                  agent.Scope
	MemberID, RequestKey, ContextID, RunID, TargetPlatform string
	SourceDigest, InputDigest                              string
	Template                                               *TemplateRef
	HardLimits                                             ImageRunLimits
}

type ImageConfigurationSnapshot struct {
	ID, Digest                                             string
	Scope                                                  agent.Scope
	MemberID, RequestKey, ContextID, RunID, TargetPlatform string
	AgentVersion, Epoch, AgentRevision                     string
	SourceDigest, InputDigest, ParametersDigest            string
	Template                                               TemplateRef
	HardLimits                                             ImageRunLimits
	Parameters                                             SetTemplate `json:"-"`
}

func (s ImageConfigurationSnapshot) Ref() agent.ConfigurationSnapshotRef {
	return agent.ConfigurationSnapshotRef{Kind: SnapshotKind, ID: s.ID, Digest: s.Digest}
}

type ImageRunAdmissionCommand struct {
	Scope                                              agent.Scope
	Snapshot                                           agent.ConfigurationSnapshotRef
	MemberID, RunID, ConfirmActionID                   string
	SourceDigest, InputDigest, PlanDigest, QuoteDigest string
	Limits                                             ImageRunLimits
}

type ImageRunAdmissionReceipt struct {
	ID, Digest           string
	Command              ImageRunAdmissionCommand
	AdmittedAt, Deadline time.Time
}

type ImageConfigurationRepository interface {
	PrepareImageConfiguration(context.Context, ImageStartCommand) (ImageConfigurationSnapshot, error)
	LoadImageConfiguration(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (ImageConfigurationSnapshot, error)
	AdmitImageRun(context.Context, ImageRunAdmissionCommand, ImageRunLimits) (ImageRunAdmissionReceipt, error)
	ReadImageRunAdmission(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (ImageRunAdmissionReceipt, error)
}
