package openmeter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

const (
	pocMeterPollInterval = 250 * time.Millisecond
	pocMeterPollTimeout  = 30 * time.Second
)

type pocFixture struct {
	Environment   pocEnvironment
	Names         pocNames
	SDK           *openmeterapi.Client
	Meters        []*openmeterapi.Meter
	Features      []*openmeterapi.Feature
	Customers     []*openmeterapi.Customer
	Plan          *openmeterapi.Plan
	Subscriptions []*openmeterapi.BillingSubscription
}

type pocRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn pocRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

const pocConflictBody = `{"status":409,"title":"Conflict","detail":"fixture key already exists"}`

type pocSDKStep struct {
	Method string
	Path   string
	Query  url.Values
	Status int
	Body   string
}

type pocSequenceTransport struct {
	steps    []pocSDKStep
	next     int
	failures []string
}

func (transport *pocSequenceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.next >= len(transport.steps) {
		transport.failures = append(transport.failures, fmt.Sprintf("unexpected request %s %s", request.Method, request.URL.String()))
		return nil, errors.New("unexpected OpenMeter SDK request")
	}

	step := transport.steps[transport.next]
	transport.next++
	if request.Method != step.Method {
		transport.failures = append(transport.failures, fmt.Sprintf("request %d method = %s, want %s", transport.next, request.Method, step.Method))
	}
	if request.URL.Path != step.Path {
		transport.failures = append(transport.failures, fmt.Sprintf("request %d path = %s, want %s", transport.next, request.URL.Path, step.Path))
	}
	actualQuery := request.URL.Query()
	queriesEqual := len(actualQuery) == 0 && len(step.Query) == 0
	if !queriesEqual && !reflect.DeepEqual(actualQuery, step.Query) {
		transport.failures = append(transport.failures, fmt.Sprintf("request %d query = %v, want %v", transport.next, request.URL.Query(), step.Query))
	}

	return &http.Response{
		StatusCode: step.Status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(step.Body)),
		Request:    request,
	}, nil
}

func (transport *pocSequenceTransport) Verify(t *testing.T) {
	t.Helper()
	for _, failure := range transport.failures {
		t.Error(failure)
	}
	if transport.next != len(transport.steps) {
		t.Errorf("official OpenMeter SDK made %d requests, want %d", transport.next, len(transport.steps))
	}
}

func newPoCSequenceSDK(t *testing.T, steps ...pocSDKStep) (*openmeterapi.Client, *pocSequenceTransport) {
	t.Helper()
	transport := &pocSequenceTransport{steps: steps}
	sdk, err := openmeterapi.New("http://openmeter.invalid/api/v3", openmeterapi.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("construct official OpenMeter SDK: %v", err)
	}
	return sdk, transport
}

func marshalPoCTestJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal official OpenMeter test response: %v", err)
	}
	return string(encoded)
}

func pocMeterRequests(names pocNames) []openmeterapi.CreateMeterRequest {
	quantity := "$.quantity"
	return []openmeterapi.CreateMeterRequest{
		{
			Name:        names.StudioMeterKey,
			Key:         names.StudioMeterKey,
			Aggregation: openmeterapi.MeterAggregationCount,
			EventType:   eventTypeForMetric(MetricStudioDesignJobsSucceeded),
		},
		{
			Name:        names.SheinMeterKey,
			Key:         names.SheinMeterKey,
			Aggregation: openmeterapi.MeterAggregationCount,
			EventType:   eventTypeForMetric(MetricSheinDraftsSucceeded),
		},
		{
			Name:          names.StorageMeterKey,
			Key:           names.StorageMeterKey,
			Aggregation:   openmeterapi.MeterAggregationLatest,
			EventType:     eventTypeForMetric(MetricStorageBytesCurrent),
			ValueProperty: &quantity,
		},
	}
}

func pocCustomerRequests(names pocNames) []openmeterapi.CreateCustomerRequest {
	currency := "USD"
	return []openmeterapi.CreateCustomerRequest{
		{
			Name:     names.CustomerAKey,
			Key:      names.CustomerAKey,
			Currency: &currency,
			UsageAttribution: &openmeterapi.CustomerUsageAttribution{
				SubjectKeys: []string{names.SubjectA},
			},
		},
		{
			Name:     names.CustomerBKey,
			Key:      names.CustomerBKey,
			Currency: &currency,
			UsageAttribution: &openmeterapi.CustomerUsageAttribution{
				SubjectKeys: []string{names.SubjectB},
			},
		},
	}
}

func pocPlanRequest(names pocNames, studioFeatureID, sheinFeatureID, storageFeatureID string) (openmeterapi.CreatePlanRequest, error) {
	freePrice, err := openmeterapi.PriceFromPriceFree(openmeterapi.PriceFree{})
	if err != nil {
		return openmeterapi.CreatePlanRequest{}, fmt.Errorf("construct OpenMeter free price: %w", err)
	}

	billingCadence := "P1M"
	rateCardInputs := []struct {
		key       string
		featureID string
		limit     float64
	}{
		{key: names.StudioFeatureKey, featureID: studioFeatureID, limit: 5},
		{key: names.SheinFeatureKey, featureID: sheinFeatureID, limit: 3},
		{key: names.StorageFeatureKey, featureID: storageFeatureID, limit: 10 * 1024 * 1024},
	}
	rateCards := make([]openmeterapi.RateCardInput, 0, len(rateCardInputs))
	for _, input := range rateCardInputs {
		softLimit := false
		limit := input.limit
		entitlement, err := openmeterapi.RateCardEntitlementFromRateCardMeteredEntitlement(openmeterapi.RateCardMeteredEntitlement{
			IsSoftLimit: &softLimit,
			Limit:       &limit,
		})
		if err != nil {
			return openmeterapi.CreatePlanRequest{}, fmt.Errorf("construct OpenMeter metered entitlement for %q: %w", input.key, err)
		}
		rateCards = append(rateCards, openmeterapi.RateCardInput{
			Name:           input.key,
			Key:            input.key,
			Feature:        &openmeterapi.FeatureReference{ID: input.featureID},
			BillingCadence: &billingCadence,
			Price:          freePrice,
			Entitlement:    &entitlement,
		})
	}

	return openmeterapi.CreatePlanRequest{
		Name:           names.PlanKey,
		Key:            names.PlanKey,
		Currency:       "USD",
		BillingCadence: billingCadence,
		Phases: []openmeterapi.PlanPhaseInput{
			{
				Name:      names.PhaseKey,
				Key:       names.PhaseKey,
				RateCards: rateCards,
			},
		},
	}, nil
}

