package listingkit

import "testing"

func TestResolveLayerTemporalPlatformPrefersTargetQueueQuery(t *testing.T) {
	t.Parallel()

	req := &ExecuteGenerationActionRequest{
		Target: &AssetGenerationActionTarget{
			QueueQuery: &GenerationQueueQuery{Platform: "  AMAzon  "},
			NavigationTarget: &GenerationReviewNavigationTarget{
				QueueQuery:   &GenerationQueueQuery{Platform: "tiktok"},
				SessionQuery: &GenerationQueueQuery{Platform: "etsy"},
				PreviewQuery: &GenerationQueueQuery{Platform: "shopify"},
			},
		},
	}

	if got := resolveLayerTemporalPlatform(req); got != "amazon" {
		t.Fatalf("resolveLayerTemporalPlatform() = %q, want %q", got, "amazon")
	}
}

func TestResolveLayerTemporalPlatformTraversesNavigationQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *ExecuteGenerationActionRequest
		want string
	}{
		{
			name: "navigation queue query when root queue missing",
			req: &ExecuteGenerationActionRequest{
				Target: &AssetGenerationActionTarget{
					NavigationTarget: &GenerationReviewNavigationTarget{
						QueueQuery:   &GenerationQueueQuery{Platform: "  TIKTOK  "},
						SessionQuery: &GenerationQueueQuery{Platform: "etsy"},
						PreviewQuery: &GenerationQueueQuery{Platform: "shopify"},
					},
				},
			},
			want: "tiktok",
		},
		{
			name: "navigation session query when queue query missing",
			req: &ExecuteGenerationActionRequest{
				Target: &AssetGenerationActionTarget{
					NavigationTarget: &GenerationReviewNavigationTarget{
						SessionQuery: &GenerationQueueQuery{Platform: "  ETSY  "},
						PreviewQuery: &GenerationQueueQuery{Platform: "shopify"},
					},
				},
			},
			want: "etsy",
		},
		{
			name: "navigation preview query when earlier shapes missing",
			req: &ExecuteGenerationActionRequest{
				Target: &AssetGenerationActionTarget{
					NavigationTarget: &GenerationReviewNavigationTarget{
						PreviewQuery: &GenerationQueueQuery{Platform: "  SHOPIFY  "},
					},
				},
			},
			want: "shopify",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveLayerTemporalPlatform(tt.req); got != tt.want {
				t.Fatalf("resolveLayerTemporalPlatform() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveLayerTemporalPlatformTraversesFollowUpReadsAndNestedActionTarget(t *testing.T) {
	t.Parallel()

	t.Run("descriptor followup reads when earlier shapes missing", func(t *testing.T) {
		t.Parallel()

		req := &ExecuteGenerationActionRequest{
			Target: &AssetGenerationActionTarget{
				NavigationTarget: &GenerationReviewNavigationTarget{
					Descriptor: &GenerationNavigationDescriptor{
						FollowUpReads: []GenerationNavigationFollowUpRead{
							{Query: &GenerationQueueQuery{Platform: "  WALMART  "}},
							{Query: &GenerationQueueQuery{Platform: "target"}},
						},
					},
				},
			},
		}

		if got := resolveLayerTemporalPlatform(req); got != "walmart" {
			t.Fatalf("resolveLayerTemporalPlatform() = %q, want %q", got, "walmart")
		}
	})

	t.Run("nested action target traversal when earlier shapes missing", func(t *testing.T) {
		t.Parallel()

		req := &ExecuteGenerationActionRequest{
			Target: &AssetGenerationActionTarget{
				NavigationTarget: &GenerationReviewNavigationTarget{
					ActionTarget: &AssetGenerationActionTarget{
						NavigationTarget: &GenerationReviewNavigationTarget{
							SessionQuery: &GenerationQueueQuery{Platform: "  TEMU  "},
						},
					},
				},
			},
		}

		if got := resolveLayerTemporalPlatform(req); got != "temu" {
			t.Fatalf("resolveLayerTemporalPlatform() = %q, want %q", got, "temu")
		}
	})

	t.Run("followup reads win before recursive nested action target", func(t *testing.T) {
		t.Parallel()

		req := &ExecuteGenerationActionRequest{
			Target: &AssetGenerationActionTarget{
				NavigationTarget: &GenerationReviewNavigationTarget{
					Descriptor: &GenerationNavigationDescriptor{
						FollowUpReads: []GenerationNavigationFollowUpRead{
							{Query: &GenerationQueueQuery{Platform: "  WALMART  "}},
						},
					},
					ActionTarget: &AssetGenerationActionTarget{
						NavigationTarget: &GenerationReviewNavigationTarget{
							SessionQuery: &GenerationQueueQuery{Platform: "  TEMU  "},
						},
					},
				},
			},
		}

		if got := resolveLayerTemporalPlatform(req); got != "walmart" {
			t.Fatalf("resolveLayerTemporalPlatform() = %q, want %q", got, "walmart")
		}
	})
}

func TestResolveLayerTemporalPlatformDefaultsToShein(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *ExecuteGenerationActionRequest
	}{
		{name: "nil request"},
		{name: "empty request", req: &ExecuteGenerationActionRequest{}},
		{name: "empty target", req: &ExecuteGenerationActionRequest{Target: &AssetGenerationActionTarget{}}},
		{
			name: "blank platform values",
			req: &ExecuteGenerationActionRequest{
				Target: &AssetGenerationActionTarget{
					QueueQuery: &GenerationQueueQuery{Platform: "   "},
					NavigationTarget: &GenerationReviewNavigationTarget{
						QueueQuery:   &GenerationQueueQuery{Platform: " "},
						SessionQuery: &GenerationQueueQuery{Platform: "\t"},
						PreviewQuery: &GenerationQueueQuery{Platform: "\n"},
						Descriptor: &GenerationNavigationDescriptor{
							FollowUpReads: []GenerationNavigationFollowUpRead{
								{Query: &GenerationQueueQuery{Platform: " "}},
							},
						},
						ActionTarget: &AssetGenerationActionTarget{
							QueueQuery: &GenerationQueueQuery{Platform: " "},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveLayerTemporalPlatform(tt.req); got != "shein" {
				t.Fatalf("resolveLayerTemporalPlatform() = %q, want %q", got, "shein")
			}
		})
	}
}

func TestTaskGenerationLayerTemporalRequestBoundary(t *testing.T) {
	t.Parallel()

	t.Run("request_parsing_home_owns_traversal", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_temporal_request_platform.go", "resolveTemporalRequestPlatform")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_temporal_request_platform.go", "resolveTemporalRequestPlatform")

		assertSourceContainsAll(t, source, []string{
			"QueueQuery",
			"SessionQuery",
			"PreviewQuery",
			"FollowUpReads",
			"ActionTarget",
			"&ExecuteGenerationActionRequest{",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"normalizeTemporalRequestPlatform",
			"resolveTemporalRequestPlatform",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"resolveLayerTemporalPlatform",
		})
	})

	t.Run("platform_temporal_phase_consumes_local_parsing_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_temporal_platform.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_temporal_platform.go", "run")

		assertSourceContainsAll(t, source, []string{
			"resolveTemporalRequestPlatform(req)",
			"&GenerationQueueQuery{Platform: platform}",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"resolveTemporalRequestPlatform",
			"StartPlatformAdaptation",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"resolveLayerTemporalPlatform",
			"normalizeTemporalRequestPlatform",
		})
		assertSourceExcludesAll(t, source, []string{
			"FollowUpReads",
			"SessionQuery",
			"PreviewQuery",
		})
	})

	t.Run("service_generation_actions_keeps_only_seam_alias", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "service_generation_actions.go", "resolveLayerTemporalPlatform")
		callNames := readNamedFunctionCallNames(t, "service_generation_actions.go", "resolveLayerTemporalPlatform")

		assertSourceContainsAll(t, source, []string{
			"resolveTemporalRequestPlatform(req)",
		})
		assertSourceExcludesAll(t, source, []string{
			"normalizeTemporalRequestPlatform(",
			"NavigationTarget",
			"FollowUpReads",
		})
		if len(callNames) != 1 || callNames[0] != "resolveTemporalRequestPlatform" {
			t.Fatalf("resolveLayerTemporalPlatform() calls = %v, want only resolveTemporalRequestPlatform", callNames)
		}
	})
}
