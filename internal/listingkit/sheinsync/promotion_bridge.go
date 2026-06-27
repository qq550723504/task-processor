package sheinsync

import "task-processor/internal/shein/api/marketing"

type SheinPromotionRegistrationResult struct {
	Request          *marketing.SaveConfigRequest
	Response         *marketing.SaveConfigResponse
	ActivityRequest  *marketing.CreateActivityRequest
	ActivityResponse *marketing.CreateActivityResponse
	FilterReasons    map[string]string
}
