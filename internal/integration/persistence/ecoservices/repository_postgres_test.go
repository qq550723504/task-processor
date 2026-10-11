//go:build integration

package ecoservices

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	pg "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"strings"
	"sync"
	e "task-processor/internal/ecoservices"
	"testing"
	"time"
)

func postgresFixture(t *testing.T) (context.Context, *gorm.DB, *Repository, *e.Service) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pg.Run(ctx, "postgres:16-alpine", pg.WithDatabase("ecoservices"), pg.WithUsername("eco_installer"), pg.WithPassword(uuid.NewString()), pg.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	if err := Install(ctx, db); err != nil {
		t.Fatal(err)
	}
	password := uuid.NewString()
	if err := db.Exec("CREATE ROLE ecoservices_runtime LOGIN PASSWORD '" + password + "'; REVOKE CREATE,TEMP ON DATABASE ecoservices FROM PUBLIC; REVOKE ALL ON SCHEMA public FROM PUBLIC;").Error; err != nil {
		t.Fatal(err)
	}
	if err := GrantRuntime(ctx, db); err != nil {
		t.Fatal(err)
	}
	uri, _ := url.Parse(dsn)
	uri.User = url.UserPassword("ecoservices_runtime", password)
	runtime, err := gorm.Open(postgres.Open(uri.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	runtimePool, _ := runtime.DB()
	t.Cleanup(func() { _ = runtimePool.Close() })
	repo, err := NewRepository(ctx, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service, err := e.NewService(repo, nil, 180)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, db, repo, service
}

func TestQualificationOnlyPostgresRuntimeUsesExistingRestrictedOwner(t *testing.T) {
	_, _, repo, _ := postgresFixture(t)
	assertQualificationWorkflow(t, repo)
}
func TestEcoservicesPostgresLatePaymentReopensOriginalCancellation(t *testing.T) {
	for _, closedBeforePayment := range []bool{true, false} {
		t.Run(map[bool]string{true: "closed-in-E", false: "closure-projection-delayed"}[closedBeforePayment], func(t *testing.T) {
			ctx, _, r, s := postgresFixture(t)
			assertLatePaymentReopensOnlyOriginalCancellation(t, ctx, r, s, closedBeforePayment)
		})
	}
}

func TestEcoservicesPostgresMerchantOriginalClaimAndImmutableIntent(t *testing.T) {
	ctx, _, r, _ := postgresFixture(t)
	scope, in := merchantFixture(t, r, "provider")
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := r.CreateMerchantAttempt(ctx, in)
			if err == nil && a.Intent.ID != in.ID {
				err = e.ErrConflict
			}
			out <- err
		}()
	}
	wg.Wait()
	close(out)
	for err := range out {
		if err != nil {
			t.Fatal(err)
		}
	}
	claims := make(chan bool, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
			if err != nil {
				t.Error(err)
			}
			claims <- claimed
		}()
	}
	wg.Wait()
	close(claims)
	count := 0
	for claimed := range claims {
		if claimed {
			count++
		}
	}
	if count != 1 {
		t.Fatal("concurrent human entries acquired multiple dispatch claims")
	}
	a, err := r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.db.Model(&merchantIntentRow{}).Where("id=?", a.Intent.ID).Update("payload", []byte("changed-private-intent")).Error; err == nil {
		t.Fatal("runtime can modify immutable onboarding payload")
	}
	if err = r.ReleaseMerchantClaim(ctx, a); err != nil {
		t.Fatal(err)
	}
	newer, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !claimed || newer.ClaimToken == a.ClaimToken {
		t.Fatal("next human original claim not fenced", err)
	}
	if err = r.SaveMerchantMedia(ctx, a, in.FileIDs[0], "late-media"); !errors.Is(err, e.ErrConflict) {
		t.Fatal("stale writer overwrote new original claim", err)
	}
}
func TestEcoservicesPostgresMerchantCorrectionAtomicCurrentPointerAndProof(t *testing.T) {
	ctx, _, r, _ := postgresFixture(t)
	scope, in := merchantFixture(t, r, "provider")
	if _, err := r.CreateMerchantAttempt(ctx, in); err != nil {
		t.Fatal(err)
	}
	old, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	for _, id := range in.FileIDs {
		if err = r.SaveMerchantMedia(ctx, old, id, "media-"+id); err != nil {
			t.Fatal(err)
		}
	}
	old.DispatchActorID = scope.ActorID
	if err = r.MarkMerchantDispatched(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err = r.ObserveMerchant(ctx, old, merchantObservation(old, "REJECTED", "", ""), []byte("verified-original-rejection")); err != nil {
		t.Fatal(err)
	}
	if err = r.ReleaseMerchantClaim(ctx, old); err != nil {
		t.Fatal(err)
	}
	version := merchantApplication(t, r, in.ApplicationID).Version
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			corrected := in
			corrected.ID = uuid.NewString()
			corrected.Key = uuid.NewString()
			corrected.Fingerprint = corrected.ID
			corrected.SealedDetails = []byte("sealed-correction-" + corrected.ID)
			corrected.ApplicationVersion = version
			corrected.ExpectedRevisionVersion = 1
			_, err := r.CreateMerchantAttempt(ctx, corrected)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, e.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("correction CAS has multiple winners", successes, conflicts)
	}
	app := merchantApplication(t, r, in.ApplicationID)
	if app.Version != version+1 || app.CurrentMerchantRevisionVersion != 2 {
		t.Fatal("application/revision not atomic")
	}
	current, claimed, err := r.ClaimMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	if current.Intent.ID != in.ID || current.Intent.OutRequestNo != in.OutRequestNo || current.Revision.ID == old.Revision.ID {
		t.Fatal("original identity replaced")
	}
	if err = r.SaveMerchantMedia(ctx, old, in.FileIDs[0], "stale"); !errors.Is(err, e.ErrConflict) {
		t.Fatal("old worker wrote current", err)
	}
	for _, id := range in.FileIDs {
		if err = r.SaveMerchantMedia(ctx, current, id, "new-media-"+id); err != nil {
			t.Fatal(err)
		}
	}
	current.DispatchActorID = scope.ActorID
	if err = r.MarkMerchantDispatched(ctx, current); err != nil {
		t.Fatal(err)
	}
	proof := e.MerchantSubmissionAcceptance{RevisionID: current.Revision.ID, RevisionVersion: 2, DetailsFingerprint: current.Revision.Input.Fingerprint, Profile: in.Profile, OutRequestNo: in.OutRequestNo, ChannelApplicationID: "1001", VerificationVersion: "verified-current-sdk", SignedResponseDigest: strings.Repeat("a", 64)}
	if err = r.SaveMerchantAcceptance(ctx, current, proof, []byte("sealed-current-acceptance")); err != nil {
		t.Fatal(err)
	}
	current, err = r.ReadMerchantAttempt(ctx, scope, in.ApplicationID)
	if err != nil || current.Acceptance == nil {
		t.Fatal("durable acceptance lost", err)
	}
	observation := current.BindQuery(merchantObservation(current, "FINISH", "SIGNED", "1900000011"))
	if err = r.ObserveMerchant(ctx, current, observation, []byte("verified-current-query")); err != nil {
		t.Fatal(err)
	}
	if merchantApplication(t, r, in.ApplicationID).State != "ACTIVE" {
		t.Fatal("accepted same-number correction cannot qualify")
	}
	for _, kind := range []string{"MERCHANT_DETAILS", "MERCHANT_SUBMISSION_ACCEPTANCE"} {
		if err = r.db.Model(&versionRow{}).Where("id=? AND kind=?", in.ID, kind).Update("payload", []byte("tampered")).Error; err == nil {
			t.Fatal("runtime can overwrite immutable revision/proof", kind)
		}
	}
	var count int64
	r.db.Model(&merchantIntentRow{}).Where("application_id=?", in.ApplicationID).Count(&count)
	if count != 1 {
		t.Fatal("second original created")
	}
	r.db.Model(&versionRow{}).Where("id=? AND kind=?", in.ID, "MERCHANT_DETAILS").Count(&count)
	if count != 2 {
		t.Fatal("losing correction left partial version", count)
	}
}
func TestEcoservicesPostgresRecoveryWorkersRotatePastUnpaidHead(t *testing.T) {
	ctx, _, r, _ := postgresFixture(t)
	for n := 0; n < 21; n++ {
		c := e.FinancialCommand{ID: uuid.NewString(), RequestID: uuid.NewString(), OrderID: uuid.NewString(), Kind: "CREATE_PURCHASE", State: "PROCESSING"}
		raw, _ := json.Marshal(c)
		if err := r.db.Create(&financialRow{ID: c.ID, RequestID: c.RequestID, OrderID: c.OrderID, Kind: c.Kind, Fingerprint: e.Fingerprint(c), Payload: raw, State: c.State, DispatchAdmitted: true, CreatedAt: time.Now().UTC()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	selected := make(chan []e.FinancialCommand, 2)
	var wg sync.WaitGroup
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := r.PendingFinancialCommands(ctx, 20)
			if err != nil {
				t.Error(err)
			}
			selected <- c
		}()
	}
	wg.Wait()
	close(selected)
	seen := map[string]bool{}
	for list := range selected {
		for _, c := range list {
			if seen[c.ID] {
				t.Fatal("workers selected same due command instead of leased rotation")
			}
			seen[c.ID] = true
		}
	}
	if len(seen) != 21 {
		t.Fatal("unpaid first page prevented later paid command selection")
	}
}
func TestEcoservicesPostgresStartCancelExclusiveAndCounts(t *testing.T) {
	ctx, db, repo, service := postgresFixture(t)
	app := e.Application{ID: uuid.NewString(), OrganizationID: "provider", CompanyName: "qualified fixture", State: "ACTIVE", Version: 1, MerchantID: "original-sub", AgreementAccepted: true, AgreementVersion: e.PolicyVersion, OnboardingState: "FINISH"}
	if err := db.Create(applicationRecord(app)).Error; err != nil {
		t.Fatal(err)
	}
	request, _ := checkoutAdmissionFixture(t, repo)
	id := request.ID
	request.State = "PAID_READY"
	request.PaymentReceiptID = "verified-original-payment"
	if err := db.Save(requestRecord(request)).Error; err != nil {
		t.Fatal(err)
	}
	// This test exercises the E request lock; current canonical funds are an
	// explicit supplied proof, rather than a missing trading dependency.
	var err error
	service, err = e.NewService(repo, originalPaymentRecovery{paidOrder: request.OrderID}, 180)
	if err != nil {
		t.Fatal(err)
	}
	commands := []e.Command{{Scope: e.Scope{OrganizationID: "buyer", ActorID: "b"}, Kind: "cancel", Key: uuid.NewString(), ID: id, Version: 1}, {Scope: e.Scope{OrganizationID: "provider", ActorID: "p"}, Kind: "start", Key: uuid.NewString(), ID: id, Version: 1}}
	var wg sync.WaitGroup
	successes := make(chan e.Result, 2)
	failures := make(chan error, 2)
	for _, c := range commands {
		wg.Go(func() {
			got, err := service.Mutate(ctx, c)
			if err != nil {
				failures <- err
			} else {
				successes <- got
			}
		})
	}
	wg.Wait()
	close(successes)
	close(failures)
	if len(successes) != 1 || len(failures) != 1 {
		t.Fatalf("start/cancel both accepted: success=%d failure=%d", len(successes), len(failures))
	}
	for err := range failures {
		if !errors.Is(err, e.ErrConflict) {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		fixture := request
		fixture.ID = uuid.NewString()
		fixture.OrderID = uuid.NewString()
		fixture.State = "SERVICING"
		if err := db.Create(requestRecord(fixture)).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "buyer", ActorID: "b"}, Kind: "requests", Page: 1, PageSize: 1})
	if err != nil || len(page.Requests) != 1 || page.Total != 4 || page.Counts["SERVICING"] < 3 {
		t.Fatalf("summary counted only visible page: %+v %v", page, err)
	}
	foreign, err := service.Read(ctx, e.Query{Scope: e.Scope{OrganizationID: "stranger", ActorID: "x"}, Kind: "requests", ID: id, Page: 1, PageSize: 20})
	if !errors.Is(err, e.ErrNotFound) {
		t.Fatalf("cross-org request access: %+v %v", foreign, err)
	}
	if err := db.Exec("GRANT UPDATE(result) ON ecoservices_operations TO ecoservices_runtime").Error; err != nil {
		t.Fatal(err)
	}
	if err := VerifyRuntime(ctx, repo.db); !errors.Is(err, e.ErrUnavailable) {
		t.Fatalf("immutable operation mutation allowed: %v", err)
	}
}
func TestEcoservicesPostgresEachFinancialStageHonorsNewDispute(t *testing.T) {
	ctx, db, repo, _ := postgresFixture(t)
	service, err := e.NewService(repo, &refundCapacityTrading{remaining: 101}, 180)
	if err != nil {
		t.Fatal(err)
	}
	app := e.Application{ID: uuid.NewString(), OrganizationID: "provider", CompanyName: "qualified fixture", State: "ACTIVE", Version: 1, MerchantID: "original-sub", AgreementAccepted: true, OnboardingState: "FINISH"}
	if err := db.Create(applicationRecord(app)).Error; err != nil {
		t.Fatal(err)
	}
	request := e.Request{ID: uuid.NewString(), BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "ACCEPTED", AcceptanceID: "acceptance-proof", Version: 1, OrderID: uuid.NewString(), PaymentReceiptID: "controlled-receipt", Quote: &e.Quote{AmountMinor: 101, DeliveryDays: 7, Version: 1}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(requestRecord(request)).Error; err != nil {
		t.Fatal(err)
	}
	command := e.FinancialCommand{ID: "acceptance-proof", RequestID: request.ID, OrderID: request.OrderID, Kind: "SETTLE", SourceProofID: request.AcceptanceID, ActorID: "buyer", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", MerchantID: "original-sub", Quote: *request.Quote, AmountMinor: 101, PolicyVersion: e.PolicyVersion, State: "PENDING"}
	payload, _ := json.Marshal(command)
	if err := db.Create(&financialRow{ID: command.ID, RequestID: command.RequestID, OrderID: command.OrderID, Kind: command.Kind, Fingerprint: e.Fingerprint(command), Payload: payload, State: "PENDING", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	command.DispatchOperationID = "original-share"
	if _, err := repo.AdmitFinancialCommand(ctx, command); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Mutate(ctx, e.Command{Scope: e.Scope{OrganizationID: "buyer", ActorID: "b"}, Kind: "refund_propose", ID: request.ID, Key: uuid.NewString(), Version: 1, RefundAmountMinor: 2, Reason: "partial dispute"}); err != nil {
		t.Fatal(err)
	}
	command.DispatchOperationID = "new-normal-finish"
	if _, err := repo.AdmitFinancialCommand(ctx, command); !errors.Is(err, e.ErrConflict) {
		t.Fatalf("new finish ignored dispute fence: %v", err)
	}
	command.DispatchOperationID = "original-share"
	if _, err := repo.AdmitFinancialCommand(ctx, command); err != nil {
		t.Fatalf("existing original in-flight fact lost admission: %v", err)
	}
}

func TestEcoservicesPostgresCheckoutReplayHonorsCommittedCancellation(t *testing.T) {
	ctx, _, repo, service := postgresFixture(t)
	assertCheckoutReplayRejectsCommittedCancellation(t, ctx, repo, service)
}

func TestEcoservicesPostgresConcurrentRejectedApplicationCorrections(t *testing.T) {
	ctx, _, repo, service := postgresFixture(t)
	rejected, original, _ := correctedApplicationCommand(t, repo, service)
	commands := []e.Command{original, original}
	commands[1].Key = uuid.NewString()
	results := make(chan e.Result, 2)
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for _, c := range commands {
		wg.Go(func() {
			result, err := service.Mutate(ctx, c)
			if err != nil {
				failures <- err
			} else {
				results <- result
			}
		})
	}
	wg.Wait()
	close(results)
	close(failures)
	if len(results) != 1 || len(failures) != 1 {
		t.Fatalf("corrections both changed the original: successes=%d failures=%d", len(results), len(failures))
	}
	for err := range failures {
		if !errors.Is(err, e.ErrConflict) {
			t.Fatal(err)
		}
	}
	for result := range results {
		if result.Application.ID != rejected.ID || result.Application.Version != rejected.Version+1 {
			t.Fatal("correction changed original identity or skipped versions", result.Application)
		}
	}
	var applications, versions int64
	if err := repo.db.Model(&applicationRow{}).Where("organization_id=?", original.Scope.OrganizationID).Count(&applications).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Model(&versionRow{}).Where("id=? AND kind='APPLICATION'", rejected.ID).Count(&versions).Error; err != nil {
		t.Fatal(err)
	}
	if applications != 1 || versions != 3 {
		t.Fatalf("lost history or duplicated original: applications=%d versions=%d", applications, versions)
	}
}
