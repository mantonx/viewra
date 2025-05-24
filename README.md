# Viewra

[![GitHub license](https://img.shields.io/github/license/mantonx/viewra)](https://github.com/mantonx/viewra/blob/main/LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/mantonx/viewra)](https://github.com/mantonx/viewra/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/mantonx/viewra)](https://github.com/mantonx/viewra/issues)
[![GitHub forks](https://img.shields.io/github/forks/mantonx/viewra)](https://github.com/mantonx/viewra/network)

A modern media management system built with React and Go, similar to Emby or Jellyfin.

## 🚧 Current Status: Hello World Setup

This project is currently in its initial setup phase with a basic hello world example demonstrating the full-stack architecture.

## Architecture

- **Frontend**: React 19 + TypeScript + Tailwind CSS + Jotai (state management) + React Router
- **Backend**: Go + Gin framework
- **Development**: Vite + Docker Compose

## 🚀 Quick Start

### Option 1: Development Helper (Recommended)
```bash
chmod +x dev.sh
./dev.sh
# Choose option 3 for Tilt (recommended) or 2 for Docker Compose
```

### Option 2: Tilt (Advanced Development)
```bash
# Install Tilt
./install-tilt.sh

# Start development environment
tilt up
# Open http://localhost:10350 for Tilt UI
```

### Option 3: Docker Compose
```bash
docker-compose up --build
```

### Option 4: Manual Setup

### Option 4: Manual Setup

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

3. **Access the application:**
   - Frontend: http://localhost:5173
   - Backend API: http://localhost:8080/api/health

## Current Features (Hello World)

- ✅ React frontend with dark theme
- ✅ Go backend with Gin framework
- ✅ API connection testing
- ✅ State management with Jotai
- ✅ Tailwind CSS styling
- ✅ Docker development environment
- ✅ Tilt development orchestration
- ✅ TypeScript setup

## Planned Features (Media Manager)

- 🎬 Media library management
- 📹 Video streaming & playback
- 👤 User authentication & profiles
- 🔍 Metadata scraping & search
- 📱 Responsive mobile interface
- 🎨 Customizable themes
- 📊 Watch history & statistics

## Project Structure

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
└── docker-compose.yml      # Development environment
```

## Development

The project uses modern development practices and is set up for future expansion into a full media management system.
