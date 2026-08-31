package tests

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type providerLoggerState uint8

const (
	providerLoggerUnbound providerLoggerState = 1 << iota
	providerLoggerGood
	providerLoggerBad
)

func loggerContractSSAViolations(loaded []*packages.Package, providerRules []providerLoggerRule, nilRules []typedNilCallRule) []string {
	program, _ := ssautil.AllPackages(loaded, ssa.InstantiateGenerics)
	program.Build()
	rulesByConstructor := make(map[string]providerLoggerRule, len(providerRules))
	for _, rule := range providerRules {
		rulesByConstructor[rule.PackagePath+"."+rule.ConstructorName] = rule
	}
	rulesByNilCheckedFunction := make(map[string]typedNilCallRule, len(nilRules))
	for _, rule := range nilRules {
		rulesByNilCheckedFunction[rule.PackagePath+"."+rule.FunctionName] = rule
	}
	var violations []string
	for _, function := range loadedSourceFunctions(program, loaded) {
		for _, block := range function.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(*ssa.Call)
				if !ok {
					continue
				}
				object, ok := providerFunctionObject(call.Common().StaticCallee())
				if !ok || object.Pkg() == nil {
					continue
				}
				functionKey := object.Pkg().Path() + "." + object.Name()
				if rule, ok := rulesByConstructor[functionKey]; ok && function.Pkg.Pkg.Path() != rule.PackagePath {
					if rule.ConfigArgument >= len(call.Common().Args) || !providerConfigDefinitelyBound(function, call.Common().Args[rule.ConfigArgument], call, rule, make(map[*ssa.Function]bool)) {
						position := program.Fset.Position(call.Pos())
						violations = append(violations, fmt.Sprintf("%s: %s.%s config Logger is not definitely bound by %s.AdaptLogrus", position, rule.PackagePath, rule.ConstructorName, rule.PackagePath))
					}
				}
				if rule, ok := rulesByNilCheckedFunction[functionKey]; ok && rule.Argument < len(call.Common().Args) && providerValueDefinitelyNil(call.Common().Args[rule.Argument], make(map[ssa.Value]bool)) {
					position := program.Fset.Position(call.Pos())
					violations = append(violations, fmt.Sprintf("%s: %s.%s argument %d is typed nil", position, rule.PackagePath, rule.FunctionName, rule.Argument))
				}
			}
		}
	}
	sort.Strings(violations)
	return violations
}

func loadedSourceFunctions(program *ssa.Program, loaded []*packages.Package) []*ssa.Function {
	loadedPaths := make(map[string]struct{}, len(loaded))
	functions := make(map[*ssa.Function]struct{})
	var addFunction func(*ssa.Function)
	addFunction = func(function *ssa.Function) {
		if function == nil || function.Blocks == nil || function.Syntax() == nil {
			return
		}
		if _, exists := functions[function]; exists {
			return
		}
		functions[function] = struct{}{}
		for _, anonymous := range function.AnonFuncs {
			addFunction(anonymous)
		}
	}
	for _, loadedPackage := range loaded {
		loadedPaths[loadedPackage.PkgPath] = struct{}{}
		for _, object := range loadedPackage.TypesInfo.Defs {
			functionObject, ok := object.(*types.Func)
			if ok {
				addFunction(program.FuncValue(functionObject))
			}
		}
	}
	for function := range ssautil.AllFunctions(program) {
		if function == nil || function.Pkg == nil {
			continue
		}
		if _, ok := loadedPaths[function.Pkg.Pkg.Path()]; ok {
			addFunction(function)
		}
	}
	result := make([]*ssa.Function, 0, len(functions))
	for function := range functions {
		result = append(result, function)
	}
	return result
}

func providerFunctionObject(function *ssa.Function) (*types.Func, bool) {
	if function == nil {
		return nil, false
	}
	if origin := function.Origin(); origin != nil {
		function = origin
	}
	object, ok := function.Object().(*types.Func)
	return object, ok
}

