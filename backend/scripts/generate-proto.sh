#!/bin/bash

# Generate protobuf code for enrichment service
# This script generates Go code from the protobuf definitions

set -e

# Check if protoc is installed
if ! command -v protoc &> /dev/null; then
    echo "ERROR: protoc is not installed. Please install Protocol Buffers compiler."
    echo "On Ubuntu/Debian: sudo apt install protobuf-compiler"
    echo "On macOS: brew install protobuf"
    exit 1
fi

# Check if protoc-gen-go is installed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "ERROR: protoc-gen-go is not installed. Installing..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

# Check if protoc-gen-go-grpc is installed
if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "ERROR: protoc-gen-go-grpc is not installed. Installing..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

echo "Generating protobuf code for enrichment service..."

# Create output directory if it doesn't exist
mkdir -p api/proto/enrichment

# Generate Go code from protobuf
protoc --go_out=api/proto/enrichment \
       --go_opt=paths=source_relative \
       --go-grpc_out=api/proto/enrichment \
       --go-grpc_opt=paths=source_relative \
       api/proto/enrichment.proto

echo "Protobuf code generated successfully!"
echo ""
echo "Next steps:"
echo "1. Remove '// +build ignore' from internal/modules/enrichmentmodule/grpc_server.go"
echo "2. Remove '// +build ignore' from internal/plugins/enrichment/grpc_client.go"
echo "3. Update imports to use generated protobuf types"
echo "4. Test compilation: go build ./..."

echo "INFO: Generated files:"
echo "  - api/proto/enrichment/enrichment.pb.go"
echo "  - api/proto/enrichment/enrichment_grpc.pb.go"

echo ""
echo "Next steps:"
echo "1. Update the enrichment module to use the generated protobuf code"
echo "2. Update the gRPC server registration in the module"
echo "3. Update external plugins to use the gRPC client" 