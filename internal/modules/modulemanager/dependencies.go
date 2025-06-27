// Package modulemanager provides module dependency management and initialization ordering
package modulemanager

import (
	"fmt"
	"sort"

	"github.com/mantonx/viewra/internal/logger"
)

// DependencyProvider is an optional interface for modules that declare dependencies
type DependencyProvider interface {
	// Dependencies returns the list of module IDs this module depends on
	Dependencies() []string
}

// ServiceProvider is an optional interface for modules that provide services
type ServiceProvider interface {
	// ProvidedServices returns the list of service names this module provides
	ProvidedServices() []string
}

// ServiceConsumer is an optional interface for modules that consume services
type ServiceConsumer interface {
	// RequiredServices returns the list of service names this module requires
	RequiredServices() []string
}

// ModuleDependencyGraph represents the dependency relationships between modules
type ModuleDependencyGraph struct {
	nodes        map[string]*DependencyNode
	serviceGraph map[string]string // service name -> module ID that provides it
}

// DependencyNode represents a module in the dependency graph
type DependencyNode struct {
	ModuleID         string
	Module           Module
	Dependencies     []string // Module IDs this module depends on
	Dependents       []string // Module IDs that depend on this module
	ProvidedServices []string // Services provided by this module
	RequiredServices []string // Services required by this module
	Visited          bool     // For cycle detection
	InStack          bool     // For cycle detection
	InitOrder        int      // Order in which to initialize (lower = earlier)
}

// BuildDependencyGraph creates a dependency graph from registered modules
func BuildDependencyGraph(modules map[string]Module) (*ModuleDependencyGraph, error) {
	graph := &ModuleDependencyGraph{
		nodes:        make(map[string]*DependencyNode),
		serviceGraph: make(map[string]string),
	}

	// First pass: create nodes and collect service information
	for id, module := range modules {
		node := &DependencyNode{
			ModuleID:         id,
			Module:           module,
			Dependencies:     []string{},
			Dependents:       []string{},
			ProvidedServices: []string{},
			RequiredServices: []string{},
		}

		// Check if module declares dependencies
		if depProvider, ok := module.(DependencyProvider); ok {
			node.Dependencies = depProvider.Dependencies()
		}

		// Check if module provides services
		if serviceProvider, ok := module.(ServiceProvider); ok {
			node.ProvidedServices = serviceProvider.ProvidedServices()
			// Register services in the service graph
			for _, service := range node.ProvidedServices {
				if existingProvider, exists := graph.serviceGraph[service]; exists {
					return nil, fmt.Errorf("service '%s' is provided by multiple modules: %s and %s",
						service, existingProvider, id)
				}
				graph.serviceGraph[service] = id
			}
		}

		// Check if module requires services
		if serviceConsumer, ok := module.(ServiceConsumer); ok {
			node.RequiredServices = serviceConsumer.RequiredServices()
		}

		graph.nodes[id] = node
	}

	// Second pass: resolve service dependencies to module dependencies
	for id, node := range graph.nodes {
		for _, requiredService := range node.RequiredServices {
			if providerID, exists := graph.serviceGraph[requiredService]; exists {
				// Add the provider module as a dependency
				if providerID != id { // Don't add self-dependency
					node.Dependencies = append(node.Dependencies, providerID)
					logger.Debug("Module %s depends on %s for service '%s'", id, providerID, requiredService)
				}
			} else {
				logger.Warn("Module %s requires service '%s' but no provider found", id, requiredService)
			}
		}
	}

	// Third pass: build dependents lists
	for id, node := range graph.nodes {
		for _, depID := range node.Dependencies {
			if depNode, exists := graph.nodes[depID]; exists {
				depNode.Dependents = append(depNode.Dependents, id)
			} else {
				return nil, fmt.Errorf("module %s depends on non-existent module %s", id, depID)
			}
		}
	}

	// Check for cycles
	if err := graph.detectCycles(); err != nil {
		return nil, err
	}

	return graph, nil
}

