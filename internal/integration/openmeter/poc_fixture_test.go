package openmeter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
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
		{key: names.StudioFeatureKey + "-rate-card", featureID: studioFeatureID, limit: 5},
		{key: names.SheinFeatureKey + "-rate-card", featureID: sheinFeatureID, limit: 3},
		{key: names.StorageFeatureKey + "-rate-card", featureID: storageFeatureID, limit: 10 * 1024 * 1024},
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
	ticker := time.NewTicker(pocMeterPollInterval)
	defer ticker.Stop()
	timer := time.NewTimer(pocMeterPollTimeout)
	defer timer.Stop()

	lastResult := "no visibility request completed"
	for {
		meter, err := sdk.Meters.Get(ctx, meterID)
		if err == nil {
			return meter, nil
		}
		apiErr, isAPIError := openmeterapi.AsAPIError(err)
		if !isAPIError || apiErr.StatusCode != http.StatusNotFound {
			return nil, err
		}
		lastResult = err.Error()

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context ended after last result %q: %w", lastResult, ctx.Err())
		case <-timer.C:
			return nil, fmt.Errorf("timed out after %s; last result: %s", pocMeterPollTimeout, lastResult)
		case <-ticker.C:
		}
	}
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
	plan, err := sdk.Plans.Create(ctx, request)
	if err != nil {
		if !isPoCConflict(err) {
			return nil, fmt.Errorf("create OpenMeter plan %q: %w", request.Key, err)
		}
		plan, err = findPoCPlanByKey(ctx, sdk, request.Key)
		if err != nil {
			return nil, fmt.Errorf("fetch conflicting OpenMeter plan %q: %w", request.Key, err)
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

func findPoCPlanByKey(ctx context.Context, sdk *openmeterapi.Client, key string) (*openmeterapi.Plan, error) {
	page, err := sdk.Plans.List(ctx, openmeterapi.PlanListParams{Filter: &openmeterapi.PlanFilter{Key: &openmeterapi.StringFilter{Eq: &key}}})
	if err != nil {
		return nil, err
	}
	if len(page.Data) != 1 {
		return nil, fmt.Errorf("expected exactly one plan for key %q, got %d", key, len(page.Data))
	}
	return &page.Data[0], nil
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
	if !equalFloatPointers(gotEntitlement.Limit, wantEntitlement.Limit) || !equalBoolPointers(gotEntitlement.IsSoftLimit, wantEntitlement.IsSoftLimit) || !equalStringPointers(gotEntitlement.UsagePeriod, wantEntitlement.UsagePeriod) {
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
		{key: "poc-run-42-studio-meter", aggregation: openmeterapi.MeterAggregationCount, eventType: "listingkit.usage.studio_design_jobs_succeeded"},
		{key: "poc-run-42-shein-meter", aggregation: openmeterapi.MeterAggregationCount, eventType: "listingkit.usage.shein_drafts_succeeded"},
		{key: "poc-run-42-storage-meter", aggregation: openmeterapi.MeterAggregationLatest, eventType: "listingkit.usage.storage_bytes_current", valueProperty: "$.quantity"},
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

	wantKeys := []string{"poc-run-42-customer-a", "poc-run-42-customer-b"}
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
	request, err := pocPlanRequest(pocNamesForRunID("run-42"), "feature-studio", "feature-shein", "feature-storage")
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
	if phase.Key != "poc-run-42-phase" || phase.Duration != nil {
		t.Fatalf("pocPlanRequest() phase = %+v, want namespaced indefinite phase", phase)
	}
	if len(phase.RateCards) != 3 {
		t.Fatalf("pocPlanRequest() rate cards = %d, want 3", len(phase.RateCards))
	}

	wantFeatures := []string{"feature-studio", "feature-shein", "feature-storage"}
	wantLimits := []float64{5, 3, 10 * 1024 * 1024}
	for index, rateCard := range phase.RateCards {
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
		Name:  "poc-run-42-studio-feature",
		Key:   "poc-run-42-studio-feature",
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

func compatiblePoCPlan(request openmeterapi.CreatePlanRequest) openmeterapi.Plan {
	phaseInput := request.Phases[0]
	rateCards := make([]openmeterapi.RateCard, 0, len(phaseInput.RateCards))
	for _, input := range phaseInput.RateCards {
		rateCards = append(rateCards, openmeterapi.RateCard{
			Name:           input.Name,
			Key:            input.Key,
			Feature:        input.Feature,
			BillingCadence: input.BillingCadence,
			Price:          input.Price,
			Entitlement:    input.Entitlement,
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
