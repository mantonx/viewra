module github.com/mantonx/viewra/plugins/audiodb_enricher

go 1.24.3

require (
	github.com/hashicorp/go-hclog v1.6.3
	github.com/mantonx/viewra/pkg/plugins v0.0.0
	gorm.io/driver/sqlite v1.5.7
	gorm.io/gorm v1.25.12
)

replace github.com/mantonx/viewra/pkg/plugins => ../../../pkg/plugins

require (
	github.com/fatih/color v1.13.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/hashicorp/go-plugin v1.6.2 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.24 // indirect
	github.com/oklog/run v1.1.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