func pocSubscriptionRequest(customerID, planID string) openmeterapi.SubscriptionCreate {
	return openmeterapi.SubscriptionCreate{
		Customer: openmeterapi.SubscriptionChangeCustomer{ID: &customerID},
		Plan:     openmeterapi.SubscriptionChangePlan{ID: &planID},
	}
}

func requirePoCFixture(t *testing.T) *pocFixture {
	t.Helper()
	environment, err := loadPoCEnvironment()
	if err != nil {
		t.Fatalf("load OpenMeter PoC environment: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)
	fixture, enabled, err := loadPoCFixture(ctx, environment, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		t.Fatalf("set up OpenMeter PoC fixture: %v", err)
	}
	if !enabled {
		t.Skip("OpenMeter PoC is disabled; set OPENMETER_POC=1 to opt in")
	}
	return fixture
}

func loadPoCFixture(ctx context.Context, environment pocEnvironment, httpClient *http.Client) (*pocFixture, bool, error) {
	if !environment.Enabled {
		return nil, false, nil
	}

	options := []openmeterapi.Option{openmeterapi.WithHTTPClient(httpClient)}
	if environment.APIKey != "" {
		options = append(options, openmeterapi.WithToken(environment.APIKey))
	}
	sdk, err := openmeterapi.New(environment.BaseURL, options...)
	if err != nil {
		return nil, true, fmt.Errorf("construct OpenMeter PoC SDK: %w", err)
	}
	fixture, err := setupPoCFixture(ctx, sdk, environment)
	if err != nil {
		return nil, true, err
	}
	return fixture, true, nil
}

func setupPoCFixture(ctx context.Context, sdk *openmeterapi.Client, environment pocEnvironment) (*pocFixture, error) {
	names := pocNamesForRunID(environment.RunID)
	fixture := &pocFixture{Environment: environment, Names: names, SDK: sdk}

	for _, request := range pocMeterRequests(names) {
		meter, err := ensurePoCMeter(ctx, sdk, request)
		if err != nil {
			return nil, err
		}
		fixture.Meters = append(fixture.Meters, meter)
	}

	featureRequests := []openmeterapi.CreateFeatureRequest{
		{Name: names.StudioFeatureKey, Key: names.StudioFeatureKey, Meter: &openmeterapi.FeatureMeterReferenceInput{ID: fixture.Meters[0].ID}},
		{Name: names.SheinFeatureKey, Key: names.SheinFeatureKey, Meter: &openmeterapi.FeatureMeterReferenceInput{ID: fixture.Meters[1].ID}},
		{Name: names.StorageFeatureKey, Key: names.StorageFeatureKey, Meter: &openmeterapi.FeatureMeterReferenceInput{ID: fixture.Meters[2].ID}},
	}
	for _, request := range featureRequests {
		feature, err := ensurePoCFeature(ctx, sdk, request)
		if err != nil {
			return nil, err
		}
		fixture.Features = append(fixture.Features, feature)
	}

	for _, request := range pocCustomerRequests(names) {
		customer, err := ensurePoCCustomer(ctx, sdk, request)
		if err != nil {
			return nil, err
		}
		fixture.Customers = append(fixture.Customers, customer)
	}

	planRequest, err := pocPlanRequest(names, fixture.Features[0].ID, fixture.Features[1].ID, fixture.Features[2].ID)
	if err != nil {
		return nil, err
	}
	fixture.Plan, err = ensurePoCPlan(ctx, sdk, planRequest)
	if err != nil {
		return nil, err
	}

	for _, customer := range fixture.Customers {
		subscription, err := ensurePoCSubscription(ctx, sdk, pocSubscriptionRequest(customer.ID, fixture.Plan.ID))
		if err != nil {
			return nil, err
		}
		fixture.Subscriptions = append(fixture.Subscriptions, subscription)
	}

	return fixture, nil
}

func ensurePoCMeter(ctx context.Context, sdk *openmeterapi.Client, request openmeterapi.CreateMeterRequest) (*openmeterapi.Meter, error) {
	meter, err := sdk.Meters.Create(ctx, request)
	if err != nil {
		if !isPoCConflict(err) {
			return nil, fmt.Errorf("create OpenMeter meter %q: %w", request.Key, err)
		}
		meter, err = findPoCMeterByKey(ctx, sdk, request.Key)
		if err != nil {
			return nil, fmt.Errorf("fetch conflicting OpenMeter meter %q: %w", request.Key, err)
		}
	}
	if err := validatePoCMeter(request, meter); err != nil {
		return nil, err
	}
	visible, err := waitForPoCMeter(ctx, sdk, meter.ID)
	if err != nil {
		return nil, fmt.Errorf("wait for OpenMeter meter %q visibility: %w", request.Key, err)
	}
	if err := validatePoCMeter(request, visible); err != nil {
		return nil, err
	}
	return visible, nil
}

func findPoCMeterByKey(ctx context.Context, sdk *openmeterapi.Client, key string) (*openmeterapi.Meter, error) {
	page, err := sdk.Meters.List(ctx, openmeterapi.MeterListParams{Filter: &openmeterapi.MeterFilter{Key: &openmeterapi.StringFilter{Eq: &key}}})
	if err != nil {
		return nil, err
	}
	if len(page.Data) != 1 {
		return nil, fmt.Errorf("expected exactly one meter for key %q, got %d", key, len(page.Data))
	}
	return &page.Data[0], nil
}

func waitForPoCMeter(ctx context.Context, sdk *openmeterapi.Client, meterID string) (*openmeterapi.Meter, error) {
	return waitForPoCMeterWithin(ctx, sdk, meterID, pocMeterPollInterval, pocMeterPollTimeout)
}

func waitForPoCMeterWithin(ctx context.Context, sdk *openmeterapi.Client, meterID string, interval, timeout time.Duration) (*openmeterapi.Meter, error) {
	pollContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastResult := "no visibility request completed"
	for {
		meter, err := sdk.Meters.Get(pollContext, meterID)
		if err == nil {
			return meter, nil
		}
		if pollContext.Err() != nil {
			return nil, meterPollContextError(ctx, pollContext, timeout, lastResult)
		}
		apiErr, isAPIError := openmeterapi.AsAPIError(err)
		if !isAPIError || apiErr.StatusCode != http.StatusNotFound {
			return nil, err
		}
		lastResult = err.Error()

		select {
		case <-pollContext.Done():
			return nil, meterPollContextError(ctx, pollContext, timeout, lastResult)
		case <-ticker.C:
		}
	}
}

func meterPollContextError(parentContext, pollContext context.Context, timeout time.Duration, lastResult string) error {
	if parentContext.Err() != nil {
		return fmt.Errorf("meter visibility context ended after last result %q: %w", lastResult, parentContext.Err())
	}
	return fmt.Errorf("meter visibility timed out after %s; last result: %s: %w", timeout, lastResult, pollContext.Err())
}

func ensurePoCFeature(ctx context.Context, sdk *openmeterapi.Client, request openmeterapi.CreateFeatureRequest) (*openmeterapi.Feature, error) {
	feature, err := sdk.Features.Create(ctx, request)
	if err != nil {
		if !isPoCConflict(err) {
			return nil, fmt.Errorf("create OpenMeter feature %q: %w", request.Key, err)
		}
		feature, err = findPoCFeatureByKey(ctx, sdk, request.Key)
		if err != nil {
			return nil, fmt.Errorf("fetch conflicting OpenMeter feature %q: %w", request.Key, err)
		}
	}
	if err := validatePoCFeature(request, feature); err != nil {
		return nil, err
	}
	return feature, nil
}

func findPoCFeatureByKey(ctx context.Context, sdk *openmeterapi.Client, key string) (*openmeterapi.Feature, error) {
	page, err := sdk.Features.List(ctx, openmeterapi.FeatureListParams{Filter: &openmeterapi.FeatureFilter{Key: &openmeterapi.StringFilter{Eq: &key}}})
	if err != nil {
		return nil, err
	}
	if len(page.Data) != 1 {
		return nil, fmt.Errorf("expected exactly one feature for key %q, got %d", key, len(page.Data))
	}
	return &page.Data[0], nil
}

func ensurePoCCustomer(ctx context.Context, sdk *openmeterapi.Client, request openmeterapi.CreateCustomerRequest) (*openmeterapi.Customer, error) {
	customer, err := sdk.Customers.Create(ctx, request)
	if err != nil {
		if !isPoCConflict(err) {
			return nil, fmt.Errorf("create OpenMeter customer %q: %w", request.Key, err)
		}
		customer, err = findPoCCustomerByKey(ctx, sdk, request.Key)
		if err != nil {
			return nil, fmt.Errorf("fetch conflicting OpenMeter customer %q: %w", request.Key, err)
		}
	}
	if err := validatePoCCustomer(request, customer); err != nil {
		return nil, err
	}
	return customer, nil
}

func findPoCCustomerByKey(ctx context.Context, sdk *openmeterapi.Client, key string) (*openmeterapi.Customer, error) {
	page, err := sdk.Customers.List(ctx, openmeterapi.CustomerListParams{Filter: &openmeterapi.CustomerFilter{Key: &openmeterapi.StringFilter{Eq: &key}}})
	if err != nil {
		return nil, err
	}
	if len(page.Data) != 1 {
		return nil, fmt.Errorf("expected exactly one customer for key %q, got %d", key, len(page.Data))
	}
	return &page.Data[0], nil
}

func ensurePoCPlan(ctx context.Context, sdk *openmeterapi.Client, request openmeterapi.CreatePlanRequest) (*openmeterapi.Plan, error) {
	page, err := sdk.Plans.List(ctx, openmeterapi.PlanListParams{Filter: &openmeterapi.PlanFilter{Key: &openmeterapi.StringFilter{Eq: &request.Key}}})
	if err != nil {
		return nil, fmt.Errorf("list OpenMeter plans for key %q: %w", request.Key, err)
	}
	if len(page.Data) > 1 {
		return nil, fmt.Errorf("expected at most one OpenMeter plan for key %q, got %d", request.Key, len(page.Data))
	}
	var plan *openmeterapi.Plan
	if len(page.Data) == 1 {
		plan = &page.Data[0]
	} else {
		plan, err = sdk.Plans.Create(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("create OpenMeter plan %q: %w", request.Key, err)
		}
	}
	if err := validatePoCPlan(request, plan); err != nil {
		return nil, err
	}
	if plan.Status == openmeterapi.PlanStatusDraft {
		plan, err = sdk.Plans.Publish(ctx, plan.ID)
		if err != nil {
			return nil, fmt.Errorf("publish OpenMeter plan %q: %w", request.Key, err)
		}
	}
	if plan.Status != openmeterapi.PlanStatusActive {
		return nil, fmt.Errorf("OpenMeter plan %q status = %q after publish, want %q", request.Key, plan.Status, openmeterapi.PlanStatusActive)
	}
	if err := validatePoCPlan(request, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func ensurePoCSubscription(ctx context.Context, sdk *openmeterapi.Client, request openmeterapi.SubscriptionCreate) (*openmeterapi.BillingSubscription, error) {
	existing, err := findPoCSubscriptions(ctx, sdk, *request.Customer.ID, *request.Plan.ID)
	if err != nil {
		return nil, err
	}
	if len(existing) == 1 {
		if err := validatePoCSubscription(request, &existing[0]); err != nil {
			return nil, err
		}
		return &existing[0], nil
	}
	if len(existing) > 1 {
		return nil, fmt.Errorf("expected at most one subscription for customer %q and plan %q, got %d", *request.Customer.ID, *request.Plan.ID, len(existing))
	}

	subscription, err := sdk.Subscriptions.Create(ctx, request)
	if err != nil {
		if !isPoCConflict(err) {
			return nil, fmt.Errorf("create OpenMeter subscription for customer %q: %w", *request.Customer.ID, err)
		}
		existing, err = findPoCSubscriptions(ctx, sdk, *request.Customer.ID, *request.Plan.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch conflicting OpenMeter subscription for customer %q: %w", *request.Customer.ID, err)
		}
		if len(existing) != 1 {
			return nil, fmt.Errorf("expected exactly one conflicting subscription for customer %q and plan %q, got %d", *request.Customer.ID, *request.Plan.ID, len(existing))
		}
		subscription = &existing[0]
	}
	if err := validatePoCSubscription(request, subscription); err != nil {
		return nil, err
	}
	return subscription, nil
}

func findPoCSubscriptions(ctx context.Context, sdk *openmeterapi.Client, customerID, planID string) ([]openmeterapi.BillingSubscription, error) {
	page, err := sdk.Subscriptions.List(ctx, openmeterapi.BillingSubscriptionListParams{Filter: &openmeterapi.BillingSubscriptionFilter{
		CustomerID: &openmeterapi.StringExactFilter{Eq: &customerID},
		PlanID:     &openmeterapi.StringExactFilter{Eq: &planID},
	}})
	if err != nil {
		return nil, fmt.Errorf("list OpenMeter subscriptions for customer %q and plan %q: %w", customerID, planID, err)
	}
	return page.Data, nil
}

func validatePoCMeter(request openmeterapi.CreateMeterRequest, meter *openmeterapi.Meter) error {
	if meter == nil {
		return errors.New("OpenMeter meter is nil")
	}
	wantDimensions := map[string]string(nil)
	if request.Dimensions != nil {
		wantDimensions = *request.Dimensions
	}
	if meter.Key != request.Key || meter.Name != request.Name || meter.Aggregation != request.Aggregation || meter.EventType != request.EventType || !equalStringPointers(meter.ValueProperty, request.ValueProperty) || !equalTimePointers(meter.EventsFrom, request.EventsFrom) || !equalStringMaps(meter.Dimensions, wantDimensions) {
		return fmt.Errorf("OpenMeter meter %q has incompatible configuration", request.Key)
	}
	return nil
}

func validatePoCFeature(request openmeterapi.CreateFeatureRequest, feature *openmeterapi.Feature) error {
	if feature == nil || request.Meter == nil || feature.Meter == nil {
		return fmt.Errorf("OpenMeter feature %q is missing its meter reference", request.Key)
	}
	if feature.Key != request.Key || feature.Name != request.Name || feature.Meter.ID != request.Meter.ID || len(feature.Meter.Filters) != 0 || (request.Meter.Filters != nil && len(*request.Meter.Filters) != 0) {
		return fmt.Errorf("OpenMeter feature %q has incompatible configuration", request.Key)
	}
	return nil
}

func validatePoCCustomer(request openmeterapi.CreateCustomerRequest, customer *openmeterapi.Customer) error {
	if customer == nil {
		return fmt.Errorf("OpenMeter customer %q is nil", request.Key)
	}
	if customer.Key != request.Key || customer.Name != request.Name || !reflect.DeepEqual(customer.UsageAttribution, request.UsageAttribution) || !equalStringPointers(customer.Currency, request.Currency) {
		return fmt.Errorf("OpenMeter customer %q has incompatible configuration", request.Key)
	}
	return nil
}

func validatePoCPlan(request openmeterapi.CreatePlanRequest, plan *openmeterapi.Plan) error {
	if plan == nil || plan.Key != request.Key || plan.Name != request.Name || plan.Currency != request.Currency || plan.BillingCadence != request.BillingCadence || len(plan.Phases) != len(request.Phases) {
		return fmt.Errorf("OpenMeter plan %q has incompatible top-level configuration", request.Key)
	}
	for phaseIndex, phaseRequest := range request.Phases {
		phase := plan.Phases[phaseIndex]
		if phase.Key != phaseRequest.Key || phase.Name != phaseRequest.Name || !equalStringPointers(phase.Duration, phaseRequest.Duration) || len(phase.RateCards) != len(phaseRequest.RateCards) {
			return fmt.Errorf("OpenMeter plan %q phase %q has incompatible configuration", request.Key, phaseRequest.Key)
		}
		for rateCardIndex, rateCardRequest := range phaseRequest.RateCards {
			if err := validatePoCRateCard(rateCardRequest, phase.RateCards[rateCardIndex]); err != nil {
				return fmt.Errorf("OpenMeter plan %q: %w", request.Key, err)
			}
		}
	}
	return nil
}

func validatePoCRateCard(request openmeterapi.RateCardInput, rateCard openmeterapi.RateCard) error {
	if rateCard.Key != request.Key || rateCard.Name != request.Name || request.Feature == nil || rateCard.Feature == nil || rateCard.Feature.ID != request.Feature.ID || !equalStringPointers(rateCard.BillingCadence, request.BillingCadence) || !reflect.DeepEqual(rateCard.UnitConfig, request.UnitConfig) {
		return fmt.Errorf("rate card %q has incompatible configuration", request.Key)
	}
	if _, err := rateCard.Price.AsPriceFree(); err != nil {
		return fmt.Errorf("rate card %q price is incompatible: %w", request.Key, err)
	}
	if request.Entitlement == nil || rateCard.Entitlement == nil {
		return fmt.Errorf("rate card %q is missing its entitlement", request.Key)
	}
	wantEntitlement, err := request.Entitlement.AsRateCardMeteredEntitlement()
	if err != nil {
		return fmt.Errorf("rate card %q requested entitlement is invalid: %w", request.Key, err)
	}
	gotEntitlement, err := rateCard.Entitlement.AsRateCardMeteredEntitlement()
	if err != nil {
		return fmt.Errorf("rate card %q existing entitlement is incompatible: %w", request.Key, err)
	}
	wantUsagePeriod := wantEntitlement.UsagePeriod
	if wantUsagePeriod == nil {
		wantUsagePeriod = request.BillingCadence
	}
	if !equalFloatPointers(gotEntitlement.Limit, wantEntitlement.Limit) || !equalBoolPointers(gotEntitlement.IsSoftLimit, wantEntitlement.IsSoftLimit) || !equalStringPointers(gotEntitlement.UsagePeriod, wantUsagePeriod) {
		return fmt.Errorf("rate card %q metered entitlement has incompatible configuration", request.Key)
	}
	return nil
}

func validatePoCSubscription(request openmeterapi.SubscriptionCreate, subscription *openmeterapi.BillingSubscription) error {
	if subscription == nil || request.Customer.ID == nil || request.Plan.ID == nil || subscription.CustomerID != *request.Customer.ID || subscription.PlanID == nil || *subscription.PlanID != *request.Plan.ID || subscription.Status != openmeterapi.SubscriptionStatusActive {
		return errors.New("OpenMeter subscription has incompatible customer, plan, or status")
	}
	return nil
}

func isPoCConflict(err error) bool {
	apiErr, ok := openmeterapi.AsAPIError(err)
	return ok && apiErr.StatusCode == http.StatusConflict
}

func equalStringPointers(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalFloatPointers(left, right *float64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalBoolPointers(left, right *bool) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalTimePointers(left, right *time.Time) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && left.Equal(*right))
}

func equalStringMaps(left, right map[string]string) bool {
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	return reflect.DeepEqual(left, right)
}

func TestPoCMeterRequestsMatchUsageContract(t *testing.T) {
	names := pocNamesForRunID("run-42")
	requests := pocMeterRequests(names)
	if len(requests) != 3 {
		t.Fatalf("pocMeterRequests() returned %d requests, want 3", len(requests))
	}

	want := []struct {
		key           string
		aggregation   openmeterapi.MeterAggregation
		eventType     string
		valueProperty string
	}{
		{key: "poc_run_42_studio_meter", aggregation: openmeterapi.MeterAggregationCount, eventType: "listingkit.usage.studio_design_jobs_succeeded"},
		{key: "poc_run_42_shein_meter", aggregation: openmeterapi.MeterAggregationCount, eventType: "listingkit.usage.shein_drafts_succeeded"},
		{key: "poc_run_42_storage_meter", aggregation: openmeterapi.MeterAggregationLatest, eventType: "listingkit.usage.storage_bytes_current", valueProperty: "$.quantity"},
	}
	for index, request := range requests {
		if request.Key != want[index].key || request.Aggregation != want[index].aggregation || request.EventType != want[index].eventType {
			t.Errorf("pocMeterRequests()[%d] = %+v", index, request)
		}
		gotValueProperty := ""
		if request.ValueProperty != nil {
			gotValueProperty = *request.ValueProperty
		}
		if gotValueProperty != want[index].valueProperty {
			t.Errorf("pocMeterRequests()[%d].ValueProperty = %q, want %q", index, gotValueProperty, want[index].valueProperty)
		}
	}
}

func TestPoCCustomerRequestsUseUniqueSubjects(t *testing.T) {
	requests := pocCustomerRequests(pocNamesForRunID("run-42"))
	if len(requests) != 2 {
		t.Fatalf("pocCustomerRequests() returned %d requests, want 2", len(requests))
	}

	wantKeys := []string{"poc_run_42_customer_a", "poc_run_42_customer_b"}
	wantSubjects := []string{"tenant:poc-run-42-a", "tenant:poc-run-42-b"}
	for index, request := range requests {
		if request.Key != wantKeys[index] {
			t.Errorf("pocCustomerRequests()[%d].Key = %q, want %q", index, request.Key, wantKeys[index])
		}
		if request.UsageAttribution == nil {
			t.Fatalf("pocCustomerRequests()[%d].UsageAttribution = nil", index)
		}
		if len(request.UsageAttribution.SubjectKeys) != 1 || request.UsageAttribution.SubjectKeys[0] != wantSubjects[index] {
			t.Errorf("pocCustomerRequests()[%d].SubjectKeys = %v, want [%q]", index, request.UsageAttribution.SubjectKeys, wantSubjects[index])
		}
	}
	if requests[0].UsageAttribution.SubjectKeys[0] == requests[1].UsageAttribution.SubjectKeys[0] {
		t.Fatal("pocCustomerRequests() reused one subject for both customers")
	}
}

func TestPoCPlanRequestUsesOfficialFreePriceAndMeteredEntitlements(t *testing.T) {
	names := pocNamesForRunID("run-42")
	request, err := pocPlanRequest(names, "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}
	if request.Currency != "USD" || request.BillingCadence != "P1M" {
		t.Fatalf("pocPlanRequest() currency/cadence = %q/%q, want USD/P1M", request.Currency, request.BillingCadence)
	}
	if len(request.Phases) != 1 {
		t.Fatalf("pocPlanRequest() phases = %d, want 1", len(request.Phases))
	}
	phase := request.Phases[0]
	if phase.Key != "poc_run_42_phase" || phase.Duration != nil {
		t.Fatalf("pocPlanRequest() phase = %+v, want namespaced indefinite phase", phase)
	}
	if len(phase.RateCards) != 3 {
		t.Fatalf("pocPlanRequest() rate cards = %d, want 3", len(phase.RateCards))
	}

	wantFeatures := []string{"feature-studio", "feature-shein", "feature-storage"}
	wantRateCardKeys := []string{names.StudioFeatureKey, names.SheinFeatureKey, names.StorageFeatureKey}
	wantLimits := []float64{5, 3, 10 * 1024 * 1024}
	for index, rateCard := range phase.RateCards {
		if rateCard.Key != wantRateCardKeys[index] {
			t.Errorf("rate card %d key = %q, want feature key %q", index, rateCard.Key, wantRateCardKeys[index])
		}
		if rateCard.Feature == nil || rateCard.Feature.ID != wantFeatures[index] {
			t.Errorf("rate card %d feature = %+v, want %q", index, rateCard.Feature, wantFeatures[index])
		}
		if rateCard.BillingCadence == nil || *rateCard.BillingCadence != "P1M" {
			t.Errorf("rate card %d billing cadence = %v, want P1M", index, rateCard.BillingCadence)
		}
		if _, err := rateCard.Price.AsPriceFree(); err != nil {
			t.Errorf("rate card %d price is not official free union: %v", index, err)
		}
		if rateCard.Entitlement == nil {
			t.Fatalf("rate card %d entitlement = nil", index)
		}
		entitlement, err := rateCard.Entitlement.AsRateCardMeteredEntitlement()
		if err != nil {
			t.Fatalf("rate card %d entitlement is not official metered union: %v", index, err)
		}
		if entitlement.Limit == nil || *entitlement.Limit != wantLimits[index] {
			t.Errorf("rate card %d limit = %v, want %v", index, entitlement.Limit, wantLimits[index])
		}
		if entitlement.IsSoftLimit == nil || *entitlement.IsSoftLimit {
			t.Errorf("rate card %d soft limit = %v, want false", index, entitlement.IsSoftLimit)
		}
	}
}

func TestValidatePoCPlanAcceptsServerDefaultedEntitlementUsagePeriod(t *testing.T) {
	request, err := pocPlanRequest(pocNamesForRunID("run-42"), "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}
	plan := compatiblePoCPlan(request)
	for index := range plan.Phases[0].RateCards {
		entitlement, err := plan.Phases[0].RateCards[index].Entitlement.AsRateCardMeteredEntitlement()
		if err != nil {
			t.Fatalf("decode rate card %d entitlement: %v", index, err)
		}
		entitlement.UsagePeriod = openmeterapi.Ptr("P1M")
		serverEntitlement, err := openmeterapi.RateCardEntitlementFromRateCardMeteredEntitlement(*entitlement)
		if err != nil {
			t.Fatalf("encode rate card %d entitlement: %v", index, err)
		}
		plan.Phases[0].RateCards[index].Entitlement = &serverEntitlement
	}

	if err := validatePoCPlan(request, &plan); err != nil {
		t.Fatalf("validatePoCPlan() server-defaulted usage period error = %v", err)
	}
}

func TestPoCFixtureValidationRejectsIncompatibleExistingResources(t *testing.T) {
	names := pocNamesForRunID("run-42")

	meterRequest := pocMeterRequests(names)[0]
	meter := openmeterapi.Meter{
		ID:            "meter-studio",
		Name:          meterRequest.Name,
		Key:           meterRequest.Key,
		Aggregation:   meterRequest.Aggregation,
		EventType:     meterRequest.EventType,
		ValueProperty: meterRequest.ValueProperty,
	}
	if err := validatePoCMeter(meterRequest, &meter); err != nil {
		t.Fatalf("validatePoCMeter() compatible error = %v", err)
	}
	meter.EventType = "wrong.event"
	if err := validatePoCMeter(meterRequest, &meter); err == nil {
		t.Fatal("validatePoCMeter() incompatible error = nil")
	}
	meter.EventType = meterRequest.EventType
	meter.Dimensions = map[string]string{"region": "$.region"}
	if err := validatePoCMeter(meterRequest, &meter); err == nil {
		t.Fatal("validatePoCMeter() unexpected dimensions error = nil")
	}

	featureRequest := openmeterapi.CreateFeatureRequest{
		Name:  "poc_run_42_studio_feature",
		Key:   "poc_run_42_studio_feature",
		Meter: &openmeterapi.FeatureMeterReferenceInput{ID: "meter-studio"},
	}
	feature := openmeterapi.Feature{
		ID:   "feature-studio",
		Name: featureRequest.Name,
		Key:  featureRequest.Key,
		Meter: &openmeterapi.FeatureMeterReference{
			ID: "meter-studio",
		},
	}
	if err := validatePoCFeature(featureRequest, &feature); err != nil {
		t.Fatalf("validatePoCFeature() compatible error = %v", err)
	}
	feature.Meter.ID = "meter-other"
	if err := validatePoCFeature(featureRequest, &feature); err == nil {
		t.Fatal("validatePoCFeature() incompatible error = nil")
	}
	feature.Meter.ID = "meter-studio"
	feature.Meter.Filters = map[string]openmeterapi.QueryFilterStringMapItem{"region": {}}
	if err := validatePoCFeature(featureRequest, &feature); err == nil {
		t.Fatal("validatePoCFeature() unexpected meter filters error = nil")
	}

	customerRequest := pocCustomerRequests(names)[0]
	customer := openmeterapi.Customer{
		ID:               "customer-a",
		Name:             customerRequest.Name,
		Key:              customerRequest.Key,
		UsageAttribution: customerRequest.UsageAttribution,
		Currency:         customerRequest.Currency,
	}
	if err := validatePoCCustomer(customerRequest, &customer); err != nil {
		t.Fatalf("validatePoCCustomer() compatible error = %v", err)
	}
	customer.UsageAttribution = &openmeterapi.CustomerUsageAttribution{SubjectKeys: []string{"tenant:wrong"}}
	if err := validatePoCCustomer(customerRequest, &customer); err == nil {
		t.Fatal("validatePoCCustomer() incompatible error = nil")
	}

	planRequest, err := pocPlanRequest(names, "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}
	plan := compatiblePoCPlan(planRequest)
	if err := validatePoCPlan(planRequest, &plan); err != nil {
		t.Fatalf("validatePoCPlan() compatible error = %v", err)
	}
	plan.Currency = "EUR"
	if err := validatePoCPlan(planRequest, &plan); err == nil {
		t.Fatal("validatePoCPlan() incompatible error = nil")
	}
	plan.Currency = "USD"
	plan.Phases[0].RateCards[0].UnitConfig = &openmeterapi.UnitConfig{
		Operation:        openmeterapi.UnitConfigOperationMultiply,
		ConversionFactor: "2",
	}
	if err := validatePoCPlan(planRequest, &plan); err == nil {
		t.Fatal("validatePoCPlan() unexpected unit config error = nil")
	}

	planID := "plan-1"
	customerID := "customer-a"
	subscriptionRequest := pocSubscriptionRequest(customerID, planID)
	subscription := openmeterapi.BillingSubscription{CustomerID: customerID, PlanID: &planID, Status: openmeterapi.SubscriptionStatusActive}
	if err := validatePoCSubscription(subscriptionRequest, &subscription); err != nil {
		t.Fatalf("validatePoCSubscription() compatible error = %v", err)
	}
	subscription.PlanID = openmeterapi.Ptr("plan-other")
	if err := validatePoCSubscription(subscriptionRequest, &subscription); err == nil {
		t.Fatal("validatePoCSubscription() incompatible error = nil")
	}
	subscription.PlanID = &planID
	subscription.Status = openmeterapi.SubscriptionStatusCanceled
	if err := validatePoCSubscription(subscriptionRequest, &subscription); err == nil {
		t.Fatal("validatePoCSubscription() canceled status error = nil")
	}
}

func TestWaitForPoCMeterCancelsInFlightRequestAtTotalDeadline(t *testing.T) {
	var requestCanceled atomic.Bool
	httpClient := &http.Client{Transport: pocRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		requestCanceled.Store(true)
		return nil, request.Context().Err()
	})}
	sdk, err := openmeterapi.New("http://openmeter.invalid/api/v3", openmeterapi.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("construct official OpenMeter SDK: %v", err)
	}

	parentContext, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	_, err = waitForPoCMeterWithin(parentContext, sdk, "meter-studio", time.Millisecond, 20*time.Millisecond)
	elapsed := time.Since(startedAt)
	if err == nil {
		t.Fatal("waitForPoCMeterWithin() error = nil, want total-deadline error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForPoCMeterWithin() error = %v, want context deadline exceeded", err)
	}
	if !requestCanceled.Load() {
		t.Fatal("meter visibility request context was not canceled")
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("waitForPoCMeterWithin() elapsed = %s, want below 250ms", elapsed)
	}
}

func TestEnsurePoCMeterConflictFetchesExactKeyAndRejectsIncompatibleConfig(t *testing.T) {
	request := pocMeterRequests(pocNamesForRunID("run-42"))[0]
	existing := openmeterapi.Meter{
		ID:          "meter-studio",
		Name:        request.Name,
		Key:         request.Key,
		Aggregation: request.Aggregation,
		EventType:   "listingkit.usage.wrong",
	}
	sdk, transport := newPoCSequenceSDK(t,
		pocSDKStep{Method: http.MethodPost, Path: "/api/v3/openmeter/meters", Status: http.StatusConflict, Body: pocConflictBody},
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/meters",
			Query:  url.Values{"filter[key][eq]": []string{request.Key}},
			Status: http.StatusOK,
			Body: marshalPoCTestJSON(t, openmeterapi.MeterPagePaginatedResponse{
				Data: []openmeterapi.Meter{existing},
			}),
		},
	)

	_, err := ensurePoCMeter(t.Context(), sdk, request)
	if err == nil || !strings.Contains(err.Error(), "incompatible configuration") {
		t.Fatalf("ensurePoCMeter() error = %v, want incompatible conflict error", err)
	}
	transport.Verify(t)
}

func TestEnsurePoCFeatureConflictFetchesExactKeyAndRejectsFilters(t *testing.T) {
	request := openmeterapi.CreateFeatureRequest{
		Name:  "poc_run_42_studio_feature",
		Key:   "poc_run_42_studio_feature",
		Meter: &openmeterapi.FeatureMeterReferenceInput{ID: "meter-studio"},
	}
	existing := openmeterapi.Feature{
		ID:   "feature-studio",
		Name: request.Name,
		Key:  request.Key,
		Meter: &openmeterapi.FeatureMeterReference{
			ID:      "meter-studio",
			Filters: map[string]openmeterapi.QueryFilterStringMapItem{"region": {}},
		},
	}
	sdk, transport := newPoCSequenceSDK(t,
		pocSDKStep{Method: http.MethodPost, Path: "/api/v3/openmeter/features", Status: http.StatusConflict, Body: pocConflictBody},
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/features",
			Query:  url.Values{"filter[key][eq]": []string{request.Key}},
			Status: http.StatusOK,
			Body: marshalPoCTestJSON(t, openmeterapi.FeaturePagePaginatedResponse{
				Data: []openmeterapi.Feature{existing},
			}),
		},
	)

	_, err := ensurePoCFeature(t.Context(), sdk, request)
	if err == nil || !strings.Contains(err.Error(), "incompatible configuration") {
		t.Fatalf("ensurePoCFeature() error = %v, want incompatible filters error", err)
	}
	transport.Verify(t)
}

func TestEnsurePoCCustomerConflictFetchesExactKeyAndRejectsAttribution(t *testing.T) {
	request := pocCustomerRequests(pocNamesForRunID("run-42"))[0]
	existing := openmeterapi.Customer{
		ID:               "customer-a",
		Name:             request.Name,
		Key:              request.Key,
		Currency:         request.Currency,
		UsageAttribution: &openmeterapi.CustomerUsageAttribution{SubjectKeys: []string{"tenant:wrong"}},
	}
	sdk, transport := newPoCSequenceSDK(t,
		pocSDKStep{Method: http.MethodPost, Path: "/api/v3/openmeter/customers", Status: http.StatusConflict, Body: pocConflictBody},
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/customers",
			Query:  url.Values{"filter[key][eq]": []string{request.Key}},
			Status: http.StatusOK,
			Body: marshalPoCTestJSON(t, openmeterapi.CustomerPagePaginatedResponse{
				Data: []openmeterapi.Customer{existing},
			}),
		},
	)

	_, err := ensurePoCCustomer(t.Context(), sdk, request)
	if err == nil || !strings.Contains(err.Error(), "incompatible configuration") {
		t.Fatalf("ensurePoCCustomer() error = %v, want incompatible attribution error", err)
	}
	transport.Verify(t)
}

func TestEnsurePoCPlanFindsExactKeyAndRejectsIncompatibleConfig(t *testing.T) {
	request, err := pocPlanRequest(pocNamesForRunID("run-42"), "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}
	existing := compatiblePoCPlan(request)
	existing.Currency = "EUR"
	sdk, transport := newPoCSequenceSDK(t,
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/plans",
			Query:  url.Values{"filter[key][eq]": []string{request.Key}},
			Status: http.StatusOK,
			Body: marshalPoCTestJSON(t, openmeterapi.PlanPagePaginatedResponse{
				Data: []openmeterapi.Plan{existing},
			}),
		},
	)

	_, err = ensurePoCPlan(t.Context(), sdk, request)
	if err == nil || !strings.Contains(err.Error(), "incompatible top-level configuration") {
		t.Fatalf("ensurePoCPlan() error = %v, want incompatible plan error", err)
	}
	transport.Verify(t)
}

func TestEnsurePoCPlanReusesExistingActiveVersionWithoutCreatingDraft(t *testing.T) {
	request, err := pocPlanRequest(pocNamesForRunID("run-42"), "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}
	existing := compatiblePoCPlan(request)
	existing.Status = openmeterapi.PlanStatusActive
	sdk, transport := newPoCSequenceSDK(t,
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/plans",
			Query:  url.Values{"filter[key][eq]": []string{request.Key}},
			Status: http.StatusOK,
			Body: marshalPoCTestJSON(t, openmeterapi.PlanPagePaginatedResponse{
				Data: []openmeterapi.Plan{existing},
			}),
		},
	)

	plan, err := ensurePoCPlan(t.Context(), sdk, request)
	if err != nil {
		t.Fatalf("ensurePoCPlan() error = %v", err)
	}
	if plan.ID != existing.ID || plan.Status != openmeterapi.PlanStatusActive {
		t.Fatalf("ensurePoCPlan() = %+v, want existing active plan %+v", plan, existing)
	}
	transport.Verify(t)
}

func TestEnsurePoCPlanPublishesDraftAndSubscriptionCreatesThenReuses(t *testing.T) {
	planRequest, err := pocPlanRequest(pocNamesForRunID("run-42"), "feature-studio", "feature-shein", "feature-storage")
	if err != nil {
		t.Fatalf("pocPlanRequest() error = %v", err)
	}
	draftPlan := compatiblePoCPlan(planRequest)
	activePlan := compatiblePoCPlan(planRequest)
	activePlan.Status = openmeterapi.PlanStatusActive
	customerID := "customer-a"
	activeSubscription := openmeterapi.BillingSubscription{
		ID:         "subscription-a",
		CustomerID: customerID,
		PlanID:     &activePlan.ID,
		Status:     openmeterapi.SubscriptionStatusActive,
	}
	sdk, transport := newPoCSequenceSDK(t,
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/plans",
			Query:  url.Values{"filter[key][eq]": []string{planRequest.Key}},
			Status: http.StatusOK,
			Body:   marshalPoCTestJSON(t, openmeterapi.PlanPagePaginatedResponse{}),
		},
		pocSDKStep{Method: http.MethodPost, Path: "/api/v3/openmeter/plans", Status: http.StatusCreated, Body: marshalPoCTestJSON(t, draftPlan)},
		pocSDKStep{Method: http.MethodPost, Path: "/api/v3/openmeter/plans/plan-1/publish", Status: http.StatusOK, Body: marshalPoCTestJSON(t, activePlan)},
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/subscriptions",
			Query: url.Values{
				"filter[customer_id][eq]": []string{customerID},
				"filter[plan_id][eq]":     []string{activePlan.ID},
			},
			Status: http.StatusOK,
			Body:   marshalPoCTestJSON(t, openmeterapi.SubscriptionPagePaginatedResponse{}),
		},
		pocSDKStep{Method: http.MethodPost, Path: "/api/v3/openmeter/subscriptions", Status: http.StatusCreated, Body: marshalPoCTestJSON(t, activeSubscription)},
		pocSDKStep{
			Method: http.MethodGet,
			Path:   "/api/v3/openmeter/subscriptions",
			Query: url.Values{
				"filter[customer_id][eq]": []string{customerID},
				"filter[plan_id][eq]":     []string{activePlan.ID},
			},
			Status: http.StatusOK,
			Body: marshalPoCTestJSON(t, openmeterapi.SubscriptionPagePaginatedResponse{
				Data: []openmeterapi.BillingSubscription{activeSubscription},
			}),
		},
	)

	plan, err := ensurePoCPlan(t.Context(), sdk, planRequest)
	if err != nil {
		t.Fatalf("ensurePoCPlan() error = %v", err)
	}
	request := pocSubscriptionRequest(customerID, plan.ID)
	created, err := ensurePoCSubscription(t.Context(), sdk, request)
	if err != nil {
		t.Fatalf("ensurePoCSubscription() create error = %v", err)
	}
	reused, err := ensurePoCSubscription(t.Context(), sdk, request)
	if err != nil {
		t.Fatalf("ensurePoCSubscription() reuse error = %v", err)
	}
	if created.ID != "subscription-a" || reused.ID != created.ID {
		t.Fatalf("subscription IDs = created %q, reused %q", created.ID, reused.ID)
	}
	transport.Verify(t)
}

