package commercialbilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/commercial/billing"
	"task-processor/internal/ledger/money"
	"time"
)

type servicePurchaseRow struct {
	OrderID             string `gorm:"primaryKey;size:128"`
	RequestID           string `gorm:"uniqueIndex;size:128;not null"`
	BuyerOrganizationID string `gorm:"index;size:128;not null"`
	TradeNo             string `gorm:"uniqueIndex;size:32;not null"`
	Fingerprint         string
	Payload             []byte
	Version             int64
	LeaseToken          string
	LeaseUntil          time.Time
}

func (servicePurchaseRow) TableName() string { return "commercial_service_orders" }

type servicePurchaseOperationRow struct {
	OperationID       string `gorm:"primaryKey;size:128"`
	ProviderRequestID string `gorm:"uniqueIndex;size:32;not null"`
	OrderID           string `gorm:"index;size:128;not null"`
	Fingerprint       string
	Payload           []byte
}

func (servicePurchaseOperationRow) TableName() string { return "commercial_service_operations" }

type servicePaymentInboxRow struct {
	EventID     string `gorm:"primaryKey;size:128"`
	OrderID     string `gorm:"index;size:128;not null"`
	Fingerprint string
	Payload     []byte
	CreatedAt   time.Time
}

func (servicePaymentInboxRow) TableName() string { return "commercial_service_payment_inbox" }

type serviceEffectInboxRow struct {
	EventID              string `gorm:"primaryKey;size:128"`
	OrderID, OperationID string `gorm:"index;size:128;not null"`
	Fingerprint          string
	Payload              []byte
	CreatedAt            time.Time
}

