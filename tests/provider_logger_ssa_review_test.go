package tests

import "testing"

func TestProviderLoggerSSANilLatticeRejectsStaticHelperAndMethodReturns(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func nilEntry() *openai.Entry { return nil }

type nilFactory struct{}

func (nilFactory) entry() *openai.Entry { return nil }

func useFreeHelper() {
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus(nilEntry())}
	openai.NewClient(config)
	productenrich.NewLLMManagerAdapter("config", nilEntry())
}

func useMethodHelper() {
	factory := nilFactory{}
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus(factory.entry())}
	openai.NewClient(config)
	productenrich.NewLLMManagerAdapter("config", factory.entry())
}
`)
	violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules())
	if len(violations) != 4 {
		t.Fatalf("violations = %v, want both uses of free and method nil helpers rejected", violations)
	}
}

func TestProviderLoggerSSANilLatticeRejectsNilSelfPhi(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func nilSelfPhi(iterations int, reset bool) *openai.Entry {
	var logger *openai.Entry
	for iterations > 0 {
		if reset {
			logger = nil
		}
		iterations--
	}
	return logger
}

func build(iterations int, reset bool) {
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus(nilSelfPhi(iterations, reset))}
	openai.NewClient(config)
	productenrich.NewLLMManagerAdapter("config", nilSelfPhi(iterations, reset))
}
`)
	violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules())
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want nil+self Phi rejected at provider and LLM calls", violations)
	}
}

func TestProviderLoggerSSANilLatticeAcceptsPhiWithUnknownEdge(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func maybeEntry(entry *openai.Entry, iterations int) *openai.Entry {
	logger := entry
	for iterations > 0 {
		logger = nil
		iterations--
	}
	return logger
}

func build(entry *openai.Entry, iterations int) {
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus(maybeEntry(entry, iterations))}
	openai.NewClient(config)
	productenrich.NewLLMManagerAdapter("config", maybeEntry(entry, iterations))
}
`)
	if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules()); len(violations) != 0 {
		t.Fatalf("unknown Phi edge must not be treated as definitely nil: %v", violations)
	}
}

func TestManagedConstructorFunctionValuesRejectAliasAndPhi(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func localLLMAdapter(any, *openai.Entry) {}

func providerAlias(entry *openai.Entry) {
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus(entry)}
	constructor := openai.NewClient
	constructor(config)
}

func llmPhi(entry *openai.Entry, local bool) {
	constructor := productenrich.NewLLMManagerAdapter
	if local {
		constructor = localLLMAdapter
	}
	constructor("config", entry)
}
`)
	violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules())
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want provider alias and LLM Phi rejected", violations)
	}
}

func TestManagedConstructorsAcceptDirectStaticCalls(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func build(entry *openai.Entry) {
	config := &openai.ClientConfig{Logger: openai.AdaptLogrus(entry)}
	openai.NewClient(config)
	productenrich.NewLLMManagerAdapter("config", entry)
}
`)
	if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules()); len(violations) != 0 {
		t.Fatalf("direct static constructor calls reported violations: %v", violations)
	}
}

func TestProviderLoggerSSAWholeConfigStoreAcceptsBoundLogger(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import openai "example/root/openai"

func build(entry *openai.Entry) {
	config := &openai.ClientConfig{}
	*config = openai.ClientConfig{Logger: openai.AdaptLogrus(entry)}
	openai.NewClient(config)
}
`)
	if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), nil); len(violations) != 0 {
		t.Fatalf("bound whole-config store reported violations: %v", violations)
	}
}

func TestProviderLoggerSSAWholeConfigStoreRejectsWrongOrNilLogger(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	grsai "example/root/grsai"
	openai "example/root/openai"
)

func nilEntry() *openai.Entry { return nil }

func wrongProvider(entry *grsai.Entry) {
	config := &openai.ClientConfig{}
	*config = openai.ClientConfig{Logger: grsai.AdaptLogrus(entry)}
	openai.NewClient(config)
}

