# Viewra - Your Modern Media Management Platform

Viewra is a flexible and extensible media management platform designed to help you organize, browse, and interact with your media library efficiently. It features a robust backend built in Go and a modern frontend using Vue.js.

## Key Features

- **Extensible Plugin System**: Customize and extend Viewra's functionality with powerful plugins. (See [Plugin Documentation](docs/PLUGINS.md))
- **Efficient Media Scanning**: Fast and reliable scanning of your media libraries.
- **Metadata Enrichment**: Leverage plugins (like MusicBrainz) to enrich your media files with detailed metadata.
- **Modern Web Interface**: A clean and user-friendly interface built with Vue.js.
- **Dockerized Deployment**: Easy to deploy and manage using Docker.
- **CueLang Configuration**: Type-safe and powerful configuration for plugins using CueLang.

## Tech Stack

- **Backend**: Go (Golang)
- **Frontend**: Vue.js, TypeScript
- **Plugin System**: HashiCorp go-plugin, gRPC, CueLang
- **Database**: (Specify primary database, e.g., SQLite, PostgreSQL - GORM is used for ORM)
- **Containerization**: Docker, Docker Compose

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go (version X.Y.Z) - for backend development or running outside Docker
- Node.js and npm/yarn - for frontend development
- `jq` and `curl` (for some utility scripts)

### Installation & Setup

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/mantonx/viewra.git
    cd viewra
    ```

2.  **Development Environment (Docker Compose):**
    The easiest way to get started for development is using Docker Compose. This will set up the backend, frontend, and any necessary services.

    ```bash
    # Make sure development startup scripts are executable
    chmod +x dev-compose.sh
    chmod +x backend/scripts/scanner/test-scanner.sh # If you plan to use it

    # Start the development environment
    ./dev-compose.sh up
    ```

    This typically starts:

    - Viewra Backend (with hot-reloading via Air)
    - Viewra Frontend (Vite dev server with hot-reloading)
    - (Any other services defined in `docker-compose.yml`)

3.  **Manual Setup (Backend):**
    If you prefer to run the backend manually:

    ```bash
    cd backend
    # Install dependencies (if not already handled by your Go environment)
    go mod download
    # Run the backend (example)
    go run cmd/viewra/main.go
    ```

4.  **Manual Setup (Frontend):**
    If you prefer to run the frontend manually:
    ```bash
    cd frontend
    npm install # or yarn install
    npm run dev # or yarn dev
    ```

### Accessing Viewra

- **Frontend**: Typically `http://localhost:5173` (Vite default) or `http://localhost:3000` (check `docker-compose.yml` or your frontend setup).
- **Backend API**: Typically `http://localhost:8080/api` (check `docker-compose.yml` or your backend setup).

## Usage

Once Viewra is running, navigate to the frontend URL in your web browser. You can start by:

1.  Configuring your media libraries through the admin interface.
2.  Initiating a scan of your libraries.
3.  Browsing your media.

## Plugin System

Viewra features a powerful plugin system that allows for significant customization and extension of its core capabilities. For detailed information on how the plugin system works and how to develop your own plugins, please refer to the [Plugin Documentation](docs/PLUGINS.md).

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for more details on how to get involved, coding standards, and the development process.

## Testing

Viewra aims for comprehensive test coverage.

- **Backend Testing**: Go tests are used for unit and integration testing. You can run backend tests using the Makefile targets:
  ```bash
  cd backend
  make test          # Run all backend tests
  make test-coverage # Generate coverage reports
  ```
- **Scanner Testing**: A specific script is available for testing the media scanner functionality. See [Scanner Testing Documentation](backend/scripts/scanner/README.md).
- **Frontend Testing**: (Details about frontend testing tools and commands would go here - e.g., Vitest, Cypress).

For more general information on testing, please refer to `docs/TESTING.md` (if this file exists and is up-to-date).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

_This README is a starting point. Please update it with more specific details about your project setup, database choices, and frontend testing procedures._
