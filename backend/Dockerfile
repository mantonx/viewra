# Development Dockerfile for Go backend
FROM golang:1.24-alpine

WORKDIR /app

# Install git for go modules
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Expose port
EXPOSE 8080

# Default command (can be overridden in docker-compose)
CMD ["go", "run", "cmd/viewra/main.go"]
