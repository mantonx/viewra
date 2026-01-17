package main

import (
	"fmt"
	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/host"
)

func main() {
	ds := host.NewDataServer(nil, nil)
	fmt.Printf("DataServer type: %T\n", ds)
	
	// Check if it implements the gRPC interface
	_, ok := interface{}(ds).(pluginv1.HostDataServer)
	fmt.Printf("Implements HostDataServer: %v\n", ok)
}
