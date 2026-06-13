package amazonlisting

import (
	amazonapi "task-processor/internal/amazon/api"
	amazonpublishing "task-processor/internal/marketplace/amazon/publishing"
)

func normalizeListingIssues(resp *amazonapi.ListingResponse) []AmazonIssue {
	return amazonpublishing.NormalizeListingIssues(resp)
}

func summarizeAmazonIssues(issues []AmazonIssue) *AmazonIssueSummary {
	return amazonpublishing.SummarizeAmazonIssues(issues)
}
