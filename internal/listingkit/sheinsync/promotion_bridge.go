package sheinsync

import "task-processor/internal/shein/api/marketing"

type SheinPromotionRegistrationResult struct {
	Request  *marketing.SaveConfigRequest
	Response *marketing.SaveConfigResponse
}
