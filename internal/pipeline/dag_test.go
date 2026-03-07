package pipeline

import (
	"sort"
	"testing"
)

func makeWorkflow(name string, nodes []Node, connections map[string]NodeConnection) *Workflow {
	return &Workflow{
		ID:          name,
		Name:        name,
		Nodes:       nodes,
		Connections: connections,
	}
}

func conn(targets ...string) NodeConnection {
	var main [][]ConnectionTarget
	for _, t := range targets {
		main = append(main, []ConnectionTarget{{Node: t, Type: "main", Index: 0}})
	}
	return NodeConnection{Main: main}
}

func TestTopologicalSort_LinearChain(t *testing.T) {
	w := makeWorkflow("linear",
		[]Node{
			{Name: "a", Type: "x"},
			{Name: "b", Type: "x"},
			{Name: "c", Type: "x"},
		},
		map[string]NodeConnection{
			"a": conn("b"),
			"b": conn("c"),
		},
	)
	dag := BuildDAG(w)
	sorted, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx := make(map[string]int)
	for i, id := range sorted {
		idx[id] = i
	}
	if idx["a"] > idx["b"] || idx["b"] > idx["c"] {
		t.Errorf("invalid order: %v", sorted)
	}
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
	w := makeWorkflow("cyclic",
		[]Node{
			{Name: "a", Type: "x"},
			{Name: "b", Type: "x"},
			{Name: "c", Type: "x"},
		},
		map[string]NodeConnection{
			"a": conn("b"),
			"b": conn("c"),
			"c": conn("a"),
		},
	)
	dag := BuildDAG(w)
	_, err := dag.TopologicalSort()
	if err == nil {
		t.Error("expected cycle detection error")
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	w := makeWorkflow("nocycle",
		[]Node{
			{Name: "a", Type: "x"},
			{Name: "b", Type: "x"},
		},
		map[string]NodeConnection{
			"a": conn("b"),
		},
	)
	dag := BuildDAG(w)
	if err := dag.DetectCycles(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindRoots(t *testing.T) {
	w := makeWorkflow("roots",
		[]Node{
			{Name: "a", Type: "x"},
			{Name: "b", Type: "x"},
			{Name: "c", Type: "x"},
		},
		map[string]NodeConnection{
			"a": conn("c"),
			"b": conn("c"),
		},
	)
	dag := BuildDAG(w)
	roots := dag.FindRoots()
	sort.Strings(roots)
	if len(roots) != 2 || roots[0] != "a" || roots[1] != "b" {
		t.Errorf("roots = %v, want [a b]", roots)
	}
}

func TestParallelGroups_DiamondShape(t *testing.T) {
	w := makeWorkflow("diamond",
		[]Node{
			{Name: "a", Type: "x"},
			{Name: "b", Type: "x"},
			{Name: "c", Type: "x"},
			{Name: "d", Type: "x"},
		},
		map[string]NodeConnection{
			"a": conn("b", "c"),
			"b": conn("d"),
			"c": conn("d"),
		},
	)
	dag := BuildDAG(w)
	groups, err := dag.ParallelGroups()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %v", len(groups), groups)
	}
	if len(groups[0]) != 1 || groups[0][0] != "a" {
		t.Errorf("group 0 = %v, want [a]", groups[0])
	}
	g1 := make([]string, len(groups[1]))
	copy(g1, groups[1])
	sort.Strings(g1)
	if len(g1) != 2 || g1[0] != "b" || g1[1] != "c" {
		t.Errorf("group 1 = %v, want [b c]", g1)
	}
	if len(groups[2]) != 1 || groups[2][0] != "d" {
		t.Errorf("group 2 = %v, want [d]", groups[2])
	}
}

func TestParallelGroups_CycleError(t *testing.T) {
	w := makeWorkflow("cyclic",
		[]Node{
			{Name: "a", Type: "x"},
			{Name: "b", Type: "x"},
		},
		map[string]NodeConnection{
			"a": conn("b"),
			"b": conn("a"),
		},
	)
	dag := BuildDAG(w)
	_, err := dag.ParallelGroups()
	if err == nil {
		t.Error("expected error for cyclic graph")
	}
}
