package sheinsync

import (
	"testing"

	"task-processor/internal/shein/api/marketing"

	"github.com/stretchr/testify/require"
)

func TestBuildPromotionEnrollmentResultsMarksSKCResponseErrorAsFailed(t *testing.T) {
	candidate := SheinActivityEnrollmentCandidate{
		CandidateID: 88478,
		SKCName:     "ss260622223319766135914",
	}
	bridgeResult := &SheinPromotionRegistrationResult{
		ActivityRequest: &marketing.CreateActivityRequest{
			AddCostAndStockInfoList: []marketing.CostAndStockInfo{{
				Skc: candidate.SKCName,
			}},
		},
		ActivityResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "OK",
			Info: &marketing.ActivityCreateInfo{
				ActivityID: 0,
				SkcErrorInfo: map[string]any{
					candidate.SKCName: "mrs-simple_platform_limit_discounts-101017",
				},
			},
		},
	}

	results := buildPromotionEnrollmentResults(
		[]SheinActivityEnrollmentCandidate{candidate},
		bridgeResult,
		nil,
		map[string]marketing.SkcInfo{candidate.SKCName: {Skc: candidate.SKCName}},
		nil,
	)

	require.Len(t, results, 1)
	require.False(t, results[0].Success)
	require.Equal(t, "mrs-simple_platform_limit_discounts-101017", results[0].ErrorMessage)
	require.Contains(t, results[0].ResponsePayload, "skc_error_info")
}

func TestBuildPromotionEnrollmentResultsMarksMissingActivityIDAsFailed(t *testing.T) {
	candidate := SheinActivityEnrollmentCandidate{CandidateID: 1, SKCName: "skc-without-activity"}
	bridgeResult := &SheinPromotionRegistrationResult{
		ActivityRequest: &marketing.CreateActivityRequest{
			AddCostAndStockInfoList: []marketing.CostAndStockInfo{{Skc: candidate.SKCName}},
		},
		ActivityResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "OK",
			Info: &marketing.ActivityCreateInfo{ActivityID: 0},
		},
	}

	results := buildPromotionEnrollmentResults(
		[]SheinActivityEnrollmentCandidate{candidate},
		bridgeResult,
		nil,
		map[string]marketing.SkcInfo{candidate.SKCName: {Skc: candidate.SKCName}},
		nil,
	)

	require.Len(t, results, 1)
	require.False(t, results[0].Success)
	require.Equal(t, "SHEIN did not create an activity", results[0].ErrorMessage)
}
