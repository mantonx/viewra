module github.com/mantonx/viewra/pkg/plugins

go 1.24

require (
	github.com/hashicorp/go-hclog v1.6.3
	github.com/hashicorp/go-plugin v1.6.0
	google.golang.org/grpc v1.73.0
	google.golang.org/protobuf v1.36.6
)

// Replace directive to point to the local protobuf module
replace github.com/mantonx/viewra/api/proto/enrichment => ../../api/proto/enrichment

require (
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/mantonx/viewra/api/proto/enrichment v0.0.0
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-testing-interface v0.0.0-20171004221916-a61a99592b77 // indirect
	github.com/oklog/run v1.0.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
)
