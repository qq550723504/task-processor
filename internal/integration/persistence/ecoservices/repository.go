package ecoservices

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	e "task-processor/internal/ecoservices"
	"time"
)

type Repository struct{ db *gorm.DB }

func (r *Repository) ReadMutationResult(ctx context.Context, c e.Command) (e.Result, bool, error) {
	var row operationRow
	var result e.Result
	err := r.db.WithContext(ctx).Where("organization_id=? AND kind=? AND key=?", c.Scope.OrganizationID, c.Kind, c.Key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, false, nil
	}
	if err != nil {
		return result, false, err
	}
	if row.Fingerprint != c.Fingerprint || json.Unmarshal(row.Result, &result) != nil {
		return result, true, e.ErrConflict
	}
	return result, true, nil
}

func requireMerchantEvidence(tx *gorm.DB, ids []string, org string) error {
	var count int64
	if err := tx.Model(&fileRow{}).Where("id IN ? AND organization_id=? AND parent_kind=? AND state=? AND content_type IN ? AND size_bytes BETWEEN 1 AND ?", ids, org, "APPLICATION", "CONFIRMED", []string{"image/png", "image/jpeg"}, 2<<20).Count(&count).Error; err != nil {
		return err
	}
	if count < 1 {
		return e.ErrInvalid
	}
	return nil
}

type applicationRow struct {
	ID             string `gorm:"primaryKey"`
	OrganizationID string `gorm:"uniqueIndex;not null"`
	State          string
	Version        int64
	MerchantID     string
	Payload        []byte
	UpdatedAt      time.Time
}

func (applicationRow) TableName() string { return "ecoservices_applications" }

type listingRow struct {
	ID                     string `gorm:"primaryKey"`
	ProviderOrganizationID string `gorm:"index;not null"`
	State                  string `gorm:"index"`
	Category               string `gorm:"index"`
	Title                  string
	Version                int64
	Payload                []byte
}

func (listingRow) TableName() string { return "ecoservices_listings" }

type requestRow struct {
	ID                                          string `gorm:"primaryKey"`
	BuyerOrganizationID, ProviderOrganizationID string `gorm:"index;not null"`
	State                                       string `gorm:"index"`
	Title                                       string
	Category                                    string
	Version                                     int64
	OrderID                                     *string `gorm:"uniqueIndex"`
	PaymentReceiptID                            string
	FinancialFence                              bool
	FinancialRevision                           int64
	FinancialState                              string
	FundsExpireAt                               *time.Time `gorm:"index"`
	Payload                                     []byte
	CreatedAt, UpdatedAt                        time.Time
}

func (requestRow) TableName() string { return "ecoservices_requests" }

type operationRow struct {
	OrganizationID string `gorm:"primaryKey"`
	Kind           string `gorm:"primaryKey"`
	Key            string `gorm:"primaryKey"`
	Fingerprint    string
	Result         []byte
}

func (operationRow) TableName() string { return "ecoservices_operations" }

type versionRow struct {
	ID        string `gorm:"primaryKey"`
	Kind      string `gorm:"primaryKey"`
	Version   int64  `gorm:"primaryKey"`
	ActorID   string
	Payload   []byte
	CreatedAt time.Time
}

func (versionRow) TableName() string { return "ecoservices_versions" }

type financialRow struct {
	RecoveryGeneration int64
	NextAttemptAt      time.Time `gorm:"index"`
	ID                 string    `gorm:"primaryKey"`
	RequestID          string    `gorm:"index;not null"`
	OrderID            string    `gorm:"index;not null"`
	Kind               string
	Fingerprint        string
	Payload            []byte
	State              string `gorm:"index"`
	DispatchAdmitted   bool
	Result             []byte
	CreatedAt          time.Time
}

func (financialRow) TableName() string { return "ecoservices_financial_commands" }

type merchantBindingRow struct {
	ApplicationID     string `gorm:"primaryKey"`
	OrganizationID    string `gorm:"uniqueIndex;not null"`
	MerchantID        string `gorm:"uniqueIndex;not null"`
	OriginalAttemptID string `gorm:"uniqueIndex;not null"`
	Proof             []byte
}

func (merchantBindingRow) TableName() string { return "ecoservices_merchant_bindings" }

