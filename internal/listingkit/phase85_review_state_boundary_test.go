package listingkit

import "testing"

func TestSheinReviewStateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("inspection review adapter delegates to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "workflow_review_state.go", "sheinInspectionReviewReasons")
		callNames := readNamedFunctionCallNames(t, "workflow_review_state.go", "sheinInspectionReviewReasons")

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.InspectionReviewReasons(result.Shein)",
		})
		assertSourceExcludesAll(t, source, []string{
			"result.Shein.Inspection.Summary",
			"result.Shein.ReviewNotes",
			"SHEIN 信息需要人工复核",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"InspectionReviewReasons",
		})
	})

	t.Run("cookie review adapters delegate to workspace", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			fn          string
			wantSource  []string
			excludeText []string
			wantCalls   []string
		}{
			{
				name:       "collect notes",
				fn:         "sheinCookieUnavailableReviewNotes",
				wantSource: []string{"return sheinworkspace.CookieUnavailableReviewNotes(pkg)"},
				excludeText: []string{
					"pkg.CategoryResolution.ReviewNotes",
					"pkg.AttributeResolution.ReviewNotes",
					"pkg.SaleAttributeResolution.ReviewNotes",
				},
				wantCalls: []string{"CookieUnavailableReviewNotes"},
			},
			{
				name:       "strip notes",
				fn:         "stripSheinCookieUnavailableReviewNotes",
				wantSource: []string{"sheinworkspace.StripCookieUnavailableReviewNotes(pkg)"},
				excludeText: []string{
					"pkg.ReviewNotes = filterOutSheinCookieUnavailableReviewNotes(pkg.ReviewNotes)",
				},
				wantCalls: []string{"StripCookieUnavailableReviewNotes"},
			},
			{
				name:       "filter notes",
				fn:         "filterOutSheinCookieUnavailableReviewNotes",
				wantSource: []string{"return sheinworkspace.FilterOutCookieUnavailableReviewNotes(notes)"},
				excludeText: []string{
					"for _, note := range notes",
				},
				wantCalls: []string{"FilterOutCookieUnavailableReviewNotes"},
			},
			{
				name:       "availability check",
				fn:         "sheinCookieUnavailable",
				wantSource: []string{"return sheinworkspace.HasCookieUnavailableReviewNotes(pkg)"},
				excludeText: []string{
					"len(sheinCookieUnavailableReviewNotes(pkg)) > 0",
				},
				wantCalls: []string{"HasCookieUnavailableReviewNotes"},
			},
			{
				name:       "text classifier",
				fn:         "isSheinCookieUnavailableText",
				wantSource: []string{"return sheinworkspace.IsCookieUnavailableText(value)"},
				excludeText: []string{
					`strings.Contains(text, "cookie 不可用")`,
					`strings.Contains(text, "店铺 cookie")`,
				},
				wantCalls: []string{"IsCookieUnavailableText"},
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				source := readNamedFunctionSource(t, "workflow_review_state.go", tc.fn)
				callNames := readNamedFunctionCallNames(t, "workflow_review_state.go", tc.fn)

				assertSourceContainsAll(t, source, tc.wantSource)
				assertSourceExcludesAll(t, source, tc.excludeText)
				assertFunctionCallsContainAll(t, callNames, tc.wantCalls)
			})
		}
	})
}
