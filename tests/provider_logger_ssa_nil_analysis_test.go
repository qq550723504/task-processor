package tests

import (
	"go/constant"

	"golang.org/x/tools/go/ssa"
)

type providerNilNode struct {
	value       ssa.Value
	function    *ssa.Function
	resultIndex int
	callContext *ssa.Call
}

type providerNilFact struct {
	dependencies []providerNilNode
	unknown      bool
}

func providerValueDefinitelyNil(value ssa.Value) bool {
	if value == nil {
		return false
	}
	root := providerNilNode{value: value}
	facts := make(map[providerNilNode]*providerNilFact)
	buildProviderNilFacts(root, facts)

	allNil := make(map[providerNilNode]bool, len(facts))
	for node, fact := range facts {
		allNil[node] = !fact.unknown
	}
	changed := true
	for changed {
		changed = false
		for node, fact := range facts {
			if !allNil[node] {
				continue
			}
			for _, dependency := range fact.dependencies {
				if !allNil[dependency] {
					allNil[node] = false
					changed = true
					break
				}
			}
		}
	}
	return allNil[root]
}

func buildProviderNilFacts(node providerNilNode, facts map[providerNilNode]*providerNilFact) {
	if _, exists := facts[node]; exists {
		return
	}
	fact := &providerNilFact{}
	facts[node] = fact
	if node.value != nil {
		buildProviderNilValueFact(node, fact, facts)
		return
	}
	buildProviderNilResultFact(node, fact, facts)
}

func buildProviderNilValueFact(node providerNilNode, fact *providerNilFact, facts map[providerNilNode]*providerNilFact) {
	var dependencies []providerNilNode
	switch typed := node.value.(type) {
	case *ssa.Const:
		fact.unknown = !typed.IsNil()
		return
	case *ssa.Parameter:
		fact.unknown = true
		return
	case *ssa.ChangeType:
		dependencies = []providerNilNode{{value: typed.X, callContext: node.callContext}}
	case *ssa.Convert:
		dependencies = []providerNilNode{{value: typed.X, callContext: node.callContext}}
	case *ssa.ChangeInterface:
		dependencies = []providerNilNode{{value: typed.X, callContext: node.callContext}}
	case *ssa.MakeInterface:
		dependencies = []providerNilNode{{value: typed.X, callContext: node.callContext}}
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			fact.unknown = true
			return
		}
		dependencies = make([]providerNilNode, 0, len(typed.Edges))
		for _, edge := range typed.Edges {
			dependencies = append(dependencies, providerNilNode{value: edge, callContext: node.callContext})
		}
	case *ssa.Call:
		callee := typed.Common().StaticCallee()
		if callee == nil {
			fact.unknown = true
			return
		}
		bindProviderNilCallParameters(callee, typed, node.callContext, facts)
		dependencies = []providerNilNode{{function: callee, resultIndex: 0, callContext: typed}}
	case *ssa.Extract:
		call, ok := typed.Tuple.(*ssa.Call)
		if !ok || call.Common().StaticCallee() == nil {
			fact.unknown = true
			return
		}
		callee := call.Common().StaticCallee()
		bindProviderNilCallParameters(callee, call, node.callContext, facts)
		dependencies = []providerNilNode{{function: callee, resultIndex: typed.Index, callContext: call}}
	default:
		fact.unknown = true
		return
	}
	fact.dependencies = dependencies
	for _, dependency := range dependencies {
		buildProviderNilFacts(dependency, facts)
	}
}

func bindProviderNilCallParameters(function *ssa.Function, call *ssa.Call, callerContext *ssa.Call, facts map[providerNilNode]*providerNilFact) {
	arguments := call.Common().Args
	for index, parameter := range function.Params {
		parameterNode := providerNilNode{value: parameter, callContext: call}
		parameterFact, exists := facts[parameterNode]
		if !exists {
			parameterFact = &providerNilFact{}
			facts[parameterNode] = parameterFact
		}
		if index >= len(arguments) {
			parameterFact.unknown = true
			continue
		}
		dependency := providerNilNode{value: arguments[index], callContext: callerContext}
		addProviderNilDependency(parameterFact, dependency)
		buildProviderNilFacts(dependency, facts)
	}
}

func addProviderNilDependency(fact *providerNilFact, dependency providerNilNode) {
	for _, existing := range fact.dependencies {
		if existing == dependency {
			return
		}
	}
	fact.dependencies = append(fact.dependencies, dependency)
}

func buildProviderNilResultFact(node providerNilNode, fact *providerNilFact, facts map[providerNilNode]*providerNilFact) {
	function := node.function
	resultIndex := node.resultIndex
	if function == nil || len(function.Blocks) == 0 {
		fact.unknown = true
		return
	}
	foundReturn := false
	for _, block := range providerNilReachableBlocks(function) {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok {
				continue
			}
			if resultIndex < 0 || resultIndex >= len(returned.Results) {
				fact.unknown = true
				return
			}
			foundReturn = true
			dependency := providerNilNode{value: returned.Results[resultIndex], callContext: node.callContext}
			fact.dependencies = append(fact.dependencies, dependency)
			buildProviderNilFacts(dependency, facts)
		}
	}
	if !foundReturn {
		fact.unknown = true
	}
}

func providerNilReachableBlocks(function *ssa.Function) []*ssa.BasicBlock {
	if function == nil || len(function.Blocks) == 0 {
		return nil
	}
	seen := make(map[*ssa.BasicBlock]struct{}, len(function.Blocks))
	pending := []*ssa.BasicBlock{function.Blocks[0]}
	blocks := make([]*ssa.BasicBlock, 0, len(function.Blocks))
	for len(pending) > 0 {
		last := len(pending) - 1
		block := pending[last]
		pending = pending[:last]
		if block == nil {
			continue
		}
		if _, exists := seen[block]; exists {
			continue
		}
		seen[block] = struct{}{}
		blocks = append(blocks, block)
		pending = append(pending, providerNilReachableSuccessors(block)...)
	}
	return blocks
}

func providerNilReachableSuccessors(block *ssa.BasicBlock) []*ssa.BasicBlock {
	if block == nil {
		return nil
	}
	if len(block.Instrs) == 0 || len(block.Succs) != 2 {
		return block.Succs
	}
	branch, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok {
		return block.Succs
	}
	condition, ok := branch.Cond.(*ssa.Const)
	if !ok || condition.Value == nil || condition.Value.Kind() != constant.Bool {
		return block.Succs
	}
	if constant.BoolVal(condition.Value) {
		return block.Succs[:1]
	}
	return block.Succs[1:]
}