type fileRow struct {
	ID                                       string `gorm:"primaryKey"`
	OrganizationID                           string `gorm:"index;not null"`
	ParentID, ParentKind                     string
	ActorID                                  string
	Fingerprint                              string
	Filename, ContentType, SHA256, ObjectKey string
	SizeBytes                                int64
	State                                    string
	CreatedAt                                time.Time
}

func (fileRow) TableName() string { return "ecoservices_files" }

// Install is only used by the explicit greenfield schema installer and tests.
func Install(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return e.ErrUnavailable
	}
	return db.WithContext(ctx).AutoMigrate(&applicationRow{}, &listingRow{}, &requestRow{}, &operationRow{}, &versionRow{}, &financialRow{}, &merchantBindingRow{}, &fileRow{}, &merchantIntentRow{}, &merchantProgressRow{})
}
func listingRecord(v e.Listing) *listingRow {
	p, _ := json.Marshal(v)
	return &listingRow{ID: v.ID, ProviderOrganizationID: v.ProviderOrganizationID, State: v.State, Category: string(v.Category), Title: v.Title, Version: v.Version, Payload: p}
}
func requestRecord(v e.Request) *requestRow {
	p, _ := json.Marshal(v)
	var order *string
	if v.OrderID != "" {
		o := v.OrderID
		order = &o
	}
	return &requestRow{ID: v.ID, BuyerOrganizationID: v.BuyerOrganizationID, ProviderOrganizationID: v.ProviderOrganizationID, State: v.State, Title: v.Title, Category: string(v.Category), Version: v.Version, OrderID: order, PaymentReceiptID: v.PaymentReceiptID, FinancialFence: v.FinancialFence, FinancialRevision: v.FinancialRevision, FinancialState: v.FinancialState, FundsExpireAt: v.FundsExpireAt, Payload: p, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func applicationRecord(v e.Application) *applicationRow {
	p, _ := json.Marshal(v)
	return &applicationRow{ID: v.ID, OrganizationID: v.OrganizationID, State: v.State, Version: v.Version, MerchantID: v.MerchantID, Payload: p, UpdatedAt: v.UpdatedAt}
}
func requestFact(v requestRow) (e.Request, error) {
	var r e.Request
	if json.Unmarshal(v.Payload, &r) != nil {
		return r, e.ErrConflict
	}
	r.BuyerOrganizationID = v.BuyerOrganizationID
	r.ProviderOrganizationID = v.ProviderOrganizationID
	r.FinancialRevision = v.FinancialRevision
	r.PaymentReceiptID = v.PaymentReceiptID
	r.FinancialFence = v.FinancialFence
	return r, nil
}
func applicationFact(v applicationRow) (e.Application, error) {
	var r e.Application
	if json.Unmarshal(v.Payload, &r) != nil {
		return r, e.ErrConflict
	}
	r.OrganizationID = v.OrganizationID
	r.MerchantID = v.MerchantID
	return r, nil
}
func listingFact(v listingRow) (e.Listing, error) {
	var r e.Listing
	if json.Unmarshal(v.Payload, &r) != nil {
		return r, e.ErrConflict
	}
	r.ProviderOrganizationID = v.ProviderOrganizationID
	return r, nil
}
func lockOperation(tx *gorm.DB, c e.Command) error {
	if tx.Dialector.Name() == "postgres" {
		return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "ecoservices:"+e.Fingerprint([]string{c.Scope.OrganizationID, c.Kind, c.Key})).Error
	}
	if tx.Dialector.Name() == "sqlite" {
		return nil
	}
	return e.ErrUnavailable
}
func (r *Repository) Apply(ctx context.Context, c e.Command, freezeDays int) (e.Result, error) {
	var out e.Result
	if r == nil || r.db == nil || c.Fingerprint == "" {
		return out, e.ErrUnavailable
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockOperation(tx, c); err != nil {
			return err
		}
		var old operationRow
		if err := tx.Where("organization_id=? AND kind=? AND key=?", c.Scope.OrganizationID, c.Kind, c.Key).Take(&old).Error; err == nil {
			if old.Fingerprint != c.Fingerprint {
				return e.ErrConflict
			}
			if json.Unmarshal(old.Result, &out) != nil {
				return e.ErrConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		now := time.Now().UTC()
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("ecoservices:"+c.Scope.OrganizationID+":"+c.Kind+":"+c.Key)).String()
		var versionKind, versionID string
		var version int64
		var payload []byte
		switch c.Kind {
		case "application_submit":
			var previous applicationRow
			readErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id=?", c.Scope.OrganizationID).Take(&previous).Error
			correction := readErr == nil
			if readErr != nil && !errors.Is(readErr, gorm.ErrRecordNotFound) {
				return readErr
			}
			app := *c.Application
			app.ID = id
			app.Version = 1
			if correction {
				old, err := applicationFact(previous)
				if err != nil {
					return err
				}
				if old.State != "REJECTED" || old.Version != c.Version || old.MerchantID != "" || old.OnboardingState != "NOT_STARTED" {
					return e.ErrConflict
				}
				app.ID = old.ID
				app.Version = old.Version + 1
			} else if c.Version != 0 {
				return e.ErrConflict
			}
			app.OrganizationID = c.Scope.OrganizationID
			app.State = "SUBMITTED"
			app.AgreementVersion = e.PolicyVersion
			app.AgreementAccepted = false
			app.MerchantID = ""
			app.OnboardingState = "NOT_STARTED"
			app.ReviewReason = ""
			app.UpdatedAt = now
			if err := requireMerchantEvidence(tx, app.FileIDs, c.Scope.OrganizationID); err != nil {
				return err
			}
			if err := attachFiles(tx, app.FileIDs, c.Scope.OrganizationID, "APPLICATION", app.ID); err != nil {
				return err
			}
			var saveErr error
			if correction {
				saveErr = tx.Save(applicationRecord(app)).Error
			} else {
				saveErr = tx.Create(applicationRecord(app)).Error
			}
			if saveErr != nil {
				return saveErr
			}
			out.Application = &app
			versionKind = "APPLICATION"
			versionID = app.ID
			version = app.Version
			payload, _ = json.Marshal(app)
		case "application_review", "application_reject", "agreement_accept":
			var row applicationRow
			q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", c.ID)
			if !c.Scope.Platform {
				q = q.Where("organization_id=?", c.Scope.OrganizationID)
			}
			if err := q.Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return e.ErrNotFound
			} else if err != nil {
				return err
			}
			app, err := applicationFact(row)
			if err != nil {
				return err
			}
			if app.Version != c.Version {
				return e.ErrConflict
			}
			if c.Kind == "agreement_accept" {
				if app.State != "APPROVED" {
					return e.ErrConflict
				}
				app.AgreementAccepted = true
			} else {
				if app.State != "SUBMITTED" {
					return e.ErrConflict
				}
				app.State = "APPROVED"
				if c.Kind == "application_review" {
					if err := requireMerchantEvidence(tx, app.FileIDs, app.OrganizationID); err != nil {
						return err
					}
				}
				if c.Kind == "application_reject" {
					app.State = "REJECTED"
				}
				app.ReviewReason = c.Reason
			}
			if app.State == "APPROVED" && app.AgreementAccepted && app.MerchantID != "" && app.OnboardingState == "FINISH" {
				app.State = "ACTIVE"
			}
			app.Version++
			app.UpdatedAt = now
			if err := tx.Save(applicationRecord(app)).Error; err != nil {
				return err
			}
			out.Application = &app
			versionKind = "APPLICATION"
			versionID = app.ID
			version = app.Version
			payload, _ = json.Marshal(app)
		case "listing_create", "listing_update", "listing_publish":
			var appRow applicationRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id=? AND state=?", c.Scope.OrganizationID, "ACTIVE").Take(&appRow).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return e.ErrNotQualified
			} else if err != nil {
				return err
			}
			app, err := applicationFact(appRow)
			if err != nil {
				return err
			}
			if app.MerchantID == "" {
				return e.ErrNotQualified
			}
			var item e.Listing
			if c.Kind == "listing_create" {
				item = *c.Listing
				item.ID = id
				item.Version = 1
				item.State = "DRAFT"
			} else {
				var row listingRow
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND provider_organization_id=?", c.ID, c.Scope.OrganizationID).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					return e.ErrNotFound
				} else if err != nil {
					return err
				}
				item, err = listingFact(row)
				if err != nil {
					return err
				}
				if item.Version != c.Version {
					return e.ErrConflict
				}
				if c.Kind == "listing_update" {
					updated := *c.Listing
					updated.ID = item.ID
					updated.Version = item.Version
					updated.State = "DRAFT"
					item = updated
				} else {
					if e.ValidateListing(&item, freezeDays) != nil {
						return e.ErrInvalid
					}
					item.State = "PUBLISHED"
				}
				item.Version++
			}
			item.ProviderOrganizationID = c.Scope.OrganizationID
			item.ProviderName = app.CompanyName
			if err := tx.Save(listingRecord(item)).Error; err != nil {
				return err
			}
			out.Listing = &item
			versionKind = "LISTING"
			versionID = item.ID
			version = item.Version
			payload, _ = json.Marshal(item)
		case "request_create":
			var listing listingRow
			if err := tx.Where("id=? AND state=?", c.ID, "PUBLISHED").Take(&listing).Error; err != nil {
				return e.ErrNotFound
			}
			var qualified applicationRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id=? AND state=?", listing.ProviderOrganizationID, "ACTIVE").Take(&qualified).Error; err != nil || qualified.MerchantID == "" {
				return e.ErrNotQualified
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND state=?", c.ID, "PUBLISHED").Take(&listing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return e.ErrNotFound
			} else if err != nil {
				return err
			}
			item, err := listingFact(listing)
			if err != nil {
				return err
			}
			if item.ProviderOrganizationID == c.Scope.OrganizationID {
				return e.ErrForbidden
			}
			req := e.Request{ID: id, BuyerOrganizationID: c.Scope.OrganizationID, ProviderOrganizationID: item.ProviderOrganizationID, ListingID: item.ID, ListingVersion: item.Version, Title: item.Title, Category: item.Category, Description: c.Description, FileIDs: c.FileIDs, State: "REQUESTED", Version: 1, CreatedAt: now, UpdatedAt: now}
			if err := attachFiles(tx, req.FileIDs, c.Scope.OrganizationID, "REQUEST", req.ID); err != nil {
				return err
			}
			if err := tx.Create(requestRecord(req)).Error; err != nil {
				return err
			}
			out.Request = &req
			versionKind = "REQUEST"
			versionID = req.ID
			version = req.Version
			payload, _ = json.Marshal(req)
		default:
			var row requestRow
			q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", c.ID)
			if !c.Scope.Platform {
				q = q.Where("buyer_organization_id=? OR provider_organization_id=?", c.Scope.OrganizationID, c.Scope.OrganizationID)
			}
			if err := q.Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return e.ErrNotFound
			} else if err != nil {
				return err
			}
			req, err := requestFact(row)
			if err != nil {
				return err
			}
			fc, err := e.TransitionRequest(&req, c, now)
			if err != nil {
				return err
			}
			if c.Kind == "deliver" {
				if err := attachFiles(tx, req.Delivery.FileIDs, c.Scope.OrganizationID, "REQUEST", req.ID); err != nil {
					return err
				}
			}
			if fc != nil {
				var app applicationRow
				if err := tx.Where("organization_id=?", req.ProviderOrganizationID).Take(&app).Error; err != nil {
					return e.ErrNotQualified
				}
				if app.MerchantID == "" {
					return e.ErrNotQualified
				}
				if fc.Kind == "CREATE_PURCHASE" && app.State != "ACTIVE" {
					return e.ErrNotQualified
				}
				fc.MerchantID = app.MerchantID
				data, err := json.Marshal(fc)
				if err != nil {
					return err
				}
				if err := tx.Create(&financialRow{ID: fc.ID, RequestID: req.ID, OrderID: req.OrderID, Kind: fc.Kind, Fingerprint: e.Fingerprint(fc), Payload: data, State: "PENDING", CreatedAt: now}).Error; err != nil {
					return err
				}
			}
			if err := tx.Save(requestRecord(req)).Error; err != nil {
				return err
			}
			out.Request = &req
			versionKind = "REQUEST"
			versionID = req.ID
			version = req.Version
			payload, _ = json.Marshal(req)
		}
		if err := tx.Create(&versionRow{ID: versionID, Kind: versionKind, Version: version, ActorID: c.Scope.ActorID, Payload: payload, CreatedAt: now}).Error; err != nil {
			return err
		}
		result, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.Create(&operationRow{OrganizationID: c.Scope.OrganizationID, Kind: c.Kind, Key: c.Key, Fingerprint: c.Fingerprint, Result: result}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func attachFiles(tx *gorm.DB, ids []string, org, kind, parent string) error {
	for _, id := range ids {
		var file fileRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND organization_id=? AND state=?", id, org, "CONFIRMED").Take(&file).Error; err != nil {
			return e.ErrForbidden
		}
		if file.ParentID != "" && (file.ParentID != parent || file.ParentKind != kind) {
			return e.ErrForbidden
		}
		if file.ParentID == "" {
			if file.ParentKind != kind {
				return e.ErrForbidden
			}
			if err := tx.Model(&file).Update("parent_id", parent).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *Repository) Read(ctx context.Context, q e.Query) (e.Page, error) {
	out := e.Page{Counts: map[string]int64{}}
	db := r.db.WithContext(ctx)
	offset := (q.Page - 1) * q.PageSize
	switch q.Kind {
	case "applications":
		query := db.Model(&applicationRow{})
		if !q.Scope.Platform {
			query = query.Where("organization_id=?", q.Scope.OrganizationID)
		}
		if q.ID != "" {
			query = query.Where("id=?", q.ID)
		}
		if q.State != "" {
			query = query.Where("state=?", q.State)
		}
		if err := query.Count(&out.Total).Error; err != nil {
			return out, err
		}
		var rows []applicationRow
		if err := query.Order("updated_at DESC,id DESC").Offset(offset).Limit(q.PageSize).Find(&rows).Error; err != nil {
			return out, err
		}
		out.Applications = []e.Application{}
		for _, row := range rows {
			v, err := applicationFact(row)
			if err != nil {
				return out, err
			}
			out.Applications = append(out.Applications, v)
		}
	case "catalog", "provider_listings":
		query := db.Model(&listingRow{})
		if q.Kind == "catalog" {
			query = query.Where("state=?", "PUBLISHED").Where("EXISTS (SELECT 1 FROM ecoservices_applications a WHERE a.organization_id=ecoservices_listings.provider_organization_id AND a.state=? AND a.merchant_id<>'')", "ACTIVE")
		} else {
			query = query.Where("provider_organization_id=?", q.Scope.OrganizationID)
		}
		if q.ID != "" {
			query = query.Where("id=?", q.ID)
		}
		if q.Search != "" {
			query = query.Where("title LIKE ?", "%"+q.Search+"%")
		}
		var categories []struct {
			Category string
			Count    int64
		}
		if err := query.Session(&gorm.Session{}).Select("category,COUNT(*) AS count").Group("category").Scan(&categories).Error; err != nil {
			return out, err
		}
		for _, v := range categories {
			out.Counts[v.Category] = v.Count
		}
		if q.Category != "" {
			query = query.Where("category=?", q.Category)
		}
		if q.Group == "enterprise" {
			query = query.Where("category IN ?", []e.Category{e.CompanyRegistration, e.TrademarkRegistration})
		} else if q.Group == "shop" {
			query = query.Where("category IN ?", []e.Category{e.StoreOpening, e.StoreOperation})
		}
		if err := query.Count(&out.Total).Error; err != nil {
			return out, err
		}
		var rows []listingRow
		if err := query.Order("id").Offset(offset).Limit(q.PageSize).Find(&rows).Error; err != nil {
			return out, err
		}
		out.Listings = []e.Listing{}
		for _, row := range rows {
			v, err := listingFact(row)
			if err != nil {
				return out, err
			}
			out.Listings = append(out.Listings, v)
		}
	case "requests", "due_orders":
		query := db.Model(&requestRow{})
		if q.Kind == "due_orders" {
			query = query.Where("funds_expire_at IS NOT NULL AND funds_expire_at<=? AND state<>? AND financial_state NOT IN ?", time.Now().UTC().AddDate(0, 0, 7), "CANCELLED", []string{"SETTLED", "REFUNDED", "CLOSED_UNPAID"})
		}
		if !q.Scope.Platform {
			if q.Side == "buyer" {
				query = query.Where("buyer_organization_id=?", q.Scope.OrganizationID)
			} else if q.Side == "provider" {
				query = query.Where("provider_organization_id=?", q.Scope.OrganizationID)
			} else {
				query = query.Where("buyer_organization_id=? OR provider_organization_id=?", q.Scope.OrganizationID, q.Scope.OrganizationID)
			}
		}
		if q.ID != "" {
			query = query.Where("id=?", q.ID)
		}
		if q.Category != "" {
			query = query.Where("category=?", q.Category)
		}
		if q.Search != "" {
			query = query.Where("title LIKE ? OR id LIKE ?", "%"+q.Search+"%", "%"+q.Search+"%")
		}
		if q.From != nil {
			query = query.Where("created_at>=?", *q.From)
		}
		if q.To != nil {
			query = query.Where("created_at<?", *q.To)
		}
		var counts []struct {
			State string
			Count int64
		}
		if err := query.Session(&gorm.Session{}).Select("state,COUNT(*) AS count").Group("state").Scan(&counts).Error; err != nil {
			return out, err
		}
		for _, v := range counts {
			out.Counts[v.State] = v.Count
		}
		stages := map[string][]string{"pending": {"REQUESTED", "QUOTED", "ORDER_PENDING", "PAID_READY"}, "servicing": {"SERVICING"}, "acceptance": {"AWAITING_ACCEPTANCE"}, "completed": {"ACCEPTED"}, "cancelled": {"CANCEL_REQUESTED", "CANCELLED"}}
		if q.Stage != "" {
			query = query.Where("state IN ?", stages[q.Stage])
		}
		if q.State != "" {
			query = query.Where("state=?", q.State)
		}
		if err := query.Count(&out.Total).Error; err != nil {
			return out, err
		}
		var rows []requestRow
		if err := query.Order("updated_at DESC,id DESC").Offset(offset).Limit(q.PageSize).Find(&rows).Error; err != nil {
			return out, err
		}
		out.Requests = []e.Request{}
		for _, row := range rows {
			v, err := requestFact(row)
			if err != nil {
				return out, err
			}
			v.Side = "buyer"
			if v.ProviderOrganizationID == q.Scope.OrganizationID {
				v.Side = "provider"
			}
			out.Requests = append(out.Requests, v)
		}
	default:
		return out, e.ErrInvalid
	}
	if q.ID != "" && out.Total == 0 {
		return out, e.ErrNotFound
	}
	return out, nil
}
func (r *Repository) PendingFinancialCommands(ctx context.Context, limit int) ([]e.FinancialCommand, error) {
	if limit < 1 || limit > 100 {
		return nil, e.ErrInvalid
	}
	out := make([]e.FinancialCommand, 0, limit)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		var rows []financialRow
		dueOrders := tx.Model(&requestRow{}).Select("order_id").Where("payment_receipt_id<>'' AND financial_fence=false AND state NOT IN ? AND financial_state NOT IN ? AND (funds_expire_at<=? OR state IN ?)", []string{"CANCELLED"}, []string{"SETTLED", "REFUNDED"}, now, []string{"PAID_READY", "SERVICING", "AWAITING_ACCEPTANCE"})
		// A confirmed quote is not a channel dispatch. Checkout's durable
		// admission or a verified inbox wake enables its original recovery;
		// untouched purchases cannot starve financial commands with queries.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("next_attempt_at<=? AND ((state IN ? AND (kind<>'CREATE_PURCHASE' OR dispatch_admitted=true OR recovery_generation>0)) OR (state='DONE' AND kind='CREATE_PURCHASE' AND order_id IN (?)))", now, []string{"PENDING", "PROCESSING"}, dueOrders).Order("next_attempt_at,created_at,id").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			command, err := financialFact(row)
			if err != nil {
				return err
			}
			command.RecoveryGeneration = row.RecoveryGeneration
			out = append(out, command)
			// Rotate before external work, including timeout/UNKNOWN/error paths.
			delay := 15 * time.Second
			if row.State == "DONE" {
				delay = 30 * time.Minute
			}
			if err := tx.Model(&row).Update("next_attempt_at", now.Add(delay)).Error; err != nil {
				return err
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) FinancialCommand(ctx context.Context, id string) (e.FinancialCommand, error) {
	var row financialRow
	if err := r.db.WithContext(ctx).Where("id=?", id).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return e.FinancialCommand{}, e.ErrNotFound
	} else if err != nil {
		return e.FinancialCommand{}, err
	}
	return financialFact(row)
}
func (r *Repository) OriginalFinancialCommand(ctx context.Context, order string) (e.FinancialCommand, error) {
	var rows []financialRow
	if err := r.db.WithContext(ctx).Where("order_id=? AND kind='CREATE_PURCHASE'", order).Limit(2).Find(&rows).Error; err != nil {
		return e.FinancialCommand{}, err
	}
	if len(rows) != 1 {
		return e.FinancialCommand{}, e.ErrNotFound
	}
	return financialFact(rows[0])
}
func financialFact(row financialRow) (e.FinancialCommand, error) {
	var out e.FinancialCommand
	if json.Unmarshal(row.Payload, &out) != nil || out.ID != row.ID || out.OrderID != row.OrderID || out.RequestID != row.RequestID || out.Kind != row.Kind || e.Fingerprint(out) != row.Fingerprint {
		return out, e.ErrConflict
	}
	out.DispatchAdmitted = row.DispatchAdmitted
	return out, nil
}
func (r *Repository) AdmitFinancialCommand(ctx context.Context, in e.FinancialCommand) (e.FinancialCommand, error) {
	var out e.FinancialCommand
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request requestRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", in.RequestID).Take(&request).Error; err != nil {
			return err
		}
		var row financialRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", in.ID).Take(&row).Error; err != nil {
			return err
		}
		original := in
		original.DispatchAdmitted = false
		original.DispatchOperationID = ""
		if row.Fingerprint != e.Fingerprint(original) || row.OrderID != in.OrderID {
			return e.ErrConflict
		}
		if json.Unmarshal(row.Payload, &out) != nil {
			return e.ErrConflict
		}
		if in.Kind == "CREATE_PURCHASE" {
			// A checkout receipt records the original admission, not permission
			// to disclose a payment capability after cancellation. Recheck under
			// the request lock before either historical admission shortcut.
			req, err := requestFact(request)
			if err != nil {
				return err
			}
			if req.State != "ORDER_PENDING" || req.FinancialFence {
				return e.ErrConflict
			}
		}
		admissionKey := in.DispatchOperationID
		if admissionKey != "" {
			if len(admissionKey) > 192 {
				return e.ErrInvalid
			}
			var prior operationRow
			if err := tx.Where("organization_id=? AND kind=? AND key=?", "SYSTEM_FINANCE", "financial_dispatch", admissionKey).Take(&prior).Error; err == nil {
				if prior.Fingerprint != e.Fingerprint(original) {
					return e.ErrConflict
				}
				out.DispatchAdmitted = true
				return nil
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if row.DispatchAdmitted && admissionKey == "" {
			out.DispatchAdmitted = true
			return nil
		}
		req, err := requestFact(request)
		if err != nil {
			return err
		}
		switch in.Kind {
		case "CREATE_PURCHASE":
			if req.State != "ORDER_PENDING" || req.FinancialFence {
				return e.ErrConflict
			}
		case "SETTLE":
			if req.AcceptanceID != in.SourceProofID || req.FinancialFence || req.State != "ACCEPTED" {
				return e.ErrConflict
			}
		case "CANCEL":
			if req.State != "CANCEL_REQUESTED" {
				return e.ErrConflict
			}
		case "REFUND":
			if req.Refund == nil || req.Refund.State != "APPROVED" || req.Refund.AmountMinor != in.AmountMinor {
				return e.ErrConflict
			}
		default:
			return e.ErrInvalid
		}
		// The immutable original intent is admitted once. Later refunds must queue
		// behind this in-flight intent in billing, rather than undo its permission.
		if err := tx.Model(&row).Updates(map[string]any{"dispatch_admitted": true, "state": "PROCESSING", "next_attempt_at": time.Time{}}).Error; err != nil {
			return err
		}
		if admissionKey != "" {
			if err := tx.Create(&operationRow{OrganizationID: "SYSTEM_FINANCE", Kind: "financial_dispatch", Key: admissionKey, Fingerprint: e.Fingerprint(original), Result: row.Payload}).Error; err != nil {
				return err
			}
		}
		out.DispatchAdmitted = true
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return out, err
}
func (r *Repository) CompleteFinancialCommand(ctx context.Context, in e.FinancialCommand, result e.FinancialResult) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request requestRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", in.RequestID).Take(&request).Error; err != nil {
			return err
		}
		var row financialRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", in.ID).Take(&row).Error; err != nil {
			return err
		}
		original := in
		original.DispatchAdmitted = false
		original.DispatchOperationID = ""
		if row.Fingerprint != e.Fingerprint(original) {
			return e.ErrConflict
		}
		req, err := requestFact(request)
		if err != nil {
			return err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		terminal := result.State == "SETTLED" || result.State == "REFUNDED" || result.State == "CLOSED_UNPAID" || result.State == "CHANNEL_OPERATION_FAILED" || in.Kind == "CREATE_PURCHASE" && result.PaymentReceiptID != ""
		state := "PROCESSING"
		if terminal {
			state = "DONE"
		}
		// A newer verified notification must not be erased by a worker that
		// selected the original command before that notification was persisted.
		if row.RecoveryGeneration > in.RecoveryGeneration {
			state = "PROCESSING"
		}
		if string(row.Result) == string(data) {
			return tx.Model(&row).Update("state", state).Error
		}
		if in.Kind == "CREATE_PURCHASE" && result.State == "CANCELLATION_PENDING" && result.PaymentReceiptID != "" && req.PaymentReceiptID == "" && result.Revision >= req.FinancialRevision && (req.State == "CANCEL_REQUESTED" || req.State == "CANCELLED" && req.FinancialState == "CLOSED_UNPAID") {
			// The original close completed before a verified late payment. Wake
			// its exact cancellation, not a new refund or a new purchase. The
			// request lock and generation fence retain a concurrent newer wake.
			var cancellations []financialRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id=? AND request_id=? AND kind='CANCEL'", in.OrderID, in.RequestID).Limit(2).Find(&cancellations).Error; err != nil {
				return err
			}
			if len(cancellations) != 1 {
				return e.ErrConflict
			}
			cancelRow := cancellations[0]
			cancel, err := financialFact(cancelRow)
			var closed e.FinancialResult
			if err != nil || cancel.BuyerOrganizationID != in.BuyerOrganizationID || cancel.ProviderOrganizationID != in.ProviderOrganizationID || cancel.MerchantID != in.MerchantID || cancel.Quote != in.Quote || cancel.AmountMinor != in.AmountMinor || cancel.PolicyVersion != in.PolicyVersion {
				return e.ErrConflict
			}
			if cancelRow.State == "DONE" {
				if !cancelRow.DispatchAdmitted || json.Unmarshal(cancelRow.Result, &closed) != nil || closed.OrderID != in.OrderID || closed.State != "CLOSED_UNPAID" || closed.PaymentReceiptID != "" || closed.ReceiptID == "" {
					return e.ErrConflict
				}
			} else if cancelRow.State != "PENDING" && cancelRow.State != "PROCESSING" {
				return e.ErrConflict
			}
			if err := tx.Model(&cancelRow).Updates(map[string]any{"state": "PROCESSING", "next_attempt_at": time.Time{}, "recovery_generation": gorm.Expr("recovery_generation+1")}).Error; err != nil {
				return err
			}
			req.State = "CANCEL_REQUESTED"
		}
		if err := e.ApplyFinancialResult(&req, result, time.Now().UTC()); err != nil {
			return err
		}
		if err := tx.Save(requestRecord(req)).Error; err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"state": state, "result": data}).Error
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
func (r *Repository) WakeOriginalServicePurchase(ctx context.Context, orderID string) error {
	original, err := r.OriginalFinancialCommand(ctx, orderID)
	if err != nil {
		return err
	}
	updated := r.db.WithContext(ctx).Model(&financialRow{}).Where("id=? AND kind=?", original.ID, "CREATE_PURCHASE").Updates(map[string]any{"state": "PROCESSING", "next_attempt_at": time.Time{}, "recovery_generation": gorm.Expr("recovery_generation+1")})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return e.ErrNotFound
	}
	return nil
}
