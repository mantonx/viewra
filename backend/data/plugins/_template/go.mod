module github.com/mantonx/viewra/plugins/template

go 1.24.3

require (
	github.com/hashicorp/go-hclog v1.6.2
	github.com/hashicorp/go-plugin v1.6.0
	github.com/mantonx/viewra v0.0.0
)

// Local replace for development
replace github.com/mantonx/viewra => ../../..

// TODO: Add your plugin-specific dependencies here 