package scheduler

import (
	"fmt"
	"sort"
)

// DAG represents a directed acyclic graph of task dependencies.
type DAG struct {
	nodes map[string]*DAGNode
}

// DAGNode represents a node in the dependency graph.
type DAGNode struct {
	TaskID     string   `json:"task_id"`
	TaskName   string   `json:"task_name"`
	DependsOn  []string `json:"depends_on"` // Parent task IDs (this task depends on)
	Dependents []string `json:"dependents"` // Child task IDs (tasks that depend on this)
}

// NewDAG creates a new DAG from a list of tasks.
func NewDAG(tasks []*Task) (*DAG, error) {
	dag := &DAG{
		nodes: make(map[string]*DAGNode),
	}

	// Create nodes for all tasks
	for _, task := range tasks {
		dag.nodes[task.ID] = &DAGNode{
			TaskID:     task.ID,
			TaskName:   task.Name,
			DependsOn:  task.DependsOn,
			Dependents: []string{},
		}
	}

	// Build reverse edges (dependents)
	for _, task := range tasks {
		for _, depID := range task.DependsOn {
			if parent, ok := dag.nodes[depID]; ok {
				parent.Dependents = append(parent.Dependents, task.ID)
			}
			// Note: we don't error on missing dependencies here,
			// validation happens separately
		}
	}

	// Validate no cycles
	if err := dag.detectCycle(); err != nil {
		return nil, err
	}

	return dag, nil
}

// GetNode returns a node by task ID.
func (d *DAG) GetNode(taskID string) *DAGNode {
	return d.nodes[taskID]
}

// GetDependents returns all task IDs that depend on the given task.
func (d *DAG) GetDependents(taskID string) []string {
	if node := d.nodes[taskID]; node != nil {
		return node.Dependents
	}
	return nil
}

// GetDependencies returns all task IDs that the given task depends on.
func (d *DAG) GetDependencies(taskID string) []string {
	if node := d.nodes[taskID]; node != nil {
		return node.DependsOn
	}
	return nil
}

// GetAllDependents returns all transitive dependents of a task (recursively).
func (d *DAG) GetAllDependents(taskID string) []string {
	visited := make(map[string]bool)
	result := []string{}

	var visit func(id string)
	visit = func(id string) {
		for _, depID := range d.GetDependents(id) {
			if !visited[depID] {
				visited[depID] = true
				result = append(result, depID)
				visit(depID)
			}
		}
	}

	visit(taskID)
	return result
}

// GetAllDependencies returns all transitive dependencies of a task (recursively).
func (d *DAG) GetAllDependencies(taskID string) []string {
	visited := make(map[string]bool)
	result := []string{}

	var visit func(id string)
	visit = func(id string) {
		for _, depID := range d.GetDependencies(id) {
			if !visited[depID] {
				visited[depID] = true
				result = append(result, depID)
				visit(depID)
			}
		}
	}

	visit(taskID)
	return result
}

// TopologicalSort returns tasks in execution order (dependencies before dependents).
func (d *DAG) TopologicalSort() ([]string, error) {
	// Kahn's algorithm
	inDegree := make(map[string]int)
	for id, node := range d.nodes {
		inDegree[id] = len(node.DependsOn)
	}

	// Find all nodes with no dependencies
	queue := []string{}
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Sort for deterministic output
	sort.Strings(queue)

	result := []string{}
	for len(queue) > 0 {
		// Take from front
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree of dependents
		for _, depID := range d.GetDependents(current) {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
				sort.Strings(queue) // Keep sorted for determinism
			}
		}
	}

	// If we didn't process all nodes, there's a cycle
	if len(result) != len(d.nodes) {
		return nil, ErrCyclicDependency
	}

	return result, nil
}

// detectCycle checks if the graph contains a cycle using DFS.
func (d *DAG) detectCycle() error {
	const (
		white = 0 // Not visited
		gray  = 1 // Currently visiting (in stack)
		black = 2 // Finished visiting
	)

	color := make(map[string]int)
	for id := range d.nodes {
		color[id] = white
	}

	var dfs func(id string, path []string) error
	dfs = func(id string, path []string) error {
		color[id] = gray
		path = append(path, id)

		for _, depID := range d.GetDependencies(id) {
			if _, exists := d.nodes[depID]; !exists {
				// Dependency doesn't exist - skip (will be caught by validation)
				continue
			}

			if color[depID] == gray {
				// Found a cycle
				cycleStart := -1
				for i, p := range path {
					if p == depID {
						cycleStart = i
						break
					}
				}
				cycle := append(path[cycleStart:], depID)
				return fmt.Errorf("%w: %v", ErrCyclicDependency, cycle)
			}

			if color[depID] == white {
				if err := dfs(depID, path); err != nil {
					return err
				}
			}
		}

		color[id] = black
		return nil
	}

	// Process all nodes (handles disconnected components)
	for id := range d.nodes {
		if color[id] == white {
			if err := dfs(id, nil); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateDependencies checks that all dependencies exist.
func (d *DAG) ValidateDependencies() []string {
	missing := []string{}
	for _, node := range d.nodes {
		for _, depID := range node.DependsOn {
			if _, exists := d.nodes[depID]; !exists {
				missing = append(missing, fmt.Sprintf("%s depends on missing task %s", node.TaskID, depID))
			}
		}
	}
	return missing
}

// Nodes returns all nodes in the graph.
func (d *DAG) Nodes() []*DAGNode {
	result := make([]*DAGNode, 0, len(d.nodes))
	for _, node := range d.nodes {
		result = append(result, node)
	}
	return result
}

// RootsReady returns task IDs that have no dependencies or all dependencies satisfied.
// satisfiedDeps is a map of task IDs that have successfully completed.
func (d *DAG) RootsReady(satisfiedDeps map[string]bool) []string {
	ready := []string{}
	for id, node := range d.nodes {
		if len(node.DependsOn) == 0 {
			ready = append(ready, id)
			continue
		}

		allSatisfied := true
		for _, depID := range node.DependsOn {
			if !satisfiedDeps[depID] {
				allSatisfied = false
				break
			}
		}
		if allSatisfied {
			ready = append(ready, id)
		}
	}
	return ready
}
