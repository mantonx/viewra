package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	_ "github.com/mantonx/viewra/internal/modules/scannermodule" // Import to trigger registration
	"github.com/mantonx/viewra/internal/modules/scannermodule/scanner"
)

func main() {
	// Initialize database
	database.Initialize()
	db := database.GetDB()
	
	// Create event bus
	config := events.DefaultEventBusConfig()
	logger := &simpleLogger{}
	storage := events.NewDatabaseEventStorage(db)
	metrics := events.NewBasicEventMetrics()
	
	// Create and start event bus
	eventBus := events.NewEventBus(config, logger, storage, metrics)
	ctx := context.Background()
	if err := eventBus.Start(ctx); err != nil {
		log.Fatalf("Failed to start event bus: %v", err)
	}
	
	// Make event bus accessible globally
	events.SetGlobalEventBus(eventBus)
	
	// Dump registered modules
	log.Println("Registered modules before initialization:")
	for _, module := range modulemanager.ListModules() {
		log.Printf("- %s [%s] (core: %v)", module.Name(), module.ID(), module.Core())
	}
	
	// Initialize modules
	log.Println("Initializing modules...")
	if err := modulemanager.LoadAll(db); err != nil {
		log.Fatalf("Failed to initialize modules: %v", err)
	}
	
	// Get scanner module
	scannerModule, found := modulemanager.GetModule("system.scanner")
	if !found {
		log.Fatal("Scanner module not found!")
	}
	
	// Print module info
	log.Printf("Found scanner module: %s [%s] (core: %v)", 
		scannerModule.Name(), 
		scannerModule.ID(), 
		scannerModule.Core())
	
	// Check if scanner module implements RouteRegistrar
	log.Println("Checking if scanner module implements RouteRegistrar...")
	_, isRouteRegistrar := scannerModule.(modulemanager.RouteRegistrar)
	log.Printf("Is RouteRegistrar: %v", isRouteRegistrar)
	
	// Verify module implements the expected methods
	log.Println("\nVerifying module methods:")
	moduleType := reflect.TypeOf(scannerModule)
	for i := 0; i < moduleType.NumMethod(); i++ {
		method := moduleType.Method(i)
		log.Printf("- %s", method.Name)
	}
	
	// Create scanner manager
	scannerManager := scanner.NewManager(db, nil) // nil event bus for testing
	
	// Test the new SetParallelMode method
	scannerManager.SetParallelMode(true)
	fmt.Printf("Scanner manager created with parallel mode: %v\n", scannerManager.GetParallelMode())

	log.Println("\nTest completed successfully!")
}

// Simple logger for event bus
type simpleLogger struct{}

func (l *simpleLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *simpleLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func (l *simpleLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *simpleLogger) Debug(msg string, args ...interface{}) {
	log.Printf("[DEBUG] "+msg, args...)
}