func compatiblePoCPlan(request openmeterapi.CreatePlanRequest) openmeterapi.Plan {
	phaseInput := request.Phases[0]
	rateCards := make([]openmeterapi.RateCard, 0, len(phaseInput.RateCards))
	for _, input := range phaseInput.RateCards {
		entitlement := input.Entitlement
		if entitlement != nil {
			metered, err := entitlement.AsRateCardMeteredEntitlement()
			if err != nil {
				panic(fmt.Sprintf("decode compatible PoC entitlement: %v", err))
			}
			if metered.UsagePeriod == nil {
				metered.UsagePeriod = input.BillingCadence
			}
			serverEntitlement, err := openmeterapi.RateCardEntitlementFromRateCardMeteredEntitlement(*metered)
			if err != nil {
				panic(fmt.Sprintf("encode compatible PoC entitlement: %v", err))
			}
			entitlement = &serverEntitlement
		}
		rateCards = append(rateCards, openmeterapi.RateCard{
			Name:           input.Name,
			Key:            input.Key,
			Feature:        input.Feature,
			BillingCadence: input.BillingCadence,
			Price:          input.Price,
			Entitlement:    entitlement,
		})
	}
	return openmeterapi.Plan{
		ID:             "plan-1",
		Name:           request.Name,
		Key:            request.Key,
		Currency:       request.Currency,
		BillingCadence: request.BillingCadence,
		Status:         openmeterapi.PlanStatusDraft,
		Phases: []openmeterapi.PlanPhase{{
			Name:      phaseInput.Name,
			Key:       phaseInput.Key,
			Duration:  phaseInput.Duration,
			RateCards: rateCards,
		}},
	}
}

func TestOpenMeterPoCFixtureSetup(t *testing.T) {
	fixture := requirePoCFixture(t)
	if len(fixture.Meters) != 3 || len(fixture.Features) != 3 || len(fixture.Customers) != 2 || len(fixture.Subscriptions) != 2 {
		t.Fatalf("fixture resources = %d meters, %d features, %d customers, %d subscriptions", len(fixture.Meters), len(fixture.Features), len(fixture.Customers), len(fixture.Subscriptions))
	}
}

func TestRequirePoCFixtureDefaultModeDoesNotConnect(t *testing.T) {
	var requests atomic.Int32
	httpClient := &http.Client{Transport: pocRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("unexpected OpenMeter request")
	})}

	fixture, enabled, err := loadPoCFixture(t.Context(), pocEnvironment{BaseURL: "http://openmeter.invalid/api/v3"}, httpClient)
	if err != nil {
		t.Fatalf("loadPoCFixture() error = %v", err)
	}
	if enabled || fixture != nil {
		t.Fatalf("loadPoCFixture() = (%+v, %t), want (nil, false)", fixture, enabled)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("default-disabled fixture made %d OpenMeter requests, want 0", got)
	}
}