func providerConfigDefinitelyBound(function *ssa.Function, value ssa.Value, point ssa.Instruction, rule providerLoggerRule, helperStack map[*ssa.Function]bool) bool {
	switch typed := value.(type) {
	case *ssa.Alloc:
		return providerLoggerStateBefore(function, typed, point, rule, providerLoggerUnbound) == providerLoggerGood
	case *ssa.UnOp:
		if typed.Op == token.MUL {
			return providerConfigDefinitelyBound(function, typed.X, point, rule, helperStack)
		}
	case *ssa.ChangeType:
		return providerConfigDefinitelyBound(function, typed.X, point, rule, helperStack)
	case *ssa.Convert:
		return providerConfigDefinitelyBound(function, typed.X, point, rule, helperStack)
	case *ssa.ChangeInterface:
		return providerConfigDefinitelyBound(function, typed.X, point, rule, helperStack)
	case *ssa.MakeInterface:
		return providerConfigDefinitelyBound(function, typed.X, point, rule, helperStack)
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			return false
		}
		for _, edge := range typed.Edges {
			if !providerConfigDefinitelyBound(function, edge, point, rule, helperStack) {
				return false
			}
		}
		return true
	case *ssa.Call:
		initial := providerLoggerUnbound
		if providerHelperReturnsBound(typed.Common().StaticCallee(), 0, rule, helperStack) {
			initial = providerLoggerGood
		}
		return providerLoggerStateBefore(function, typed, point, rule, initial) == providerLoggerGood
	case *ssa.Extract:
		call, ok := typed.Tuple.(*ssa.Call)
		if !ok {
			return false
		}
		return providerHelperReturnsBound(call.Common().StaticCallee(), typed.Index, rule, helperStack)
	}
	return false
}

func providerHelperReturnsBound(function *ssa.Function, resultIndex int, rule providerLoggerRule, helperStack map[*ssa.Function]bool) bool {
	if function == nil || function.Blocks == nil || helperStack[function] {
		return false
	}
	helperStack[function] = true
	defer delete(helperStack, function)
	foundObject := false
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok || resultIndex >= len(returned.Results) {
				continue
			}
			result := returned.Results[resultIndex]
			if constant, ok := result.(*ssa.Const); ok && constant.IsNil() {
				continue
			}
			foundObject = true
			if !providerConfigDefinitelyBound(function, result, returned, rule, helperStack) {
				return false
			}
		}
	}
	return foundObject
}

func providerLoggerStateBefore(function *ssa.Function, target ssa.Value, point ssa.Instruction, rule providerLoggerRule, initial providerLoggerState) providerLoggerState {
	if function == nil || len(function.Blocks) == 0 || point == nil || point.Block() == nil {
		return providerLoggerBad
	}
	in := make(map[*ssa.BasicBlock]providerLoggerState, len(function.Blocks))
	out := make(map[*ssa.BasicBlock]providerLoggerState, len(function.Blocks))
	in[function.Blocks[0]] = initial
	changed := true
	for changed {
		changed = false
		for _, block := range function.Blocks {
			state := in[block]
			if block != function.Blocks[0] {
				state = 0
				for _, predecessor := range block.Preds {
					state |= out[predecessor]
				}
				if state != in[block] {
					in[block] = state
					changed = true
				}
			}
			next := transferProviderLoggerState(block.Instrs, state, target, point, rule)
			if next != out[block] {
				out[block] = next
				changed = true
			}
		}
	}
	state := in[point.Block()]
	for _, instruction := range point.Block().Instrs {
		if instruction == point {
			return state
		}
		state = transferProviderLoggerInstruction(instruction, state, target, point, rule)
	}
	return providerLoggerBad
}

func transferProviderLoggerState(instructions []ssa.Instruction, state providerLoggerState, target ssa.Value, point ssa.Instruction, rule providerLoggerRule) providerLoggerState {
	for _, instruction := range instructions {
		state = transferProviderLoggerInstruction(instruction, state, target, point, rule)
	}
	return state
}

