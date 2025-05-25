# Contributing to Viewra

Thank you for your interest in contributing to Viewra! This document provides guidelines and information for contributors.

## 🚀 Getting Started

### Prerequisites

- Go 1.21 or later
- Node.js 20 or later
- Docker and Docker Compose (recommended)
- Git

### Development Setup

1. Fork the repository
2. Clone your fork: `git clone https://github.com/mantonx/viewra.git`
3. Navigate to the project: `cd viewra`

#### Option 1: Docker Compose (Recommended)

```bash
# Start all services with hot reloading
./dev-compose.sh up

# Or use the dev helper script
./dev.sh
# Choose option 2 for Docker Compose
```

#### Option 2: Manual Setup

1. Install dependencies:

   ```bash
   cd backend && go mod tidy
   cd ../frontend && npm install
   ```

2. Start development servers:

   ```bash
   # Terminal 1 - Backend
   cd backend && SQLITE_PATH=./data/viewra.db PORT=8081 go run cmd/viewra/main.go

   # Terminal 2 - Frontend
   cd frontend && npm run dev
   ```

### Accessing the Application

- **Frontend**: http://localhost:5175 (Docker Compose) or http://localhost:5173 (manual)
- **Backend API**: http://localhost:8081

## 📋 Development Guidelines

### Code Style

- **Go**: Follow standard Go formatting (use `gofmt`)
- **TypeScript/React**: Use ESLint and Prettier configurations
- **Git Commits**: Use conventional commit messages

### Project Structure

```
viewra/
├── backend/                 # Go backend
│   ├── cmd/viewra/         # Application entry point
│   ├── internal/server/    # HTTP server & routes
│   └── pkg/                # Shared packages
├── frontend/               # React frontend
│   ├── src/
│   │   ├── components/     # Reusable components
│   │   ├── pages/          # Page components
│   │   └── store/          # Jotai state management
│   └── public/
└── docker-compose.yml      # Development environment
```

### Branch Naming

- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation updates
- `refactor/description` - Code refactoring

### Development Workflow

1. Make changes to your code
2. Test locally using Docker Compose or manual setup
3. Run tests to ensure everything works
4. Commit your changes with descriptive messages
5. Push to your fork and create a Pull Request

## 🧪 Testing

- Run Go tests: `cd backend && go test ./...`
- Run frontend tests: `cd frontend && npm test`
- Test API endpoints: `./dev.sh` (choose option 4)
- Ensure all tests pass before submitting PR

## 📝 Submitting Changes

1. Create a feature branch from `main`
2. Make your changes following the guidelines above
3. Test your changes thoroughly
4. Commit with descriptive messages
5. Push to your fork
6. Create a Pull Request

### Pull Request Guidelines

- Fill out the PR template completely
- Include screenshots for UI changes
- Reference any related issues
- Ensure CI passes
- Request review from maintainers

## 🎯 Areas for Contribution

### Current Priority Areas

- [ ] Media library scanning and management
- [ ] Video streaming and playback
- [ ] User authentication system
- [ ] Metadata scraping integration
- [ ] Mobile-responsive UI improvements
- [ ] Performance optimizations

### Future Areas

- [ ] Subtitle support
- [ ] Multi-user management
- [ ] Plugin system
- [ ] Advanced search and filtering
- [ ] Watch party features

## 🐛 Reporting Issues

Before creating an issue:

1. Check if the issue already exists
2. Use the appropriate issue template
3. Provide detailed reproduction steps
4. Include environment information

## 💬 Getting Help

- GitHub Issues for bug reports and feature requests
- GitHub Discussions for questions and community chat

## 📜 License

By contributing to Viewra, you agree that your contributions will be licensed under the same license as the project.

---

Thank you for helping make Viewra better! 🎬
