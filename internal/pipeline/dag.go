package pipeline

import "fmt"

// DAG represents a directed acyclic graph of workflow nodes.
type DAG struct {
	NodeNames []string            // ordered node names
	Edges     map[string][]string // node name -> list of successor node names
	InDegree  map[string]int      // node name -> number of incoming edges
}

// BuildDAG constructs a DAG from a Workflow definition using its connections.
func BuildDAG(w *Workflow) *DAG {
	d := &DAG{
		Edges:    make(map[string][]string),
		InDegree: make(map[string]int),
	}

	// Initialize all nodes with zero in-degree.
	for _, node := range w.Nodes {
		d.NodeNames = append(d.NodeNames, node.Name)
		d.InDegree[node.Name] = 0
	}

	// Build edges from connections.
	for srcName, conn := range w.Connections {
		seen := make(map[string]bool)
		for _, outputs := range conn.Main {
			for _, target := range outputs {
				if !seen[target.Node] {
					seen[target.Node] = true
					d.Edges[srcName] = append(d.Edges[srcName], target.Node)
					d.InDegree[target.Node]++
				}
			}
		}
	}

	return d
}

// FindRoots returns nodes with no incoming edges (start nodes).
func (d *DAG) FindRoots() []string {
	var roots []string
	for _, name := range d.NodeNames {
		if d.InDegree[name] == 0 {
			roots = append(roots, name)
		}
	}
	return roots
}

// DetectCycles returns an error if the graph contains cycles.
func (d *DAG) DetectCycles() error {
	_, err := d.TopologicalSort()
	return err
}

// TopologicalSort returns the nodes in a valid execution order using Kahn's algorithm.
func (d *DAG) TopologicalSort() ([]string, error) {
	inDeg := make(map[string]int, len(d.InDegree))
	for id, deg := range d.InDegree {
		inDeg[id] = deg
	}

	var queue []string
	for _, name := range d.NodeNames {
		if inDeg[name] == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		for _, succ := range d.Edges[current] {
			inDeg[succ]--
			if inDeg[succ] == 0 {
				queue = append(queue, succ)
			}
		}
	}

	if len(sorted) != len(d.NodeNames) {
		return nil, fmt.Errorf("cycle detected in workflow DAG: sorted %d of %d nodes", len(sorted), len(d.NodeNames))
	}
	return sorted, nil
}

// ParallelGroups returns groups of nodes that can execute concurrently.
func (d *DAG) ParallelGroups() ([][]string, error) {
	inDeg := make(map[string]int, len(d.InDegree))
	for id, deg := range d.InDegree {
		inDeg[id] = deg
	}

	var groups [][]string
	remaining := len(d.NodeNames)

	for remaining > 0 {
		var group []string
		for _, name := range d.NodeNames {
			if deg, ok := inDeg[name]; ok && deg == 0 {
				group = append(group, name)
			}
		}
		if len(group) == 0 {
			return nil, fmt.Errorf("cycle detected in workflow DAG")
		}

		for _, id := range group {
			delete(inDeg, id)
			for _, succ := range d.Edges[id] {
				inDeg[succ]--
			}
		}

		groups = append(groups, group)
		remaining -= len(group)
	}

	return groups, nil
}
