module github.com/mantonx/viewra/pkg/transcode

go 1.22

require (
	github.com/mantonx/viewra/data/plugins/sdk v0.0.0
	github.com/mantonx/viewra/pkg/plugins v0.0.0
)

replace github.com/mantonx/viewra/data/plugins/sdk => ../../data/plugins/sdk
replace github.com/mantonx/viewra/pkg/plugins => ../plugins 