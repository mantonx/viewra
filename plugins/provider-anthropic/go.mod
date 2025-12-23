module github.com/mantonx/viewra/plugins/provider-anthropic

go 1.25.4

require (
	github.com/anthropics/anthropic-sdk-go v1.19.0
	github.com/hashicorp/go-plugin v1.7.0
	github.com/mantonx/viewra/api/proto/plugin v0.0.0
	github.com/mantonx/viewra/pkg/plugin/sdk v0.0.0
	google.golang.org/grpc v1.72.0
)

require (
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

// Use local packages during development
replace github.com/mantonx/viewra/api/proto/plugin => ../../api/proto/plugin

replace github.com/mantonx/viewra/pkg/plugin/sdk => ../../pkg/plugin/sdk