func transferProviderLoggerInstruction(instruction ssa.Instruction, state providerLoggerState, target ssa.Value, point ssa.Instruction, rule providerLoggerRule) providerLoggerState {
	if instruction == point {
		return state
	}
	switch typed := instruction.(type) {
	case *ssa.Store:
		if field, ok := typed.Addr.(*ssa.FieldAddr); ok && providerValuesReferToSameObject(field.X, target) && providerFieldName(field) == rule.LoggerField {
			if providerValueComesFromAdapter(typed.Val, rule, make(map[ssa.Value]bool)) {
				return providerLoggerGood
			}
			return providerLoggerBad
		}
		if providerValuesReferToSameObject(typed.Addr, target) {
			return providerLoggerBad
		}
	case *ssa.Call:
		for _, argument := range typed.Common().Args {
			if providerValuesReferToSameObject(argument, target) {
				return providerLoggerBad
			}
		}
	}
	return state
}

func providerValuesReferToSameObject(value, target ssa.Value) bool {
	if value == target {
		return true
	}
	switch typed := value.(type) {
	case *ssa.UnOp:
		return typed.Op == token.MUL && providerValuesReferToSameObject(typed.X, target)
	case *ssa.ChangeType:
		return providerValuesReferToSameObject(typed.X, target)
	case *ssa.Convert:
		return providerValuesReferToSameObject(typed.X, target)
	case *ssa.ChangeInterface:
		return providerValuesReferToSameObject(typed.X, target)
	case *ssa.MakeInterface:
		return providerValuesReferToSameObject(typed.X, target)
	}
	return false
}

func providerFieldName(field *ssa.FieldAddr) string {
	pointer, ok := field.X.Type().Underlying().(*types.Pointer)
	if !ok {
		return ""
	}
	structure, ok := pointer.Elem().Underlying().(*types.Struct)
	if !ok || field.Field < 0 || field.Field >= structure.NumFields() {
		return ""
	}
	return structure.Field(field.Field).Name()
}

func providerValueComesFromAdapter(value ssa.Value, rule providerLoggerRule, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch typed := value.(type) {
	case *ssa.Call:
		callee := typed.Common().StaticCallee()
		object, ok := providerFunctionObject(callee)
		if !ok || object.Pkg() == nil || object.Pkg().Path() != rule.PackagePath || object.Name() != rule.AdapterName {
			return false
		}
		for _, argument := range typed.Common().Args {
			if providerValueDefinitelyNil(argument, make(map[ssa.Value]bool)) {
				return false
			}
		}
		return true
	case *ssa.ChangeType:
		return providerValueComesFromAdapter(typed.X, rule, seen)
	case *ssa.Convert:
		return providerValueComesFromAdapter(typed.X, rule, seen)
	case *ssa.ChangeInterface:
		return providerValueComesFromAdapter(typed.X, rule, seen)
	case *ssa.MakeInterface:
		return providerValueComesFromAdapter(typed.X, rule, seen)
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			return false
		}
		for _, edge := range typed.Edges {
			if !providerValueComesFromAdapter(edge, rule, seen) {
				return false
			}
		}
		return true
	}
	return false
}

func providerValueDefinitelyNil(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch typed := value.(type) {
	case *ssa.Const:
		return typed.IsNil()
	case *ssa.ChangeType:
		return providerValueDefinitelyNil(typed.X, seen)
	case *ssa.Convert:
		return providerValueDefinitelyNil(typed.X, seen)
	case *ssa.ChangeInterface:
		return providerValueDefinitelyNil(typed.X, seen)
	case *ssa.MakeInterface:
		return providerValueDefinitelyNil(typed.X, seen)
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			return false
		}
		for _, edge := range typed.Edges {
			if !providerValueDefinitelyNil(edge, seen) {
				return false
			}
		}
		return true
	}
	return false
}
