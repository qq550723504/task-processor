package listingkit

type GenerationReviewNavigationDelta struct {
	PlatformChanged   bool `json:"platform_changed,omitempty"`
	SlotChanged       bool `json:"slot_changed,omitempty"`
	CapabilityChanged bool `json:"capability_changed,omitempty"`
	SectionChanged    bool `json:"section_changed,omitempty"`
}

type GenerationReviewNavigationTarget struct {
	DispatchKind          string                          `json:"dispatch_kind,omitempty"`
	Conditional           *GenerationConditionalState     `json:"conditional,omitempty"`
	ResourceKind          string                          `json:"resource_kind,omitempty"`
	CacheKey              string                          `json:"cache_key,omitempty"`
	CachePolicy           string                          `json:"cache_policy,omitempty"`
	RevalidateAfterAction bool                            `json:"revalidate_after_action,omitempty"`
	Descriptor            *GenerationNavigationDescriptor `json:"descriptor,omitempty"`
	QueueQuery            *GenerationQueueQuery           `json:"queue_query,omitempty"`
	SessionQuery          *GenerationQueueQuery           `json:"session_query,omitempty"`
	PreviewQuery          *GenerationQueueQuery           `json:"preview_query,omitempty"`
	ActionTarget          *AssetGenerationActionTarget    `json:"action_target,omitempty"`
}

type GenerationNavigationDescriptor struct {
	ResourceKind                 string                             `json:"resource_kind,omitempty"`
	CacheKey                     string                             `json:"cache_key,omitempty"`
	CachePolicy                  string                             `json:"cache_policy,omitempty"`
	SupportsStaleWhileRevalidate bool                               `json:"supports_stale_while_revalidate,omitempty"`
	RevalidateAfterAction        bool                               `json:"revalidate_after_action,omitempty"`
	RefreshScope                 string                             `json:"refresh_scope,omitempty"`
	Invalidates                  []string                           `json:"invalidates,omitempty"`
	DispatchPlan                 *GenerationNavigationDispatchPlan  `json:"dispatch_plan,omitempty"`
	FollowUpReads                []GenerationNavigationFollowUpRead `json:"followup_reads,omitempty"`
	Conditional                  *GenerationConditionalState        `json:"conditional,omitempty"`
}

type GenerationNavigationDispatchPlan struct {
	Strategy           string                             `json:"strategy,omitempty"`
	StopOnNotModified  bool                               `json:"stop_on_not_modified,omitempty"`
	StopOnFirstSuccess bool                               `json:"stop_on_first_success,omitempty"`
	StopOnError        bool                               `json:"stop_on_error,omitempty"`
	FallbackStrategy   string                             `json:"fallback_strategy,omitempty"`
	MaxParallelism     int                                `json:"max_parallelism,omitempty"`
	DedupePolicy       string                             `json:"dedupe_policy,omitempty"`
	WinnerPolicy       string                             `json:"winner_policy,omitempty"`
	RequiresRevalidate bool                               `json:"requires_revalidate,omitempty"`
	Steps              []GenerationNavigationDispatchStep `json:"steps,omitempty"`
}

type GenerationNavigationDispatchStep struct {
	Kind               string                `json:"kind,omitempty"`
	ResponseMode       string                `json:"response_mode,omitempty"`
	CachePreference    string                `json:"cache_preference,omitempty"`
	RequiresRevalidate bool                  `json:"requires_revalidate,omitempty"`
	Query              *GenerationQueueQuery `json:"query,omitempty"`
}

type GenerationNavigationFollowUpRead struct {
	Kind         string                `json:"kind,omitempty"`
	ResponseMode string                `json:"response_mode,omitempty"`
	Query        *GenerationQueueQuery `json:"query,omitempty"`
}

type GenerationReviewNavigationDispatchRequest struct {
	ResponseMode string                            `json:"response_mode,omitempty"`
	PlanMode     string                            `json:"plan_mode,omitempty"`
	Target       *GenerationReviewNavigationTarget `json:"target,omitempty"`
}

