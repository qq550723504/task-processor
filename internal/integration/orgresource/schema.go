package orgresourceadapter

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type organizationResourceBucketRow struct {
	OrganizationID string                               `gorm:"column:organization_id;primaryKey;size:128;not null"`
	ResourceType   string                               `gorm:"column:resource_type;primaryKey;size:64;not null;check:chk_org_resource_bucket_type,resource_type IN ('store_renewal_period','ai_point','data_row')"`
	Available      int64                                `gorm:"column:available;not null;default:0;check:chk_org_resource_available_nonnegative,available >= 0"`
	Reserved       int64                                `gorm:"column:reserved;not null;default:0;check:chk_org_resource_reserved_nonnegative,reserved >= 0"`
	Consumed       int64                                `gorm:"column:consumed;not null;default:0;check:chk_org_resource_consumed_nonnegative,consumed >= 0"`
	CreatedAt      time.Time                            `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt      time.Time                            `gorm:"column:updated_at;not null;autoUpdateTime"`
	Reservations   []organizationResourceReservationRow `gorm:"foreignKey:OrganizationID,ResourceType;references:OrganizationID,ResourceType;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	Debts          []organizationResourceDebtRow        `gorm:"foreignKey:OrganizationID,ResourceType;references:OrganizationID,ResourceType;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (organizationResourceBucketRow) TableName() string {
	return "saas_organization_resource_buckets"
}

type organizationResourceOperationRow struct {
	OrganizationID      string                               `gorm:"column:organization_id;primaryKey;size:128;not null"`
	OperationID         string                               `gorm:"column:operation_id;primaryKey;size:128;not null"`
	OperationType       string                               `gorm:"column:operation_type;size:96;not null"`
	RequestFingerprint  string                               `gorm:"column:request_fingerprint;size:64;not null"`
	State               string                               `gorm:"column:state;size:32;not null;check:chk_org_resource_operation_state,state IN ('processing','succeeded','failed')"`
	FailureCode         string                               `gorm:"column:failure_code;size:96"`
	FailureHTTPStatus   *int                                 `gorm:"column:failure_http_status"`
	ImmutableResult     string                               `gorm:"column:immutable_result_snapshot;type:text"`
	ApprovalEvidenceID  string                               `gorm:"column:approval_evidence_id;size:192"`
	CreatedAt           time.Time                            `gorm:"column:created_at;not null;autoCreateTime"`
	CompletedAt         *time.Time                           `gorm:"column:completed_at"`
	SourceClaims        []organizationResourceSourceClaimRow `gorm:"foreignKey:OrganizationID,OperationID;references:OrganizationID,OperationID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	Events              []organizationResourceEventRow       `gorm:"foreignKey:OrganizationID,OperationID;references:OrganizationID,OperationID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	CreatedReservations []organizationResourceReservationRow `gorm:"foreignKey:OrganizationID,OperationID;references:OrganizationID,OperationID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	SettledReservations []organizationResourceReservationRow `gorm:"foreignKey:OrganizationID,SettlementOperationID;references:OrganizationID,OperationID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	AuditLogs           []organizationResourceAuditLogRow    `gorm:"foreignKey:OrganizationID,OperationID;references:OrganizationID,OperationID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (organizationResourceOperationRow) TableName() string {
	return "saas_organization_resource_operations"
}

type organizationResourceSourceClaimRow struct {
	SourceType         string    `gorm:"column:source_type;primaryKey;size:96;not null"`
	SourceIdentity     string    `gorm:"column:source_identity;primaryKey;size:192;not null"`
	ResourceType       string    `gorm:"column:resource_type;primaryKey;size:64;not null;check:chk_org_resource_source_claim_type,resource_type IN ('store_renewal_period','ai_point','data_row')"`
	OrganizationID     string    `gorm:"column:organization_id;size:128;not null;index"`
	OperationID        string    `gorm:"column:operation_id;size:128;not null"`
	RequestFingerprint string    `gorm:"column:request_fingerprint;size:64;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (organizationResourceSourceClaimRow) TableName() string {
	return "saas_organization_resource_source_claims"
}

type organizationResourceEventRow struct {
	EventID        string    `gorm:"column:event_id;primaryKey;size:128;not null"`
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:128;not null;index:idx_org_resource_event_operation,priority:1"`
	OperationID    string    `gorm:"column:operation_id;size:128;not null;index:idx_org_resource_event_operation,priority:2"`
	ReservationID  *string   `gorm:"column:reservation_id;size:128;index"`
	ResourceType   string    `gorm:"column:resource_type;size:64;not null;index;check:chk_org_resource_event_type,resource_type IN ('store_renewal_period','ai_point','data_row')"`
	Quantity       int64     `gorm:"column:quantity;not null;check:chk_org_resource_event_quantity,quantity > 0"`
	AvailableDelta int64     `gorm:"column:available_delta;not null"`
	ReservedDelta  int64     `gorm:"column:reserved_delta;not null"`
	ConsumedDelta  int64     `gorm:"column:consumed_delta;not null"`
	Reason         string    `gorm:"column:reason;size:96;not null"`
	SourceType     string    `gorm:"column:source_type;size:96;not null"`
	SourceIdentity string    `gorm:"column:source_identity;size:192;not null"`
	BalanceAfter   int64     `gorm:"column:balance_after;not null;check:chk_org_resource_event_balance_nonnegative,balance_after >= 0"`
	AvailableAfter int64     `gorm:"column:available_after;not null;default:0;check:chk_org_resource_event_available_nonnegative,available_after >= 0"`
	ReservedAfter  int64     `gorm:"column:reserved_after;not null;default:0;check:chk_org_resource_event_reserved_nonnegative,reserved_after >= 0"`
	ConsumedAfter  int64     `gorm:"column:consumed_after;not null;default:0;check:chk_org_resource_event_consumed_nonnegative,consumed_after >= 0"`
	GrossCredit    int64     `gorm:"column:gross_credit;not null;default:0;check:chk_org_resource_event_gross_credit_nonnegative,gross_credit >= 0"`
	DebtRepaid     int64     `gorm:"column:debt_repaid;not null;default:0;check:chk_org_resource_event_debt_repaid_nonnegative,debt_repaid >= 0"`
	NetCredit      int64     `gorm:"column:net_credit;not null;default:0;check:chk_org_resource_event_net_credit_nonnegative,net_credit >= 0"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (organizationResourceEventRow) TableName() string {
	return "saas_organization_resource_events"
}

type organizationResourceReservationRow struct {
	OrganizationID        string                         `gorm:"column:organization_id;primaryKey;size:128;not null;uniqueIndex:uq_org_resource_reservation_scope,priority:1;uniqueIndex:uq_org_resource_reservation_owner,priority:1"`
	ReservationID         string                         `gorm:"column:reservation_id;primaryKey;size:128;not null;uniqueIndex:uq_org_resource_reservation_scope,priority:2"`
	OperationID           string                         `gorm:"column:operation_id;size:128;not null;index"`
	SettlementOperationID *string                        `gorm:"column:settlement_operation_id;size:128;index"`
	OwnerType             string                         `gorm:"column:owner_type;size:96;not null;uniqueIndex:uq_org_resource_reservation_owner,priority:2"`
	OwnerAttemptID        string                         `gorm:"column:owner_attempt_id;size:192;not null;uniqueIndex:uq_org_resource_reservation_owner,priority:3"`
	BusinessScope         string                         `gorm:"column:business_scope;size:256;not null"`
	ResourceType          string                         `gorm:"column:resource_type;size:64;not null;check:chk_org_resource_reservation_type,resource_type IN ('store_renewal_period','ai_point','data_row');uniqueIndex:uq_org_resource_reservation_scope,priority:3;uniqueIndex:uq_org_resource_reservation_owner,priority:4"`
	ReservationPurpose    string                         `gorm:"column:reservation_purpose;size:96;not null;uniqueIndex:uq_org_resource_reservation_owner,priority:5"`
	Quantity              int64                          `gorm:"column:quantity;not null;check:chk_org_resource_reservation_quantity,quantity > 0"`
	State                 string                         `gorm:"column:state;size:32;not null;check:chk_org_resource_reservation_state,state IN ('reserved','committed','released','reconciliation_required')"`
	RequestFingerprint    string                         `gorm:"column:request_fingerprint;size:64;not null"`
	CreatedAt             time.Time                      `gorm:"column:created_at;not null;autoCreateTime"`
	SettledAt             *time.Time                     `gorm:"column:settled_at"`
	Events                []organizationResourceEventRow `gorm:"foreignKey:OrganizationID,ReservationID,ResourceType;references:OrganizationID,ReservationID,ResourceType;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (organizationResourceReservationRow) TableName() string {
	return "saas_organization_resource_reservations"
}

type organizationResourceDebtRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:128;not null"`
	ResourceType   string    `gorm:"column:resource_type;primaryKey;size:64;not null;check:chk_org_resource_debt_type,resource_type IN ('store_renewal_period','ai_point','data_row')"`
	Amount         int64     `gorm:"column:amount;not null;default:0;check:chk_org_resource_debt_nonnegative,amount >= 0"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (organizationResourceDebtRow) TableName() string {
	return "saas_organization_resource_debts"
}

type organizationResourceAuditLogRow struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement"`
	OrganizationID     string    `gorm:"column:organization_id;size:128;not null;index"`
	OperationID        string    `gorm:"column:operation_id;size:128;not null;index"`
	Action             string    `gorm:"column:action;size:96;not null"`
	ActorID            string    `gorm:"column:actor_id;size:192;not null"`
	ApprovalEvidenceID string    `gorm:"column:approval_evidence_id;size:192"`
	Payload            string    `gorm:"column:payload;type:text;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

func (organizationResourceAuditLogRow) TableName() string {
	return "saas_organization_resource_audit_logs"
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("organization resource database is required")
	}
	return db.AutoMigrate(
		&organizationResourceOperationRow{},
		&organizationResourceBucketRow{},
		&organizationResourceDebtRow{},
		&organizationResourceSourceClaimRow{},
		&organizationResourceReservationRow{},
		&organizationResourceEventRow{},
		&organizationResourceAuditLogRow{},
	)
}
