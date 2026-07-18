package workflow

import "time"

const SDSDesignSyncTimeout = 180 * time.Second
const SDSDesignSyncExtraPollCap = 24

func SDSDesignSyncTimeoutForVariantCount(targetCount int) time.Duration {
	if targetCount <= 1 {
		return SDSDesignSyncTimeout
	}
	extraPolls := (targetCount - 1) * 8
	if extraPolls > SDSDesignSyncExtraPollCap {
		extraPolls = SDSDesignSyncExtraPollCap
	}
	return SDSDesignSyncTimeout + time.Duration(extraPolls)*5*time.Second
}
