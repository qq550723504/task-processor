package workspace

type HistoryDetail[Record any, Payload any] struct {
	TaskID         string             `json:"task_id"`
	Record         *Record            `json:"record,omitempty"`
	Navigation     *HistoryNavigation `json:"navigation,omitempty"`
	RestorePayload *Payload           `json:"restore_payload,omitempty"`
	HistoryIndex   int                `json:"history_index"`
	TotalRecords   int                `json:"total_records"`
	IsTruncated    bool               `json:"is_truncated"`
	MaxRecords     int                `json:"max_records"`
}

func BuildHistoryDetail[Record any, Payload any](
	taskID string,
	record *Record,
	navigation *HistoryNavigation,
	restorePayload *Payload,
	historyIndex int,
	totalRecords int,
	isTruncated bool,
	maxRecords int,
) *HistoryDetail[Record, Payload] {
	return &HistoryDetail[Record, Payload]{
		TaskID:         taskID,
		Record:         record,
		Navigation:     navigation,
		RestorePayload: restorePayload,
		HistoryIndex:   historyIndex,
		TotalRecords:   totalRecords,
		IsTruncated:    isTruncated,
		MaxRecords:     maxRecords,
	}
}
