package main

import (
	"log"

	_ "github.com/mantonx/viewra/docs/swagger" // Import generated docs
	"github.com/mantonx/viewra/cmd/viewra/bootstrap"
)

// @title           ViewRA Media Server API
// @version         0.0.1
// @description     Self-hosted media server for movies, TV shows, and music
// @termsOfService  http://swagger.io/terms/

// @contact.name   ViewRA Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @schemes http https

func main() {
	// Initialize application with all dependencies
	app, err := bootstrap.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Run application (blocks until shutdown signal)
	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
