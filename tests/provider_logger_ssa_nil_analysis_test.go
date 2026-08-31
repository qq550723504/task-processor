package tests

import "golang.org/x/tools/go/ssa"

type providerNilNode struct {
	value       ssa.Value
	function    *ssa.Function
	resultIndex int
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
		buildProviderNilValueFact(node.value, fact, facts)
		return
	}
	buildProviderNilResultFact(node.function, node.resultIndex, fact, facts)
}

func buildProviderNilValueFact(value ssa.Value, fact *providerNilFact, facts map[providerNilNode]*providerNilFact) {
	var dependencies []providerNilNode
	switch typed := value.(type) {
	case *ssa.Const:
		fact.unknown = !typed.IsNil()
		return
	case *ssa.ChangeType:
		dependencies = []providerNilNode{{value: typed.X}}
	case *ssa.Convert:
		dependencies = []providerNilNode{{value: typed.X}}
	case *ssa.ChangeInterface:
		dependencies = []providerNilNode{{value: typed.X}}
	case *ssa.MakeInterface:
		dependencies = []providerNilNode{{value: typed.X}}
	case *ssa.Phi:
		if len(typed.Edges) == 0 {
			fact.unknown = true
			return
		}
		dependencies = make([]providerNilNode, 0, len(typed.Edges))
		for _, edge := range typed.Edges {
			dependencies = append(dependencies, providerNilNode{value: edge})
		}
	case *ssa.Call:
		callee := typed.Common().StaticCallee()
		if callee == nil {
			fact.unknown = true
			return
		}
		dependencies = []providerNilNode{{function: callee, resultIndex: 0}}
	case *ssa.Extract:
		call, ok := typed.Tuple.(*ssa.Call)
		if !ok || call.Common().StaticCallee() == nil {
			fact.unknown = true
			return
		}
		dependencies = []providerNilNode{{function: call.Common().StaticCallee(), resultIndex: typed.Index}}
	default:
		fact.unknown = true
		return
	}
	fact.dependencies = dependencies
	for _, dependency := range dependencies {
		buildProviderNilFacts(dependency, facts)
	}
}

func buildProviderNilResultFact(function *ssa.Function, resultIndex int, fact *providerNilFact, facts map[providerNilNode]*providerNilFact) {
	if function == nil || function.Blocks == nil {
		fact.unknown = true
		return
	}
	foundReturn := false
	for _, block := range function.Blocks {
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
			dependency := providerNilNode{value: returned.Results[resultIndex]}
			fact.dependencies = append(fact.dependencies, dependency)
			buildProviderNilFacts(dependency, facts)
		}
	}
	if !foundReturn {
		fact.unknown = true
	}
}