func nilLogger() {
	config := &openai.ClientConfig{}
	*config = openai.ClientConfig{Logger: openai.AdaptLogrus(nilEntry())}
	openai.NewClient(config)
}
`)
	violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), nil)
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want wrong-provider and nil logger whole stores rejected", violations)
	}
}

func TestProviderLoggerSSACallSiteNilRejectsIdentityAndRecursiveIdentityOfNil(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func identity(entry *openai.Entry) *openai.Entry { return entry }

func recursiveIdentity(entry *openai.Entry, remaining int) *openai.Entry {
	if remaining <= 0 {
		return entry
	}
	return recursiveIdentity(entry, remaining-1)
}

type entryAlias openai.Entry

func (entry *entryAlias) identity() *openai.Entry {
	return (*openai.Entry)(entry)
}

func build() {
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(identity(nil))})
	productenrich.NewLLMManagerAdapter("config", identity(nil))
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(recursiveIdentity(nil, 3))})
	productenrich.NewLLMManagerAdapter("config", recursiveIdentity(nil, 3))
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(((*entryAlias)(nil)).identity())})
	productenrich.NewLLMManagerAdapter("config", ((*entryAlias)(nil)).identity())
}
`)
	violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules())
	if len(violations) != 6 {
		t.Fatalf("violations = %v, want nil actuals propagated through identity, recursive identity, and receiver", violations)
	}
}

func TestProviderLoggerSSACallSiteNilAcceptsUnknownAndNonNilActuals(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func identity(entry *openai.Entry) *openai.Entry { return entry }

func recursiveIdentity(entry *openai.Entry, remaining int) *openai.Entry {
	if remaining <= 0 {
		return entry
	}
	return recursiveIdentity(entry, remaining-1)
}

type entryAlias openai.Entry

func (entry *entryAlias) identity() *openai.Entry {
	return (*openai.Entry)(entry)
}

func build(entry *openai.Entry, alias *entryAlias) {
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(identity(entry))})
	productenrich.NewLLMManagerAdapter("config", identity(&openai.Entry{}))
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(recursiveIdentity(entry, 3))})
	productenrich.NewLLMManagerAdapter("config", recursiveIdentity(&openai.Entry{}, 3))
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(alias.identity())})
	productenrich.NewLLMManagerAdapter("config", (&entryAlias{}).identity())
}
`)
	if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules()); len(violations) != 0 {
		t.Fatalf("unknown or nonnil actuals must not be treated as definitely nil: %v", violations)
	}
}

func TestProviderLoggerSSAReachableReturnsIgnoreConstantFalseBranch(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func reachableNil() *openai.Entry {
	if false {
		return &openai.Entry{}
	}
	return nil
}

func build() {
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(reachableNil())})
	productenrich.NewLLMManagerAdapter("config", reachableNil())
}
`)
	violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules())
	if len(violations) != 2 {
		t.Fatalf("violations = %v, want only entry-reachable nil return considered", violations)
	}
}

func TestProviderLoggerSSAReachableReturnsTreatNoReturnAsUnknown(t *testing.T) {
	loaded := loadProviderLoggerSSAFixture(t, `package sample

import (
	openai "example/root/openai"
	productenrich "example/root/productenrich"
)

func neverReturns() *openai.Entry {
	for {}
}

func build() {
	openai.NewClient(&openai.ClientConfig{Logger: openai.AdaptLogrus(neverReturns())})
	productenrich.NewLLMManagerAdapter("config", neverReturns())
}
`)
	if violations := loggerContractSSAViolations(loaded, fixtureProviderLoggerRules(), fixtureTypedNilRules()); len(violations) != 0 {
		t.Fatalf("helper without an entry-reachable return must remain unknown: %v", violations)
	}
}

func fixtureTypedNilRules() []typedNilCallRule {
	return []typedNilCallRule{{
		PackagePath:  "example/root/productenrich",
		FunctionName: "NewLLMManagerAdapter",
		Argument:     1,
	}}
}
