package servicepayments

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	b "task-processor/internal/commercial/billing"
	"testing"
	"time"
)

type ingressFixture struct {
	order       b.ServicePurchaseOrder
	observation b.ServicePaymentObservation
	recorded    bool
	wakeFail    bool
	foreign     bool
}

func (f *ingressFixture) VerifyServiceNotification(*http.Request) (b.ServicePaymentObservation, error) {
	p := f.observation
	if f.foreign {
		p.ProviderMerchantID = "other"
	}
	return p, nil
}
func (f *ingressFixture) ReadServicePurchaseByTrade(context.Context, string) (b.ServicePurchaseOrder, error) {
	return f.order, nil
}
func (f *ingressFixture) RecordServicePaymentObservation(context.Context, b.ServicePurchaseOrder, b.ServicePaymentObservation) error {
	f.recorded = true
	return nil
}
func (f *ingressFixture) WakeOriginalServicePurchase(context.Context, string) error {
	if !f.recorded || f.wakeFail {
		return errors.New("recovery unavailable")
	}
	return nil
}
func TestServiceNotificationNeverAcknowledgesMissingDurableRecovery(t *testing.T) {
	p, o, _ := wechatServiceFixture()
	f := &ingressFixture{order: o, observation: p.paymentObservation(o, "PAID", "transaction", 100, time.Now()), wakeFail: true}
	i := NotificationIngress{f, f, f}
	request := httptest.NewRequest("POST", "/notify", nil)
	if err := i.AcceptNotification(request); err == nil || !f.recorded {
		t.Fatal("acknowledged missing wake or skipped durable inbox")
	}
	f.wakeFail = false
	if err := i.AcceptNotification(request); err != nil {
		t.Fatal(err)
	}
	f.recorded = false
	f.foreign = true
	if err := i.AcceptNotification(request); err == nil || f.recorded {
		t.Fatal("foreign merchant reached original payment inbox")
	}
}
