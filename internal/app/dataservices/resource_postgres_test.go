package dataservicesapp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"task-processor/internal/dataservice"
	"task-processor/internal/integration/acquisition/amazon"
	resourceadapter "task-processor/internal/integration/orgresource"
	keystore "task-processor/internal/integration/persistence/dataservice"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	jobstore "task-processor/internal/integration/persistence/product/dataacquisition"
	sourcestore "task-processor/internal/integration/persistence/product/sourcing"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"testing"
	"time"
)

type executionFixture struct {
	authorizedFixture
	denied          bool
	fetches         int
	fetchErr        error
	authDelay       time.Duration
	executionChecks int
	startErr        error
	starts          int
	credentialCheck func(context.Context, collection.Scope, string) error
}

func (f *executionFixture) Check(ctx context.Context, scope collection.Scope, permission string) error {
	if f.credentialCheck != nil {
		return f.credentialCheck(ctx, scope, permission)
	}
	return nil
}

func (f *executionFixture) CheckExecution(ctx context.Context, _ dataacquisition.Principal, _ orgresource.ResourceFunding) error {
	f.executionChecks++
	if f.authDelay > 0 {
		timer := time.NewTimer(f.authDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if f.denied {
		return dataacquisition.ErrForbidden
	}
	return nil
}
func (f *executionFixture) EnsureExecution(context.Context, dataacquisition.Job) error {
	f.starts++
	return f.startErr
}
func (f *executionFixture) Funding(context.Context, collection.Scope) (orgresource.ResourceFunding, error) {
	return orgresource.FundingEnterprise, nil
}
func (f *executionFixture) Ready(context.Context) error   { return nil }
func (f *executionFixture) Sites() []dataacquisition.Site { return dataacquisition.Sites() }
func (f *executionFixture) Discover(_ context.Context, q dataacquisition.Query) ([]string, error) {
	return q.ASINs, nil
}
func (f *executionFixture) Fetch(_ context.Context, site, asin string) (dataacquisition.Evidence, error) {
	f.fetches++
	if f.fetchErr != nil {
		return dataacquisition.Evidence{}, f.fetchErr
	}
	return dataacquisition.Evidence{Site: site, ASIN: asin, Title: "controlled fixture", MainImage: "https://m.media-amazon.com/images/I/fixture.jpg", Availability: "available", Price: 10, Currency: "USD", CapturedAt: time.Now().UTC().Format(time.RFC3339Nano), ParserVersion: "amazon-v1"}, nil
}

type lostReserveAck struct {
	*orgresource.ConsumerChargeService
	lose bool
}

type unknownAvailabilityFixture struct {
	*executionFixture
}

func (f *unknownAvailabilityFixture) Fetch(_ context.Context, site, asin string) (dataacquisition.Evidence, error) {
	f.fetches++
	raw := `<input id="ASIN" value="` + asin + `"><span id="productTitle">Incomplete public product</span><img id="landingImage" src="https://m.media-amazon.com/images/I/fixture.jpg">`
	return amazon.ParseProduct(raw, site, asin, time.Now())
}

type unavailableReconcile struct {
	*orgresource.ConsumerChargeService
	unavailable bool
}

func (c *unavailableReconcile) Reconcile(ctx context.Context, id orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	if c.unavailable {
		return orgresource.ConsumerChargeReceipt{}, dataacquisition.ErrUnknown
	}
	return c.ConsumerChargeService.Reconcile(ctx, id)
}

type terminalDiscoveryFixture struct {
	*executionFixture
	result error
	calls  int
}

func (f *terminalDiscoveryFixture) Discover(context.Context, dataacquisition.Query) ([]string, error) {
	f.calls++
	return nil, f.result
}

func (c *lostReserveAck) Reserve(ctx context.Context, id orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	receipt, err := c.ConsumerChargeService.Reserve(ctx, id)
	if err == nil && c.lose {
		c.lose = false
		return orgresource.ConsumerChargeReceipt{}, dataacquisition.ErrUnknown
	}
	return receipt, err
}

func TestTwoDatabasesRecoverOriginalReservationAndChargeOnlySavedProduct(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("product621"), tcpostgres.WithUsername("test_owner"), tcpostgres.WithPassword("isolated-data621"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	open := func(dsn string) *gorm.DB {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return db
	}
	productDB := open(dsn)
	require.NoError(t, productDB.Exec("CREATE DATABASE resource621").Error)
	address, err := url.Parse(dsn)
	require.NoError(t, err)
	address.Path = "/resource621"
	resourceDB := open(address.String())
	require.NoError(t, keystore.InstallSchema(productDB))
	require.NoError(t, catalogstore.AutoMigrate(productDB))
	require.NoError(t, sourcestore.InstallSchema(productDB))
	require.NoError(t, collectionstore.InstallSchema(productDB))
	require.NoError(t, jobstore.InstallSchema(productDB))
	require.NoError(t, resourceadapter.AutoMigrate(resourceDB))
	require.NoError(t, resourceDB.Exec("INSERT INTO saas_organization_resource_buckets(organization_id,resource_type,available,allocated,reserved,consumed,created_at,updated_at) VALUES('org','data_row',10,0,0,0,now(),now())").Error)
	fixture := &executionFixture{}
	publisher, err := NewProductPublisher(fixture)
	require.NoError(t, err)
	repo, err := jobstore.NewRepository(ctx, productDB, fixture, publisher, transactionResultReader)
	require.NoError(t, err)
	resourceRepo, err := resourceadapter.NewGormConsumerChargeRepository(resourceDB, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	charges, err := orgresource.NewConsumerChargeService(resourceRepo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerAmazonData: dataacquisition.ChargeOwner{Repository: repo}})
	require.NoError(t, err)
	ack := &lostReserveAck{ConsumerChargeService: charges, lose: true}
	service, err := dataacquisition.NewService(repo, fixture, fixture, ack, fixture)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "member"}
	q := dataacquisition.Query{Site: "us", Mode: "asin", ASINs: []string{"B000123456"}, Limit: 1, Fields: []string{"asin", "title"}}
	job, err := service.Start(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), q, orgresource.FundingEnterprise, 5)
	require.NoError(t, err)
	job, err = repo.Discover(ctx, job, q.ASINs)
	require.NoError(t, err)
	items, err := repo.Items(ctx, job)
	require.NoError(t, err)
	require.ErrorIs(t, service.ProcessItem(ctx, job, items[0], ""), dataacquisition.ErrUnknown)
	chargeID := orgresource.ConsumerChargeIdentity{OrganizationID: "org", Consumer: orgresource.ConsumerAmazonData, OperationID: items[0].ID}
	original, err := charges.Lookup(ctx, chargeID)
	require.NoError(t, err)
	require.Equal(t, orgresource.ReservationReserved, original.State)
	fixture.denied = true
	require.NoError(t, service.ProcessItem(ctx, job, items[0], ""))
	released, err := charges.Lookup(ctx, chargeID)
	require.NoError(t, err)
	require.Equal(t, original.ReservationID, released.ReservationID)
	require.Equal(t, orgresource.ReservationReleased, released.State)
	require.Zero(t, fixture.fetches)
	var bucket struct{ Available, Reserved, Consumed int64 }
	readBalance := func() {
		require.NoError(t, resourceDB.Raw("SELECT available,reserved,consumed FROM saas_organization_resource_buckets WHERE organization_id='org' AND resource_type='data_row'").Scan(&bucket).Error)
	}
	readBalance()
	require.Equal(t, int64(10), bucket.Available)
	require.Zero(t, bucket.Reserved)
	fixture.denied = false
	job, err = service.Start(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), q, orgresource.FundingEnterprise, 5)
	require.NoError(t, err)
	require.NoError(t, service.Run(ctx, scope, job.ID))
	require.NoError(t, service.Run(ctx, scope, job.ID))
	require.Equal(t, 1, fixture.fetches)
	job, err = service.Read(ctx, dataacquisition.Principal{Scope: scope}, job.ID)
	require.NoError(t, err)
	require.Equal(t, "SUCCEEDED", job.State)
	require.Equal(t, int64(5), job.ConfirmedFen)
	require.Zero(t, job.PendingFen)
	readBalance()
	require.Equal(t, int64(9), bucket.Available)
	require.Equal(t, int64(1), bucket.Consumed)
	require.Zero(t, bucket.Reserved)
	store, err := sourcestore.NewRepository(productDB, newCatalogBridge)
	require.NoError(t, err)
	page, err := service.Results(ctx, dataacquisition.Principal{Scope: scope}, job.ID, "", 100, capturedResultReader{store})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Len(t, page.Items[0].Data, 2)
	require.Equal(t, "B000123456", page.Items[0].Data["asin"])
	foreign := scope
	foreign.ActorID = "another"
	_, err = service.Results(ctx, dataacquisition.Principal{Scope: foreign}, job.ID, "", 100, capturedResultReader{store})
	require.ErrorIs(t, err, dataacquisition.ErrNotFound)
	savedItems, err := repo.Items(ctx, job)
	require.NoError(t, err)
	bad := *savedItems[0].Evidence
	bad.Title = "changed"
	require.ErrorIs(t, capturedResultReader{store}.Verify(ctx, scope, *savedItems[0].Source, bad), dataacquisition.ErrUnavailable)
	t.Run("bounded console and DataKey routes use original owner", func(t *testing.T) {
		require.NoError(t, keystore.InstallCustomSchema(productDB))
		module, err := NewModule(ctx, Dependencies{ProductDB: productDB, Access: fixture, Live: fixture, Specialist: fixture, Funding: fixture, Provider: fixture, Starter: fixture, Charges: func(owner orgresource.ConsumerChargeOwner) (dataacquisition.Charges, error) {
			return orgresource.NewConsumerChargeService(resourceRepo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerAmazonData: owner})
		}})
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		for _, route := range module.BuildRoutes() {
			router.Handle(route.Method, route.Path, route.Handler)
		}
		send := func(method, path, body, authorization, command string) *httptest.ResponseRecorder {
			request := httptest.NewRequest(method, path, strings.NewReader(body))
			request.TLS = &tls.ConnectionState{}
			request.RemoteAddr = "198.51.100.2:4567"
			request.Header.Set("Content-Type", "application/json")
			if authorization != "" {
				request.Header.Set("Authorization", authorization)
			}
			if command != "" {
				request.Header.Set("Idempotency-Key", command)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, request)
			return w
		}
		command := uuid.NewString()
		w := send("POST", ConsoleBase+"/keys", `{"name":"fixture","expiresAt":"`+time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)+`","dailyRows":2,"monthlyCostFen":10,"permissions":["amazon.acquire","amazon.result.read"]}`, "", command)
		require.Equal(t, 200, w.Code)
		var created dataservice.KeyCreated
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
		require.Len(t, created.Secret, 43)
		keyHeader := "DataKey " + created.Key.ID + "." + created.Secret
		jobCommand := uuid.NewString()
		w = send("POST", APIBase, `{"query":{"site":"us","mode":"asin","asins":["B000123456"],"limit":1},"maximumRows":1,"maximumCostFen":5}`, keyHeader, jobCommand)
		require.Equal(t, http.StatusAccepted, w.Code)
		var externalJob dataacquisition.Job
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &externalJob))
		require.NoError(t, module.Runner().Run(ctx, scope, externalJob.ID))
		productPool, err := productDB.DB()
		require.NoError(t, err)
		productPool.SetMaxOpenConns(1)
		t.Cleanup(func() { productPool.SetMaxOpenConns(0) })
		w = send("GET", APIBase+"/"+externalJob.ID+"/results", "", keyHeader, "")
		require.Equal(t, 200, w.Code, "job/items/SRC/Catalog reads must reuse the credential guard's single transaction connection")
		require.Contains(t, w.Body.String(), "controlled fixture")
		fixture.startErr = dataacquisition.ErrUnavailable
		t.Cleanup(func() { fixture.startErr = nil })
		starts := fixture.starts
		w = send("GET", APIBase+"/"+externalJob.ID, "", keyHeader, "")
		require.Equal(t, 200, w.Code, "terminal Product facts must remain readable during Temporal outage")
		w = send("GET", APIBase+"/"+externalJob.ID+"/results", "", keyHeader, "")
		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), "controlled fixture")
		w = send("POST", APIBase, `{"query":{"site":"us","mode":"asin","asins":["B000123456"],"limit":1},"maximumRows":1,"maximumCostFen":5}`, keyHeader, jobCommand)
		require.Equal(t, http.StatusAccepted, w.Code, "terminal command replay uses the original result")
		require.Equal(t, starts, fixture.starts)
		productPool.SetMaxOpenConns(0)
		fixture.startErr = nil
		t.Run("saved result read observes committed revocation", func(t *testing.T) {
			fixture.credentialCheck = func(ctx context.Context, _ collection.Scope, permission string) error {
				if permission == dataservice.PermissionResult {
					fixture.credentialCheck = nil
					_, err := module.keys.Change(ctx, created.Key.ID, uuid.NewString(), 1, dataservice.KeyPatch{State: "REVOKED"})
					return err
				}
				return nil
			}
			t.Cleanup(func() { fixture.credentialCheck = nil })
			w = send("GET", APIBase+"/"+externalJob.ID+"/results", "", keyHeader, "")
			require.Equal(t, 403, w.Code, "a revoke committed after authentication's key snapshot must prevent saved-result disclosure")
			require.NotContains(t, w.Body.String(), "controlled fixture")
		})
		w = send("GET", APIBase+"/"+externalJob.ID, "", keyHeader, "")
		require.Equal(t, 403, w.Code)
		w = send("GET", ConsoleBase+"/amazon/jobs/"+externalJob.ID, "", "", "")
		require.Equal(t, 200, w.Code)
		w = send("GET", ConsoleBase+"/keys/by-command/"+command, "", "", "")
		require.Equal(t, 200, w.Code)
		require.NotContains(t, w.Body.String(), "secret")
		w = send("GET", ConsoleBase+"/overview", "", "", "")
		require.Equal(t, 200, w.Code)
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		for _, state := range []string{"DISABLED", "REVOKED"} {
			for _, action := range []string{"job", "command", "results"} {
				t.Run("completed credential change rejects "+state+" "+action, func(t *testing.T) {
					created, err := module.keys.Create(ctx, uuid.NewString(), dataservice.KeyInput{Name: "read race", ExpiresAt: time.Now().UTC().Add(time.Hour), DailyRows: 2, MonthlyCostFen: 10, Permissions: []string{dataservice.PermissionAcquire, dataservice.PermissionResult}})
					require.NoError(t, err)
					principal := dataacquisition.Principal{Scope: scope, CredentialID: created.Key.ID, CredentialRevision: 1}
					command := uuid.NewString()
					original, err := module.Runner().Start(ctx, principal, command, q, orgresource.FundingEnterprise, 5)
					require.NoError(t, err)
					_, err = repo.Cancel(ctx, scope, original.ID, uuid.NewString())
					require.NoError(t, err)
					fixture.credentialCheck = func(ctx context.Context, _ collection.Scope, permission string) error {
						if permission == dataservice.PermissionResult {
							fixture.credentialCheck = nil
							_, err := module.keys.Change(ctx, created.Key.ID, uuid.NewString(), 1, dataservice.KeyPatch{State: state})
							return err
						}
						return nil
					}
					t.Cleanup(func() { fixture.credentialCheck = nil })
					path := APIBase + "/" + original.ID
					if action == "command" {
						path = APIBase + "/by-command/" + command
					} else if action == "results" {
						path += "/results"
					}
					w := send("GET", path, "", "DataKey "+created.Key.ID+"."+created.Secret, "")
					require.Equal(t, 403, w.Code)
					require.NotContains(t, w.Body.String(), original.ID)
					w = send("GET", ConsoleBase+"/amazon/jobs/"+original.ID, "", "", "")
					require.Equal(t, 200, w.Code, "original Console ownership remains readable")
				})
			}
		}
		t.Run("committed admission returns UNKNOWN and recovers the same command and quota", func(t *testing.T) {
			created, err := module.keys.Create(ctx, uuid.NewString(), dataservice.KeyInput{Name: "startup outage", ExpiresAt: time.Now().UTC().Add(time.Hour), DailyRows: 2, MonthlyCostFen: 10, Permissions: []string{dataservice.PermissionAcquire, dataservice.PermissionResult}})
			require.NoError(t, err)
			header := "DataKey " + created.Key.ID + "." + created.Secret
			command := uuid.NewString()
			body := `{"query":{"site":"us","mode":"asin","asins":["B000123456"],"limit":1},"maximumRows":1,"maximumCostFen":5}`
			fixture.startErr = dataacquisition.ErrUnavailable
			t.Cleanup(func() { fixture.startErr = nil })
			for attempt := 0; attempt < 2; attempt++ {
				w := send("POST", APIBase, body, header, command)
				require.Equal(t, 503, w.Code)
				require.Contains(t, w.Body.String(), "DATA_UNKNOWN", "post-commit startup failure must preserve the original intent")
			}
			originalID := collection.StableID(scope.OrganizationID, scope.ActorID, "amazon-job", command)
			original, err := repo.Read(ctx, scope, originalID)
			require.NoError(t, err)
			require.Equal(t, "ADMITTED", original.State)
			checkQuota := func(reservedRows, reservedFen int64) {
				quotas, err := repo.KeyQuotas(ctx, scope)
				require.NoError(t, err)
				found := false
				for _, quota := range quotas {
					if quota.KeyID == created.Key.ID {
						found = true
						require.Equal(t, reservedRows, quota.DayReservedRows)
						require.Equal(t, reservedFen, quota.MonthReservedFen)
						require.Zero(t, quota.DayConsumedRows)
					}
				}
				require.True(t, found)
			}
			checkQuota(1, 5)
			fixture.startErr = nil
			w := send("POST", APIBase, body, header, command)
			require.Equal(t, http.StatusAccepted, w.Code)
			var recovered dataacquisition.Job
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &recovered))
			require.Equal(t, originalID, recovered.ID)
			checkQuota(1, 5)
			fixture.startErr = dataacquisition.ErrUnavailable
			starts := fixture.starts
			cancelCommand := uuid.NewString()
			for attempt := 0; attempt < 2; attempt++ {
				w = send("POST", ConsoleBase+"/amazon/jobs/"+originalID+"/cancel", `{}`, "", cancelCommand)
				require.Equal(t, 200, w.Code, "cancel must not require workflow startup")
				require.Contains(t, w.Body.String(), "CANCELED")
			}
			require.Equal(t, starts, fixture.starts, "cancel does not start an admitted job")
			require.NoError(t, module.Runner().Run(ctx, scope, originalID))
			checkQuota(0, 0)
			fixture.startErr = dataacquisition.ErrUnavailable
			w = send("GET", APIBase+"/"+originalID, "", header, "")
			require.Equal(t, 200, w.Code)
			require.Contains(t, w.Body.String(), "CANCELED")
			fixture.startErr = nil
		})
		w = send("POST", ConsoleBase+"/custom", `{"name":"HTTP fixture","query":{"site":"us","mode":"asin","asins":["B000123456"],"limit":1},"purpose":"controlled fixture","format":"json"}`, "", uuid.NewString())
		require.Equal(t, 200, w.Code)
		var custom dataservice.CustomRequest
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &custom))
		change := func(patch string) {
			w = send("POST", SpecialistBase+"/"+custom.ID+"/changes", `{"expectedRevision":`+strconv.FormatInt(custom.Revision, 10)+`,"patch":`+patch+`}`, "", uuid.NewString())
			require.Equal(t, 200, w.Code)
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &custom))
		}
		change(`{"state":"EVALUATING","note":"评估"}`)
		change(`{"state":"SPEC_CONFIRMED","note":"确认","spec":{"description":"one row","quoteNote":"线下报价","confirmationNote":"线下确认","format":"json","maximumRows":1}}`)
		change(`{"state":"PREPARING","note":"制作"}`)
		change(`{"state":"PREPARING","note":"制作进度更新"}`)
		deliveryCommand := uuid.NewString()
		request := httptest.NewRequest("POST", SpecialistBase+"/"+custom.ID+"/delivery", strings.NewReader(`[{"title":"declared HTTP fixture"}]`))
		request.Header.Set("Content-Type", "application/octet-stream")
		request.Header.Set("Idempotency-Key", deliveryCommand)
		request.Header.Set("X-Expected-Revision", strconv.FormatInt(custom.Revision, 10))
		request.Header.Set("X-Spec-Revision", strconv.FormatInt(custom.SpecRevision, 10))
		request.Header.Set("X-Data-Format", "json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, request)
		require.Equal(t, 200, w.Code)
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &custom))
		require.Equal(t, "DELIVERED", custom.State)
		require.Equal(t, 1, custom.DeliveredRows)
		require.NotEmpty(t, custom.BatchID)
		w = send("GET", SpecialistBase+"/by-command/"+deliveryCommand, "", "", "")
		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), `"actorId":"creator"`)
		w = send("GET", ConsoleBase+"/custom", "", "", "")
		require.Equal(t, 200, w.Code)
		require.NotContains(t, w.Body.String(), "applicant")
		require.NotContains(t, w.Body.String(), "events")
		require.Contains(t, w.Body.String(), "HTTP fixture")
		readBalance()
		require.Equal(t, int64(2), bucket.Consumed, "custom delivery does not consume DATA_ROW")
	})
	for _, tc := range []struct {
		name   string
		result error
		reason string
	}{
		{"challenge", amazon.ErrChallenge, "provider_challenged"},
		{"unsupported", amazon.ErrUnsupported, "provider_unsupported"},
	} {
		t.Run("terminal discovery "+tc.name, func(t *testing.T) {
			keyRepo, err := keystore.NewCredentialRepository(ctx, productDB)
			require.NoError(t, err)
			keys, err := dataservice.NewCredentialService(keyRepo, fixture)
			require.NoError(t, err)
			created, err := keys.Create(ctx, uuid.NewString(), dataservice.KeyInput{Name: tc.name, ExpiresAt: time.Now().UTC().Add(time.Hour), DailyRows: 2, MonthlyCostFen: 10, Permissions: []string{dataservice.PermissionAcquire, dataservice.PermissionResult}})
			require.NoError(t, err)
			provider := &terminalDiscoveryFixture{executionFixture: fixture, result: tc.result}
			runner, err := dataacquisition.NewService(repo, fixture, provider, charges, fixture)
			require.NoError(t, err)
			principal := dataacquisition.Principal{Scope: scope, CredentialID: created.Key.ID, CredentialRevision: created.Key.Revision}
			job, err := runner.Start(ctx, principal, uuid.NewString(), dataacquisition.Query{Site: "us", Mode: "keyword", Keyword: "fixture", Limit: 1}, orgresource.FundingEnterprise, 5)
			require.NoError(t, err)
			require.NoError(t, runner.Run(ctx, scope, job.ID))
			require.NoError(t, runner.Run(ctx, scope, job.ID), "recovery must not repeat terminal discovery")
			job, err = repo.Read(ctx, scope, job.ID)
			require.NoError(t, err)
			require.Equal(t, "FAILED", job.State)
			require.Equal(t, tc.reason, job.Reason)
			require.Equal(t, 1, provider.calls)
			quotas, err := repo.KeyQuotas(ctx, scope)
			require.NoError(t, err)
			found := false
			for _, quota := range quotas {
				if quota.KeyID == created.Key.ID {
					found = true
					require.Zero(t, quota.DayReservedRows)
					require.Zero(t, quota.MonthReservedFen)
					require.Zero(t, quota.DayConsumedRows)
					require.Zero(t, quota.MonthConsumedFen)
				}
			}
			require.True(t, found)
			readBalance()
			require.Equal(t, int64(2), bucket.Consumed)
			require.Zero(t, bucket.Reserved)
			require.Equal(t, 2, fixture.fetches)
		})
	}

	t.Run("transient fetch retains reservation and saved row recovers through Resource", func(t *testing.T) {
		provider := &executionFixture{fetchErr: context.DeadlineExceeded}
		delayed := &unavailableReconcile{ConsumerChargeService: charges, unavailable: true}
		runner, err := dataacquisition.NewService(repo, fixture, provider, delayed, fixture)
		require.NoError(t, err)
		job, err := runner.Start(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), q, orgresource.FundingEnterprise, 5)
		require.NoError(t, err)
		require.ErrorIs(t, runner.Run(ctx, scope, job.ID), context.DeadlineExceeded)
		job, err = repo.Read(ctx, scope, job.ID)
		require.NoError(t, err)
		items, err := repo.Items(ctx, job)
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Equal(t, "FETCHING", items[0].State)
		identity := orgresource.ConsumerChargeIdentity{OrganizationID: scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: items[0].ID}
		original, err := charges.Lookup(ctx, identity)
		require.NoError(t, err)
		require.Equal(t, orgresource.ReservationReserved, original.State)
		// Expire the existing claim in the isolated fixture, without waiting or inventing a new recovery path.
		require.NoError(t, productDB.Exec("UPDATE data_acquisition_items SET lease_until=now()-interval '1 second' WHERE job_id=?", job.ID).Error)
		provider.fetchErr = nil
		require.ErrorIs(t, runner.Run(ctx, scope, job.ID), dataacquisition.ErrUnknown)
		items, err = repo.Items(ctx, job)
		require.NoError(t, err)
		require.Equal(t, "SAVED", items[0].State)
		require.Equal(t, 2, provider.fetches)
		require.Equal(t, original.ReservationID, items[0].ReservationID)
		count, err := charges.RecoverDue(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		delayed.unavailable = false
		require.NoError(t, runner.Run(ctx, scope, job.ID))
		require.NoError(t, runner.Run(ctx, scope, job.ID))
		settled, err := charges.Lookup(ctx, identity)
		require.NoError(t, err)
		require.Equal(t, original.ReservationID, settled.ReservationID)
		require.Equal(t, orgresource.ReservationCommitted, settled.State)
		require.Equal(t, 2, provider.fetches)
		readBalance()
		require.Equal(t, int64(3), bucket.Consumed)
		require.Zero(t, bucket.Reserved)
	})
	for _, tc := range []struct {
		name     string
		fetchErr error
		expire   bool
		reason   string
	}{
		{"challenge", dataacquisition.ErrSourceChallenge, false, "provider_challenged"},
		{"unsupported", dataacquisition.ErrSourceUnsupported, false, "provider_unsupported"},
		{"invalid evidence", dataacquisition.ErrInvalid, false, "provider_rejected"},
		{"transient then deadline", context.Canceled, true, "deadline"},
	} {
		t.Run("fetch "+tc.name, func(t *testing.T) {
			provider := &executionFixture{fetchErr: tc.fetchErr}
			runner, err := dataacquisition.NewService(repo, fixture, provider, charges, fixture)
			require.NoError(t, err)
			job, err := runner.Start(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), q, orgresource.FundingEnterprise, 5)
			require.NoError(t, err)
			if tc.expire {
				require.ErrorIs(t, runner.Run(ctx, scope, job.ID), tc.fetchErr)
				// Advance the isolated fixture past its original 30-minute window,
				// preserving the installation constraint between created_at/deadline.
				require.NoError(t, productDB.Exec("UPDATE data_acquisition_jobs SET created_at=now()-interval '31 minutes',deadline=now()-interval '1 minute' WHERE id=?", job.ID).Error)
			}
			require.NoError(t, runner.Run(ctx, scope, job.ID))
			require.NoError(t, runner.Run(ctx, scope, job.ID))
			job, err = repo.Read(ctx, scope, job.ID)
			require.NoError(t, err)
			require.Equal(t, "FAILED", job.State)
			items, err := repo.Items(ctx, job)
			require.NoError(t, err)
			require.Len(t, items, 1)
			require.Equal(t, "FAILED", items[0].State)
			require.Equal(t, orgresource.ReservationReleased, items[0].ChargeState)
			require.Equal(t, 1, provider.fetches)
			require.Equal(t, tc.reason, items[0].Reason)
			readBalance()
			require.Equal(t, int64(3), bucket.Consumed)
			require.Zero(t, bucket.Reserved)
		})
	}
	t.Run("resource owner validates live access once within its callback budget", func(t *testing.T) {
		original, err := service.Start(ctx, dataacquisition.Principal{Scope: scope}, uuid.NewString(), q, orgresource.FundingEnterprise, 5)
		require.NoError(t, err)
		original, err = repo.Discover(ctx, original, q.ASINs)
		require.NoError(t, err)
		items, err := repo.Items(ctx, original)
		require.NoError(t, err)
		require.Len(t, items, 1)
		fixture.authDelay = 300 * time.Millisecond
		t.Cleanup(func() { fixture.authDelay = 0 })
		before := fixture.executionChecks
		reservation, err := charges.Reserve(ctx, orgresource.ConsumerChargeIdentity{OrganizationID: scope.OrganizationID, Consumer: orgresource.ConsumerAmazonData, OperationID: items[0].ID})
		fixture.authDelay = 0
		require.NoError(t, err, "one bounded live authorization fits Resource's existing 500 ms callback")
		require.Equal(t, before+1, fixture.executionChecks)
		require.Equal(t, orgresource.ReservationReserved, reservation.State)
		_, err = repo.Cancel(ctx, scope, original.ID, uuid.NewString())
		require.NoError(t, err)
		require.NoError(t, service.Run(ctx, scope, original.ID))
		settled, err := charges.Lookup(ctx, reservation.Intent.Identity)
		require.NoError(t, err)
		require.Equal(t, orgresource.ReservationReleased, settled.State)
		readBalance()
		require.Equal(t, int64(3), bucket.Consumed)
		require.Zero(t, bucket.Reserved)
	})
	t.Run("archiving a running batch does not cancel original saves or duplicate charges", func(t *testing.T) {
		owner := collection.Scope{OrganizationID: "org", ActorID: "archive-creator", MemberID: "archive-grant"}
		query := dataacquisition.Query{Site: "us", Mode: "asin", ASINs: []string{"B000111111", "B000222222"}, Limit: 2}
		original, err := service.Start(ctx, dataacquisition.Principal{Scope: owner}, uuid.NewString(), query, orgresource.FundingEnterprise, 10)
		require.NoError(t, err)
		original, err = repo.Discover(ctx, original, query.ASINs)
		require.NoError(t, err)
		items, err := repo.Items(ctx, original)
		require.NoError(t, err)
		require.Len(t, items, 2)
		readBalance()
		before := bucket.Consumed
		fetches := fixture.fetches
		require.NoError(t, service.ProcessItem(ctx, original, items[0], ""))
		partial, err := service.Read(ctx, dataacquisition.Principal{Scope: owner}, original.ID)
		require.NoError(t, err)
		require.Equal(t, 1, partial.Saved)
		require.Equal(t, 1, partial.Pending)
		collections, err := collectionstore.NewRepository(ctx, productDB, func(*gorm.DB) (collectionstore.OwnPublisher, error) { return nil, collection.ErrUnavailable })
		require.NoError(t, err)
		batch, err := collections.ReadBatch(ctx, owner, partial.BatchID)
		require.NoError(t, err)
		mutation := collection.Mutation{Action: "archive_batch", BatchID: batch.ID, ExpectedRevision: batch.Revision}
		command := uuid.NewString()
		_, err = collections.Execute(ctx, collection.Command{Scope: owner, Key: command, OperationID: command, InputHash: collection.Digest(mutation), Mutation: mutation})
		require.NoError(t, err)
		require.NoError(t, service.ProcessItem(ctx, original, items[1], ""), "existing job must still save into its original archived batch")
		require.NoError(t, service.Run(ctx, owner, original.ID))
		require.NoError(t, service.Run(ctx, owner, original.ID))
		finished, err := service.Read(ctx, dataacquisition.Principal{Scope: owner}, original.ID)
		require.NoError(t, err)
		require.Equal(t, "SUCCEEDED", finished.State)
		require.Equal(t, 2, finished.Saved)
		require.Equal(t, 0, finished.Failed)
		require.Equal(t, int64(10), finished.ConfirmedFen)
		require.Equal(t, partial.BatchID, finished.BatchID)
		results, err := service.Results(ctx, dataacquisition.Principal{Scope: owner}, original.ID, "", 100, capturedResultReader{store})
		require.NoError(t, err)
		require.Len(t, results.Items, 2, "saved source results remain readable independently of collection visibility")
		_, err = collections.ReadBatch(ctx, owner, finished.BatchID)
		require.ErrorIs(t, err, collection.ErrNotFound)
		hidden, err := collections.ListItems(ctx, owner, "", collection.Query{Limit: 100})
		require.NoError(t, err)
		require.Empty(t, hidden.Items)
		var stored int64
		require.NoError(t, productDB.Raw("SELECT count(*) FROM product_collection_items WHERE organization_id=? AND actor_id=? AND batch_id=?", owner.OrganizationID, owner.ActorID, finished.BatchID).Scan(&stored).Error)
		require.Equal(t, int64(2), stored)
		require.Equal(t, fetches+2, fixture.fetches, "replays must not fetch saved rows again")
		readBalance()
		require.Equal(t, before+2, bucket.Consumed)
		require.Zero(t, bucket.Reserved)
	})
	t.Run("unknown availability saves original evidence and charges once", func(t *testing.T) {
		owner := collection.Scope{OrganizationID: "org", ActorID: "unknown-creator", MemberID: "unknown-grant"}
		provider := &unknownAvailabilityFixture{executionFixture: fixture}
		runner, err := dataacquisition.NewService(repo, fixture, provider, charges, fixture)
		require.NoError(t, err)
		query := dataacquisition.Query{Site: "us", Mode: "asin", ASINs: []string{"B000123456"}, Limit: 1, Fields: []string{"title", "availability", "price", "currency"}}
		readBalance()
		before, fetches := bucket.Consumed, fixture.fetches
		original, err := runner.Start(ctx, dataacquisition.Principal{Scope: owner}, uuid.NewString(), query, orgresource.FundingEnterprise, 5)
		require.NoError(t, err)
		require.NoError(t, runner.Run(ctx, owner, original.ID))
		require.NoError(t, runner.Run(ctx, owner, original.ID))
		finished, err := runner.Read(ctx, dataacquisition.Principal{Scope: owner}, original.ID)
		require.NoError(t, err)
		require.Equal(t, "SUCCEEDED", finished.State)
		require.Equal(t, 1, finished.Saved)
		require.Zero(t, finished.Failed)
		require.Equal(t, int64(5), finished.ConfirmedFen)
		results, err := runner.Results(ctx, dataacquisition.Principal{Scope: owner}, original.ID, "", 100, capturedResultReader{store})
		require.NoError(t, err)
		require.Len(t, results.Items, 1)
		require.Equal(t, "Incomplete public product", results.Items[0].Data["title"])
		require.Equal(t, "unknown", results.Items[0].Data["availability"])
		require.ElementsMatch(t, []string{"availability", "price", "currency"}, results.Items[0].Missing)
		require.NotContains(t, results.Items[0].Data, "price")
		require.NotContains(t, results.Items[0].Data, "currency")
		readBalance()
		require.Equal(t, before+1, bucket.Consumed)
		require.Zero(t, bucket.Reserved)
		require.Equal(t, fetches+1, fixture.fetches)
	})
}
