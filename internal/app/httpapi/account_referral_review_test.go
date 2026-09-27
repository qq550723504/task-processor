package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"task-processor/internal/ledger/money"
	economics "task-processor/internal/referraleconomics"
)

func TestPayoutDestinationEncryptionBindsReviewerInstructions(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	ciphertext, err := encryptPayoutDestination(key, "user-1", money.PayoutAlipay, "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	destination, err := decryptPayoutDestination(key, "user-1", money.PayoutAlipay, ciphertext)
	if err != nil || destination != "alice@example.com" {
		t.Fatalf("destination=%q err=%v", destination, err)
	}
	if _, err := decryptPayoutDestination(key, "user-2", money.PayoutAlipay, ciphertext); err == nil {
		t.Fatal("destination decrypted for a different subject")
	}
}

func TestWithdrawalReviewQueueIncludesClaimantAndPayoutInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	when := time.Date(2026, 9, 21, 1, 2, 3, 0, time.UTC)
	withdrawal := economics.Withdrawal{ID: "withdrawal-1", Referrer: "user-1", Currency: economics.CurrencyCNY, Method: economics.MethodAlipay, PayoutMethodID: "method-1", AmountMinor: 10000, Status: economics.WithdrawalRequested, Version: 1, CreatedAt: when, UpdatedAt: when}
	method := money.PayoutMethod{MethodID: "method-1", SubjectUserID: "user-1", Type: money.PayoutAlipay, DisplayName: "Primary Alipay", MaskedDestination: "a***com"}
	writeWithdrawalReviewQueueJSON(c, []economics.Withdrawal{withdrawal}, func(economics.Withdrawal) (money.PayoutMethod, string, error) {
		return method, "alice@example.com", nil
	})
	var response struct {
		Withdrawals []map[string]any `json:"withdrawals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Withdrawals) != 1 || response.Withdrawals[0]["referrer"] != "user-1" || response.Withdrawals[0]["payoutDestination"] != "alice@example.com" {
		t.Fatalf("review response=%s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "SecureReference") {
		t.Fatal("review response leaked persistence field name")
	}
}

func TestWriteReferralEarningsJSONIncludesOwnedLedgerEntries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	when := time.Date(2026, 9, 21, 1, 2, 3, 0, time.UTC)
	earnings := economics.Earnings{Referrer: "user-1", Currency: economics.CurrencyCNY, PendingMinor: 2000, AvailableMinor: 10000, ReservedMinor: 0, AdjustmentMinor: -500, Version: 3, UpdatedAt: when}
	entries := []economics.EarningsLedgerEntry{{EntryID: "entry-1", Referrer: "user-1", Currency: economics.CurrencyCNY, PaymentID: "payment-1", EntryType: "COMMISSION", AmountMinor: 10000, ReferenceID: "payment-1", OccurredAt: when}}

	writeReferralEarningsJSON(c, earnings, entries)
	var response struct {
		SchemaVersion string `json:"schemaVersion"`
		EntryLimit    int    `json:"entryLimit"`
		Entries       []struct {
			EntryID     string `json:"entryId"`
			PaymentID   string `json:"paymentId"`
			EntryType   string `json:"entryType"`
			AmountMinor string `json:"amountMinor"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.SchemaVersion != "referral-earnings-v1" || response.EntryLimit != 100 || len(response.Entries) != 1 || response.Entries[0].EntryID != "entry-1" || response.Entries[0].PaymentID != "payment-1" || response.Entries[0].EntryType != "COMMISSION" || response.Entries[0].AmountMinor != "10000" {
		t.Fatalf("earnings response=%s", w.Body.String())
	}
}

func TestWriteReferralRulesJSONUsesEconomicsContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	writeReferralRulesJSON(c)
	var response struct {
		SchemaVersion          string `json:"schemaVersion"`
		Currency               string `json:"currency"`
		CommissionRateBPS      int64  `json:"commissionRateBps"`
		SettlementPeriodDays   int    `json:"settlementPeriodDays"`
		MinimumWithdrawalMinor string `json:"minimumWithdrawalMinor"`
		WithdrawalReview       string `json:"withdrawalReview"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.SchemaVersion != "referral-rules-v1" || response.Currency != economics.CurrencyCNY || response.CommissionRateBPS != economics.CommissionRateBPS || response.SettlementPeriodDays != economics.SettlementPeriodDays || response.MinimumWithdrawalMinor != "10000" || response.WithdrawalReview != "manual" {
		t.Fatalf("rules response=%s", w.Body.String())
	}
}

func TestReferralMaturityLoopStopsWithServerShutdown(t *testing.T) {
	server := &http.Server{}
	called := make(chan struct{}, 4)
	startReferralMaturityLoop(server, func(context.Context, time.Time) error {
		called <- struct{}{}
		return nil
	}, time.Hour, logrus.New())
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("maturity sweep did not run immediately")
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
		t.Fatal("maturity sweep ran after server shutdown")
	case <-time.After(50 * time.Millisecond):
	}
}
