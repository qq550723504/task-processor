package httpapi

import "testing"

func TestBuildRuntimeSupportProvidesRepositoryAndHookBundles(t *testing.T) {
	t.Parallel()

	support := BuildRuntimeSupport(RuntimeSupportInput{})
	if support.Repositories.Core.Task == nil {
		t.Fatal("expected core task repository builder")
	}
	if support.Repositories.Admin.Store == nil {
		t.Fatal("expected admin store repository builder")
	}
	if support.Hooks.SheinPricingPolicyBuilder == nil {
		t.Fatal("expected shein pricing policy builder")
	}
	if support.Hooks.ConfigureAuthorization == nil {
		t.Fatal("expected authorization hook")
	}
}

func TestBuildRuntimeModuleAndTemporalRuntimeAcceptRuntimeSupport(t *testing.T) {
	t.Parallel()

	serviceInput := buildSuccessfulServiceInputFixture()
	runtime := RuntimeDependencies{
		Config:         serviceInput.Config,
		ProductService: serviceInput.ProductService,
		Support:        BuildRuntimeSupport(RuntimeSupportInput{}),
	}

	module, err := BuildRuntimeModule(RuntimeBuildInput{
		Logger:  serviceInput.Logger,
		Runtime: runtime,
	})
	if err != nil {
		t.Fatalf("BuildRuntimeModule() error = %v", err)
	}
	if module == nil {
		t.Fatal("expected module")
	}

	result, err := BuildTemporalRuntime(TemporalRuntimeBuildInput{
		Logger:  serviceInput.Logger,
		Runtime: runtime,
	})
	if err != nil {
		t.Fatalf("BuildTemporalRuntime() error = %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