type GenerationReviewNavigationDispatchResponse struct {
	TaskID                         string                                  `json:"task_id,omitempty"`
	DispatchKind                   string                                  `json:"dispatch_kind,omitempty"`
	ResponseMode                   string                                  `json:"response_mode,omitempty"`
	PlanMode                       string                                  `json:"plan_mode,omitempty"`
	DeltaToken                     string                                  `json:"delta_token,omitempty"`
	NotModified                    bool                                    `json:"not_modified,omitempty"`
	Conditional                    *GenerationConditionalState             `json:"conditional,omitempty"`
	ResourceDescriptors            []GenerationPanelResourceDescriptor     `json:"resource_descriptors,omitempty"`
	PrimaryRecoveryDescriptor      *GenerationPanelResourceDescriptor      `json:"primary_recovery_descriptor,omitempty"`
	RecommendedRecoveryDescriptors []GenerationPanelResourceDescriptor     `json:"recommended_recovery_descriptors,omitempty"`
	ResolvedActionSummary          *GenerationResolvedActionSummary        `json:"resolved_action_summary,omitempty"`
	ExecutedPlan                   *GenerationNavigationDispatchExecution  `json:"executed_plan,omitempty"`
	FocusedSourceKind              string                                  `json:"focused_source_kind,omitempty"`
	FocusedSourceStep              int                                     `json:"focused_source_step,omitempty"`
	FocusedViaFallback             bool                                    `json:"focused_via_fallback,omitempty"`
	FocusedFallbackReason          string                                  `json:"focused_fallback_reason,omitempty"`
	FocusedResolution              *GenerationNavigationDispatchResolution `json:"focused_resolution,omitempty"`
	Queue                          *GenerationQueuePage                    `json:"queue,omitempty"`
	ReviewSession                  *GenerationReviewSessionResponse        `json:"review_session,omitempty"`
	ReviewPreview                  *GenerationReviewPreviewResponse        `json:"review_preview,omitempty"`
	Action                         *GenerationActionExecutionResult        `json:"action,omitempty"`
	PanelUpdate                    *GenerationReviewPanelUpdate            `json:"panel_update,omitempty"`
}

type GenerationNavigationDispatchExecution struct {
	Strategy       string                                      `json:"strategy,omitempty"`
	StopReason     string                                      `json:"stop_reason,omitempty"`
	Partial        bool                                        `json:"partial,omitempty"`
	CompletedSteps int                                         `json:"completed_steps,omitempty"`
	FailedSteps    int                                         `json:"failed_steps,omitempty"`
	DedupedSteps   int                                         `json:"deduped_steps,omitempty"`
	Steps          []GenerationNavigationDispatchExecutionStep `json:"steps,omitempty"`
}

type GenerationNavigationDispatchExecutionStep struct {
	Kind               string                           `json:"kind,omitempty"`
	ResponseMode       string                           `json:"response_mode,omitempty"`
	CachePreference    string                           `json:"cache_preference,omitempty"`
	RequiresRevalidate bool                             `json:"requires_revalidate,omitempty"`
	Status             string                           `json:"status,omitempty"`
	Error              string                           `json:"error,omitempty"`
	ErrorKind          string                           `json:"error_kind,omitempty"`
	Retryable          bool                             `json:"retryable,omitempty"`
	RetryHint          string                           `json:"retry_hint,omitempty"`
	Winner             bool                             `json:"winner,omitempty"`
	FallbackApplied    bool                             `json:"fallback_applied,omitempty"`
	FallbackReason     string                           `json:"fallback_reason,omitempty"`
	FallbackCandidate  bool                             `json:"fallback_candidate,omitempty"`
	FallbackSourceKind string                           `json:"fallback_source_kind,omitempty"`
	DeduplicationKey   string                           `json:"deduplication_key,omitempty"`
	DeduplicatedFrom   int                              `json:"deduplicated_from,omitempty"`
	Executed           bool                             `json:"executed,omitempty"`
	Skipped            bool                             `json:"skipped,omitempty"`
	DeltaToken         string                           `json:"delta_token,omitempty"`
	NotModified        bool                             `json:"not_modified,omitempty"`
	NoChanges          bool                             `json:"no_changes,omitempty"`
	Queue              *GenerationQueuePage             `json:"queue,omitempty"`
	ReviewSession      *GenerationReviewSessionResponse `json:"review_session,omitempty"`
	ReviewPreview      *GenerationReviewPreviewResponse `json:"review_preview,omitempty"`
}

