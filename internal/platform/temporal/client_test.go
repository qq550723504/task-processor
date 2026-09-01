package temporal

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	sdkclient "go.temporal.io/sdk/client"
	sdkmocks "go.temporal.io/sdk/mocks"
)

func TestOptionsAppliesDefaults(t *testing.T) {
	got := Options(Config{})
	if got.HostPort != "localhost:7233" || got.Namespace != "default" {
		t.Fatalf("options = %#v", got)
	}
}

func TestOptionsPreservesExplicitConnectionSettings(t *testing.T) {
	got := Options(Config{Address: "temporal.internal:7233", Namespace: "listingkit"})
	if got.HostPort != "temporal.internal:7233" || got.Namespace != "listingkit" {
		t.Fatalf("options = %#v", got)
	}
}

func TestDialWithForwardsContextAndOptionsAndClosesOnce(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("request"), "canary")
	client := sdkmocks.NewClient(t)
	client.On("Close").Return().Once()

	var gotContext context.Context
	var gotOptions sdkclient.Options
	gotClient, closeFn, err := dialWith(ctx, Config{
		Address:   "temporal.internal:7233",
		Namespace: "listingkit",
	}, func(callContext context.Context, options sdkclient.Options) (sdkclient.Client, error) {
		gotContext = callContext
		gotOptions = options
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotClient != client {
		t.Fatalf("client = %T, want injected client", gotClient)
	}
	if gotContext != ctx || gotContext.Value(contextKey("request")) != "canary" {
		t.Fatal("dial context was not forwarded unchanged")
	}
	if gotOptions.HostPort != "temporal.internal:7233" || gotOptions.Namespace != "listingkit" {
		t.Fatalf("dial options = %#v", gotOptions)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}

func TestDialWithReturnsDialErrorWithoutCloseOwner(t *testing.T) {
	want := errors.New("dial unavailable")
	client, closeFn, err := dialWith(context.Background(), Config{}, func(context.Context, sdkclient.Options) (sdkclient.Client, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if client != nil || closeFn != nil {
		t.Fatalf("client nil = %t, closeFn nil = %t; want true, true", client == nil, closeFn == nil)
	}
}

func TestDialUsesCallerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, closeFn, err := Dial(ctx, Config{Address: "127.0.0.1:1"})
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("error = %v, want an error caused by caller cancellation", err)
	}
	if client != nil || closeFn != nil {
		t.Fatalf("client nil = %t, closeFn nil = %t; want true, true", client == nil, closeFn == nil)
	}
}

func TestDialBindsSDKDialContextWithoutMutablePackageHook(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "client.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	sdkAlias := "client"
	for _, spec := range file.Imports {
		if spec.Path.Value == `"go.temporal.io/sdk/client"` && spec.Name != nil {
			sdkAlias = spec.Name.Name
		}
	}
	var dialDeclaration *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "Dial" {
			dialDeclaration = function
			break
		}
	}
	if dialDeclaration == nil || dialDeclaration.Body == nil || len(dialDeclaration.Body.List) != 1 {
		t.Fatal("Dial must contain exactly one return statement")
	}
	statement, ok := dialDeclaration.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(statement.Results) != 1 {
		t.Fatal("Dial must directly return dialWith")
	}
	call, ok := statement.Results[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 3 {
		t.Fatal("Dial must call dialWith with context, config, and SDK dialer")
	}
	callee, ok := call.Fun.(*ast.Ident)
	if !ok || callee.Name != "dialWith" {
		t.Fatal("Dial must directly call dialWith")
	}
	if dialDeclaration.Type.Params == nil || len(dialDeclaration.Type.Params.List) != 2 ||
		len(dialDeclaration.Type.Params.List[0].Names) != 1 || len(dialDeclaration.Type.Params.List[1].Names) != 1 {
		t.Fatal("Dial must accept the caller context and platform config")
	}
	contextArgument, contextOK := call.Args[0].(*ast.Ident)
	configArgument, configOK := call.Args[1].(*ast.Ident)
	if !contextOK || !configOK ||
		contextArgument.Name != dialDeclaration.Type.Params.List[0].Names[0].Name ||
		configArgument.Name != dialDeclaration.Type.Params.List[1].Names[0].Name {
		t.Fatal("Dial must forward its context and config parameters unchanged")
	}
	selector, ok := call.Args[2].(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "DialContext" {
		t.Fatal("Dial must bind the SDK DialContext function")
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Name != sdkAlias {
		t.Fatal("DialContext binding must come from the Temporal SDK client import")
	}
}
