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
