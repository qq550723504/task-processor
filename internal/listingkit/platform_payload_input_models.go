package listingkit

import amazonlisting "task-processor/internal/amazonlisting"

type amazonPreviewPayloadInput struct {
	draft      *amazonlisting.AmazonListingDraft
	visualBase platformVisualPreviewPayloadBase
}

type reviewablePlatformPreviewPayloadInput struct {
	base reviewablePlatformPreviewPayloadBase
}

type amazonExportPayloadInput struct {
	draft      *amazonlisting.AmazonListingDraft
	visualBase platformVisualExportBase
}

type reviewableExportPayloadInput struct {
	visualBase platformVisualExportBase
}
