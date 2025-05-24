# Viewra

A modern media management system built with React and Go, similar to Emby or Jellyfin.

## 🚧 Current Status: Hello World Setup

This project is currently in its initial setup phase with a basic hello world example demonstrating the full-stack architecture.

## Architecture

- **Frontend**: React 19 + TypeScript + Tailwind CSS + Jotai (state management) + React Router
- **Backend**: Go + Gin framework
- **Development**: Vite + Docker Compose

## Quick Start

### Development Setup

1. **Clone and navigate:**

   ```bash
   git clone <your-repo>
   cd viewra
   ```

2. **Start with Docker Compose:**

   ```bash
   docker-compose up --build
   ```

3. **Or run manually:**

   Backend:

   ```bash
   cd backend
   go mod tidy
   go run cmd/viewra/main.go
   ```

   Frontend:

   ```bash
   cd frontend
   npm install
   npm run dev
   ```

4. **Access the application:**
   - Frontend: http://localhost:5173
   - Backend API: http://localhost:8080/api/health

## Current Features (Hello World)

- ✅ React frontend with dark theme
- ✅ Go backend with Gin framework
- ✅ API connection testing
- ✅ State management with Jotai
- ✅ Tailwind CSS styling
- ✅ Docker development environment
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