func (serviceEffectInboxRow) TableName() string { return "commercial_service_effect_inbox" }
func migrateServicePurchases(db *gorm.DB) error {
	return db.AutoMigrate(&servicePurchaseRow{}, &servicePurchaseOperationRow{}, &servicePaymentInboxRow{}, &serviceEffectInboxRow{})
}
func decodeServicePurchase(row servicePurchaseRow) (billing.ServicePurchaseOrder, error) {
	var o billing.ServicePurchaseOrder
	if json.Unmarshal(row.Payload, &o) != nil || o.Source.Validate() != nil || o.Profile.Validate() != nil || o.Source.Kind != "CREATE_PURCHASE" || o.Source.OrderID != row.OrderID || o.Source.RequestID != row.RequestID || o.Source.BuyerOrganizationID != row.BuyerOrganizationID || o.TradeNo != row.TradeNo || o.Fingerprint() != row.Fingerprint {
		return o, billing.ErrConflict
	}
	o.Version = row.Version
	o.LeaseToken = row.LeaseToken
	o.LeaseUntil = row.LeaseUntil
	return o, nil
}
func (r *Repository) CreateServicePurchase(ctx context.Context, c billing.ServicePurchaseCommand, p billing.ServiceMerchantProfile) (billing.ServicePurchaseOrder, error) {
	var out billing.ServicePurchaseOrder
	if c.Validate() != nil || c.Kind != "CREATE_PURCHASE" || p.Validate() != nil || c.DeliveryDays > p.FreezeDays || c.ProviderMerchantID == p.PlatformMerchantID {
		return out, billing.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := money.NormalizeTimestamp(r.now())
		id := money.ServiceFingerprint([]string{"service-payment", c.OrderID})[:32]
		o := billing.ServicePurchaseOrder{Source: c, Profile: p, TradeNo: id, CreatedAt: now, ExpiresAt: now.Add(30 * time.Minute), State: "AWAITING_PAYMENT", Version: 1, CompletedCommands: map[string]billing.ServicePurchaseResult{}, Operations: map[string]billing.ServiceFinancialOperation{}, Effects: map[string]money.ServiceReceipt{}}
		payload, _ := json.Marshal(o)
		row := servicePurchaseRow{OrderID: c.OrderID, RequestID: c.RequestID, BuyerOrganizationID: c.BuyerOrganizationID, TradeNo: id, Fingerprint: o.Fingerprint(), Payload: payload, Version: 1}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		var stored servicePurchaseRow
		if err := tx.Where("order_id=?", c.OrderID).Take(&stored).Error; err != nil {
			return billing.ErrConflict
		}
		var err error
		out, err = decodeServicePurchase(stored)
		if err != nil {
			return err
		}
		if out.Source.Fingerprint() != c.Fingerprint() || out.Profile != p {
			return billing.ErrConflict
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) ReadServicePurchase(ctx context.Context, id string) (billing.ServicePurchaseOrder, error) {
	var row servicePurchaseRow
	if err := r.db.WithContext(ctx).Where("order_id=?", id).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return billing.ServicePurchaseOrder{}, billing.ErrNotFound
	} else if err != nil {
		return billing.ServicePurchaseOrder{}, err
	}
	return decodeServicePurchase(row)
}
func (r *Repository) ClaimServicePurchase(ctx context.Context, id, token string, until time.Time) (billing.ServicePurchaseOrder, error) {
	var out billing.ServicePurchaseOrder
	if token == "" || !until.After(r.now()) || until.After(r.now().Add(2*time.Minute)) {
		return out, billing.ErrInvalid
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row servicePurchaseRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id=?", id).Take(&row).Error; err != nil {
			return err
		}
		if row.LeaseToken != "" && row.LeaseUntil.After(r.now()) {
			return billing.ErrConflict
		}
		row.LeaseToken = token
		row.LeaseUntil = until
		row.Version++
		if err := tx.Model(&row).Updates(map[string]any{"lease_token": token, "lease_until": until, "version": row.Version}).Error; err != nil {
			return err
		}
		var err error
		out, err = decodeServicePurchase(row)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) SaveServicePurchase(ctx context.Context, o billing.ServicePurchaseOrder) (billing.ServicePurchaseOrder, error) {
	var out billing.ServicePurchaseOrder
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row servicePurchaseRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id=?", o.Source.OrderID).Take(&row).Error; err != nil {
			return err
		}
		if row.Fingerprint != o.Fingerprint() || row.Version != o.Version || row.LeaseToken != o.LeaseToken || o.LeaseToken == "" || !row.LeaseUntil.After(r.now()) {
			return billing.ErrConflict
		}
		before, err := decodeServicePurchase(row)
		if err != nil {
			return err
		}
		if before.PaymentDispatched && !o.PaymentDispatched || before.CancelRequested && !o.CancelRequested || before.PaymentReceiptID != "" && before.PaymentReceiptID != o.PaymentReceiptID {
			return billing.ErrConflict
		}
		if o.Operation != nil {
			original := *o.Operation
			original.Dispatched = false
			payload, _ := json.Marshal(original)
			intent := servicePurchaseOperationRow{OperationID: original.Reservation.OperationID, ProviderRequestID: original.ProviderRequestID, OrderID: o.Source.OrderID, Fingerprint: money.ServiceFingerprint(original), Payload: payload}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&intent).Error; err != nil {
				return err
			}
			var old servicePurchaseOperationRow
			if err := tx.Where("operation_id=?", intent.OperationID).Take(&old).Error; err != nil {
				return err
			}
			if old.Fingerprint != intent.Fingerprint {
				return billing.ErrConflict
			}
		}
		o.Version++
		payload, err := json.Marshal(o)
		if err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"payload": payload, "version": o.Version}).Error; err != nil {
			return err
		}
		out = o
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) ReleaseServicePurchase(ctx context.Context, id, token string) error {
	return r.db.WithContext(ctx).Model(&servicePurchaseRow{}).Where("order_id=? AND lease_token=?", id, token).Updates(map[string]any{"lease_token": "", "lease_until": time.Time{}}).Error
}
func (r *Repository) RecordServicePaymentObservation(ctx context.Context, o billing.ServicePurchaseOrder, p billing.ServicePaymentObservation) error {
	original, err := r.ReadServicePurchase(ctx, o.Source.OrderID)
	if err != nil {
		return err
	}
	if original.Fingerprint() != o.Fingerprint() || !p.Matches(original) {
		return billing.ErrConflict
	}
	payload, _ := json.Marshal(p)
	row := servicePaymentInboxRow{EventID: p.EventID, OrderID: o.Source.OrderID, Fingerprint: money.ServiceFingerprint(p), Payload: payload, CreatedAt: r.now()}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return err
	}
	var old servicePaymentInboxRow
	if err := r.db.WithContext(ctx).Where("event_id=?", p.EventID).Take(&old).Error; err != nil {
		return err
	}
	if old.OrderID != row.OrderID || old.Fingerprint != row.Fingerprint {
		return billing.ErrConflict
	}
	return nil
}
func (r *Repository) RecordServiceOperationObservation(ctx context.Context, o billing.ServicePurchaseOrder, op billing.ServiceFinancialOperation, p billing.ServiceOperationObservation) error {
	original, err := r.ReadServicePurchase(ctx, o.Source.OrderID)
	if err != nil {
		return err
	}
	if original.Fingerprint() != o.Fingerprint() || !p.Matches(o, op) {
		return billing.ErrConflict
	}
	intent := op
	intent.Dispatched = false
	var stored servicePurchaseOperationRow
	if err := r.db.WithContext(ctx).Where("operation_id=? AND order_id=?", op.Reservation.OperationID, o.Source.OrderID).Take(&stored).Error; err != nil {
		return billing.ErrConflict
	}
	if stored.Fingerprint != money.ServiceFingerprint(intent) {
		return billing.ErrConflict
	}
	payload, _ := json.Marshal(p)
	row := serviceEffectInboxRow{EventID: p.EventID, OrderID: o.Source.OrderID, OperationID: op.Reservation.OperationID, Fingerprint: money.ServiceFingerprint(p), Payload: payload, CreatedAt: r.now()}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return err
	}
	var old serviceEffectInboxRow
	if err := r.db.WithContext(ctx).Where("event_id=?", p.EventID).Take(&old).Error; err != nil {
		return err
	}
	if old.OrderID != row.OrderID || old.OperationID != row.OperationID || old.Fingerprint != row.Fingerprint {
		return billing.ErrConflict
	}
	return nil
}
func (r *Repository) ServicePaymentObservations(ctx context.Context, id string) ([]billing.ServicePaymentObservation, error) {
	var rows []servicePaymentInboxRow
	if err := r.db.WithContext(ctx).Where("order_id=?", id).Order("created_at,event_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]billing.ServicePaymentObservation, 0, len(rows))
	for _, row := range rows {
		var p billing.ServicePaymentObservation
		if json.Unmarshal(row.Payload, &p) != nil || money.ServiceFingerprint(p) != row.Fingerprint {
			return nil, billing.ErrConflict
		}
		out = append(out, p)
	}
	return out, nil
}
func (r *Repository) ServiceOperationObservations(ctx context.Context, id string) ([]billing.ServiceOperationObservation, error) {
	var rows []serviceEffectInboxRow
	if err := r.db.WithContext(ctx).Where("operation_id=?", id).Order("created_at,event_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]billing.ServiceOperationObservation, 0, len(rows))
	for _, row := range rows {
		var p billing.ServiceOperationObservation
		if json.Unmarshal(row.Payload, &p) != nil || money.ServiceFingerprint(p) != row.Fingerprint {
			return nil, billing.ErrConflict
		}
		out = append(out, p)
	}
	return out, nil
}

var _ billing.ServicePurchaseStore = (*Repository)(nil)

// Serving checks never install schema. Original intents and signed inbox facts
// are immutable, including column-level grants that bypass table checks.
func VerifyServicePurchasesRuntime(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return billing.ErrFeatureUnavailable
	}
	var safe bool
	if err := db.WithContext(ctx).Raw(`SELECT NOT(rolsuper OR rolcreaterole OR rolcreatedb OR rolbypassrls) AND NOT has_schema_privilege(current_user,'public','CREATE') AND NOT has_database_privilege(current_user,current_database(),'CREATE,TEMP') AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND pg_has_role(current_user,c.relowner,'MEMBER')) FROM pg_roles WHERE rolname=current_user`).Scan(&safe).Error; err != nil || !safe {
		return billing.ErrFeatureUnavailable
	}
	for _, table := range []string{"commercial_service_orders", "commercial_service_operations", "commercial_service_payment_inbox", "commercial_service_effect_inbox"} {
		var valid bool
		mutable := table == "commercial_service_orders"
		if err := db.WithContext(ctx).Raw(`SELECT has_table_privilege(current_user,?,'SELECT') AND has_table_privilege(current_user,?,'INSERT') AND has_table_privilege(current_user,?,'UPDATE')=? AND NOT has_table_privilege(current_user,?,'DELETE,TRUNCATE,REFERENCES,TRIGGER') AND (? OR NOT has_any_column_privilege(current_user,?,'UPDATE'))`, table, table, table, mutable, table, mutable, table).Scan(&valid).Error; err != nil || !valid {
			return billing.ErrFeatureUnavailable
		}
	}
	var foreign int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','p') AND (c.relname LIKE 'ledger\_%' ESCAPE '\' OR c.relname LIKE 'ecoservices\_%' ESCAPE '\') AND (has_any_column_privilege(current_user,c.oid,'INSERT,UPDATE,REFERENCES') OR has_table_privilege(current_user,c.oid,'DELETE,TRUNCATE,TRIGGER'))`).Scan(&foreign).Error; err != nil || foreign > 0 {
		return billing.ErrFeatureUnavailable
	}
	return nil
}
