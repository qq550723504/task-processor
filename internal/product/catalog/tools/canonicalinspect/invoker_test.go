package canonicalinspect

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"task-processor/internal/commercetool"
)

func TestNewInvokerAcceptsCompleteMultiToolAgentAllowlist(t *testing.T) {
	definition := Definition()
	reader := &catalogReaderStub{published: validPublished(1)}
	agent := commercetool.AgentDefinition{ID: "product.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{
		definition.Ref,
		{ID: "product.other.read", Version: "v1.0.0"},
	}}
	invoker, err := NewInvoker(reader, agent, invokerTestDependencies())
	if err != nil {
		t.Fatalf("NewInvoker() with complete agent allowlist: %v", err)
	}
	result, err := invoker.Invoke(context.Background(), commercetool.CallMetadata{
		CallID: "call-1", AgentID: agent.ID, AgentVersion: agent.Version, AgentRunID: "run-1", BusinessTaskID: "correlation-only",
	}, Input{ProductKey: "product-1", CatalogVersion: "1"})
	if err != nil || len(result.Output) == 0 || reader.calls != 1 {
		t.Fatalf("Invoke() result=%#v error=%v reader calls=%d", result, err, reader.calls)
	}
}

func TestNewInvokerRejectsAgentThatDoesNotAllowCanonicalInspect(t *testing.T) {
	reader := &catalogReaderStub{published: validPublished(1)}
	_, err := NewInvoker(reader, commercetool.AgentDefinition{ID: "product.agent", Version: "v1.0.0", AllowedTools: []commercetool.ToolRef{{ID: "product.other.read", Version: "v1.0.0"}}}, invokerTestDependencies())
	if err == nil {
		t.Fatal("NewInvoker() error = nil")
	}
}

func invokerTestDependencies() commercetool.InvocationDependencies {
	return commercetool.InvocationDependencies{
		PrincipalResolver: fixedPrincipalResolver{principal: schemaPrincipal()}, Authorizer: schemaAuthorizer{}, Recorder: schemaAuditRecorder{},
		Tracer: otel.Tracer("canonicalinspect-invoker-test"), Now: time.Now, AuditTimeout: time.Second,
	}
}