type GenerationReviewPanelUpdate struct {
	DispatchKind                   string                                  `json:"dispatch_kind,omitempty"`
	ResponseMode                   string                                  `json:"response_mode,omitempty"`
	DeltaToken                     string                                  `json:"delta_token,omitempty"`
	NoChanges                      bool                                    `json:"no_changes,omitempty"`
	Conditional                    *GenerationConditionalState             `json:"conditional,omitempty"`
	FocusedSourceKind              string                                  `json:"focused_source_kind,omitempty"`
	FocusedSourceStep              int                                     `json:"focused_source_step,omitempty"`
	FocusedViaFallback             bool                                    `json:"focused_via_fallback,omitempty"`
	FocusedFallbackReason          string                                  `json:"focused_fallback_reason,omitempty"`
	FocusedResolution              *GenerationNavigationDispatchResolution `json:"focused_resolution,omitempty"`
	FocusedDescriptors             []GenerationPanelResourceDescriptor     `json:"focused_descriptors,omitempty"`
	ChangedDescriptors             []GenerationPanelResourceDescriptor     `json:"changed_descriptors,omitempty"`
	PrimaryRecoveryDescriptor      *GenerationPanelResourceDescriptor      `json:"primary_recovery_descriptor,omitempty"`
	RecommendedRecoveryDescriptors []GenerationPanelResourceDescriptor     `json:"recommended_recovery_descriptors,omitempty"`
	Overview                       *AssetGenerationOverview                `json:"overview,omitempty"`
	QueueSummary                   *GenerationWorkQueueSummary             `json:"queue_summary,omitempty"`
	ReviewSummary                  *GenerationReviewSummary                `json:"review_summary,omitempty"`
	FocusedTarget                  *GenerationReviewTarget                 `json:"focused_target,omitempty"`
	FocusedRenderPreview           *AssetRenderPreviewSlot                 `json:"focused_render_preview,omitempty"`
	FocusedToolbar                 *GenerationReviewToolbarInput           `json:"focused_toolbar,omitempty"`
	ReviewPatch                    *GenerationReviewSessionPatch           `json:"review_patch,omitempty"`
	ReviewSession                  *GenerationReviewSessionResponse        `json:"review_session,omitempty"`
	ReviewPreview                  *GenerationReviewPreviewResponse        `json:"review_preview,omitempty"`
	Action                         *GenerationActionExecutionResult        `json:"action,omitempty"`
}

type GenerationPanelResourceDescriptor struct {
	Role                 string                            `json:"role,omitempty"`
	Platform             string                            `json:"platform,omitempty"`
	Slot                 string                            `json:"slot,omitempty"`
	Capability           string                            `json:"capability,omitempty"`
	SectionKey           string                            `json:"section_key,omitempty"`
	SourceKind           string                            `json:"source_kind,omitempty"`
	SourceStep           int                               `json:"source_step,omitempty"`
	ViaFallback          bool                              `json:"via_fallback,omitempty"`
	FallbackReason       string                            `json:"fallback_reason,omitempty"`
	RecoveryScope        string                            `json:"recovery_scope,omitempty"`
	RecoveryHint         string                            `json:"recovery_hint,omitempty"`
	Retryable            bool                              `json:"retryable,omitempty"`
	RecoverySeverity     string                            `json:"recovery_severity,omitempty"`
	RecoveryUrgency      string                            `json:"recovery_urgency,omitempty"`
	RecoveryCTAKind      string                            `json:"recovery_cta_kind,omitempty"`
	RecoveryActionKey    string                            `json:"recovery_action_key,omitempty"`
	RecoveryTarget       *GenerationReviewNavigationTarget `json:"recovery_target,omitempty"`
	RecoveryDispatchPlan *GenerationNavigationDispatchPlan `json:"recovery_dispatch_plan,omitempty"`
	Descriptor           *GenerationNavigationDescriptor   `json:"descriptor,omitempty"`
}

type GenerationNavigationDispatchResolution struct {
	SourceKind     string `json:"source_kind,omitempty"`
	SourceStep     int    `json:"source_step,omitempty"`
	ViaFallback    bool   `json:"via_fallback,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`
}