// detectCycles uses DFS to detect dependency cycles
func (g *ModuleDependencyGraph) detectCycles() error {
	for id, node := range g.nodes {
		if !node.Visited {
			if err := g.detectCyclesDFS(id, []string{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// detectCyclesDFS performs depth-first search for cycle detection
func (g *ModuleDependencyGraph) detectCyclesDFS(nodeID string, path []string) error {
	node := g.nodes[nodeID]
	node.Visited = true
	node.InStack = true
	path = append(path, nodeID)

	for _, depID := range node.Dependencies {
		depNode := g.nodes[depID]
		if !depNode.Visited {
			if err := g.detectCyclesDFS(depID, path); err != nil {
				return err
			}
		} else if depNode.InStack {
			// Found a cycle
			cycleStart := -1
			for i, id := range path {
				if id == depID {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cyclePath := append(path[cycleStart:], depID)
				return fmt.Errorf("circular dependency detected: %v", cyclePath)
			}
		}
	}

	node.InStack = false
	return nil
}

// GetInitializationOrder returns modules in the order they should be initialized
func (g *ModuleDependencyGraph) GetInitializationOrder() ([]Module, error) {
	// Perform topological sort
	order := make([]Module, 0, len(g.nodes))
	visited := make(map[string]bool)

	// Helper function for DFS-based topological sort
	var visit func(string) error
	visit = func(nodeID string) error {
		if visited[nodeID] {
			return nil
		}

		node := g.nodes[nodeID]

		// Visit dependencies first
		for _, depID := range node.Dependencies {
			if err := visit(depID); err != nil {
				return err
			}
		}

		// Then add this node
		visited[nodeID] = true
		order = append(order, node.Module)
		node.InitOrder = len(order)

		return nil
	}

	// Start with nodes that have no dependencies
	for id, node := range g.nodes {
		if len(node.Dependencies) == 0 {
			if err := visit(id); err != nil {
				return nil, err
			}
		}
	}

	// Visit any remaining nodes (in case of disconnected components)
	for id := range g.nodes {
		if err := visit(id); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// PrintDependencyInfo logs dependency information for debugging
func (g *ModuleDependencyGraph) PrintDependencyInfo() {
	logger.Info("Module Dependency Information:")
	logger.Info("==============================")

	// Sort module IDs for consistent output
	var moduleIDs []string
	for id := range g.nodes {
		moduleIDs = append(moduleIDs, id)
	}
	sort.Strings(moduleIDs)

	for _, id := range moduleIDs {
		node := g.nodes[id]
		logger.Info("Module: %s", node.Module.Name())

		if len(node.Dependencies) > 0 {
			logger.Info("  Dependencies: %v", node.Dependencies)
		}

		if len(node.ProvidedServices) > 0 {
			logger.Info("  Provides: %v", node.ProvidedServices)
		}

		if len(node.RequiredServices) > 0 {
			logger.Info("  Requires: %v", node.RequiredServices)
		}

		if node.InitOrder > 0 {
			logger.Info("  Init Order: %d", node.InitOrder)
		}
	}

	logger.Info("")
	logger.Info("Service Registry:")
	logger.Info("=================")
	for service, provider := range g.serviceGraph {
		logger.Info("  %s -> %s", service, provider)
	}
}

// GetModuleDependencies returns the dependencies for a specific module
func (g *ModuleDependencyGraph) GetModuleDependencies(moduleID string) ([]string, error) {
	node, exists := g.nodes[moduleID]
	if !exists {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}
	return node.Dependencies, nil
}

// GetModuleDependents returns the modules that depend on a specific module
func (g *ModuleDependencyGraph) GetModuleDependents(moduleID string) ([]string, error) {
	node, exists := g.nodes[moduleID]
	if !exists {
		return nil, fmt.Errorf("module %s not found", moduleID)
	}
	return node.Dependents, nil
}

// ValidateServiceRequirements checks if all required services are available
func (g *ModuleDependencyGraph) ValidateServiceRequirements() []error {
	var errors []error

	for id, node := range g.nodes {
		for _, requiredService := range node.RequiredServices {
			if _, exists := g.serviceGraph[requiredService]; !exists {
				errors = append(errors, fmt.Errorf("module %s requires service '%s' but no provider found",
					id, requiredService))
			}
		}
	}

	return errors
}
