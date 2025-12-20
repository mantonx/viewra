module github.com/mantonx/viewra/plugins/musicbrainz

go 1.25.4

require (
	github.com/hashicorp/go-plugin v1.7.0
	github.com/mantonx/viewra/api/proto/plugin v0.0.0
	google.golang.org/grpc v1.67.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/yamux v0.1.2 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/oklog/run v1.1.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

// Use local proto package during development
// Remove this line when building against a released version
replace github.com/mantonx/viewra/api/proto/plugin => ../../api/proto/plugin
