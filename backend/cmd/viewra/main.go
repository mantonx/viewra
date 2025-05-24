package main

import (
	"log"

	"github.com/yourusername/viewra/internal/server"
)

func main() {
	r := server.SetupRouter()
	log.Println("Starting Viewra server on :8080")
	r.Run(":8080")
}
