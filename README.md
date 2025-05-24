# Viewra

[![GitHub license](https://img.shields.io/github/license/mantonx/viewra)](https://github.com/mantonx/viewra/blob/main/LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/mantonx/viewra)](https://github.com/mantonx/viewra/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/mantonx/viewra)](https://github.com/mantonx/viewra/issues)
[![GitHub forks](https://img.shields.io/github/forks/mantonx/viewra)](https://github.com/mantonx/viewra/network)

A modern media management system built with React and Go, similar to Emby or Jellyfin.

## 🚧 Current Status

This project is in its initial setup phase with a basic hello world example demonstrating the full-stack architecture.

## 🚀 Quick Start

### Prerequisites
- Docker and Docker Compose
- Git

### Start Development Environment
```bash
# Clone the repository
git clone <your-repo-url>
cd viewra

# Start all services (this is all you need!)
./dev-compose.sh up

# Or start in background
./dev-compose.sh up -d
```

**That's it!**
- **Frontend**: http://localhost:5175
- **Backend API**: http://localhost:8081
- **Interactive UI**: API tester, system info, media upload

### Development Commands
```bash
./dev-compose.sh logs      # View logs from all services
./dev-compose.sh down      # Stop all services
./dev-compose.sh restart   # Restart services
./dev-compose.sh build     # Rebuild images
./dev-compose.sh clean     # Clean up everything
```

## 🧩 Architecture
- **Frontend**: React 19 + TypeScript + Tailwind CSS + Jotai + React Router + Vite
- **Backend**: Go + Gin framework + GORM + SQLite
- **Development**: Docker Compose with hot reloading
- **Database**: SQLite (plans for PostgreSQL support)

## 🛠️ Manual Setup (Alternative)

1. **Backend:**
   ```bash
   cd backend
   go mod tidy
   go run cmd/viewra/main.go
   ```
2. **Frontend:**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

## ⚙️ Environment Variables
You can customize the setup using environment variables (in your shell or a `.env` file):
```env
BACKEND_PORT=8081
FRONTEND_PORT=5175
VITE_API_URL=http://localhost:8081
DATABASE_TYPE=sqlite
SQLITE_PATH=/app/data/viewra.db
GIN_MODE=release
```

## 🧪 API Testing
Test the backend API endpoints:
```bash
curl http://localhost:8081/api/hello
curl http://localhost:8081/api/db-status
curl http://localhost:8081/api/users/
curl http://localhost:8081/api/media/
```

## 🐞 Troubleshooting
- **View logs:** `./dev-compose.sh logs` or `docker-compose logs <service>`
- **Rebuild everything:** `./dev-compose.sh clean && ./dev-compose.sh build && ./dev-compose.sh up`
- **Check service status:** `docker-compose ps`
- **Port conflicts:** Change ports in `.env` or export env vars before starting
- **Database issues:** Check backend logs, verify Docker volume
- **Frontend build issues:** Clear node_modules in container: `docker-compose exec frontend rm -rf node_modules && docker-compose exec frontend npm install`

## 📁 Project Structure
```
viewra/
├── backend/                 # Go backend
│   ├── cmd/viewra/         # Application entry point
│   ├── internal/server/    # HTTP server & routes
│   └── pkg/                # Shared packages (future)
├── frontend/               # React frontend
│   ├── src/
│   │   ├── components/     # Reusable components
│   │   ├── pages/          # Page components
│   │   └── store/          # Jotai state management
│   └── public/
├── docker-compose.yml      # Development environment
├── dev-compose.sh          # Helper script
└── ...
```

## 👥 Contributing
See [CONTRIBUTING.md](./CONTRIBUTING.md) for development guidelines and contribution instructions.

## 🏗️ Advanced/Production Notes
- This Docker Compose setup is for development only.
- For production: use `Dockerfile.prod`, configure secrets, set up a reverse proxy, and use a production database.
- See future docs for deployment and CI/CD.

## 📜 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
