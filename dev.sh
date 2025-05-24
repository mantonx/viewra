#!/bin/bash

# Viewra Development Helper Script

echo "🎬 Viewra Development Setup"
echo "=========================="

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
echo "Checking prerequisites..."

if ! command_exists go; then
    echo "❌ Go is not installed"
    exit 1
fi

if ! command_exists node; then
    echo "❌ Node.js is not installed"
    exit 1
fi

if ! command_exists docker; then
    echo "⚠️  Docker is not installed (optional for manual setup)"
fi

echo "✅ Prerequisites satisfied"
echo

# Menu
echo "Choose an option:"
echo "1. Start development servers manually"
echo "2. Start with Docker Compose"
echo "3. Start with Tilt (recommended)"
echo "4. Install dependencies only"
echo "5. Test API endpoints"
echo "6. Install Tilt"

read -p "Enter your choice (1-6): " choice

case $choice in
    1)
        echo "🚀 Starting development servers..."
        echo "Starting backend..."
        cd backend && go mod tidy
        gnome-terminal -- bash -c "cd backend && go run cmd/viewra/main.go; exec bash" 2>/dev/null || \
        osascript -e 'tell app "Terminal" to do script "cd '$PWD'/backend && go run cmd/viewra/main.go"' 2>/dev/null || \
        echo "Please manually run: cd backend && go run cmd/viewra/main.go"
        
        echo "Starting frontend..."
        cd ../frontend && npm install
        gnome-terminal -- bash -c "cd frontend && npm run dev; exec bash" 2>/dev/null || \
        osascript -e 'tell app "Terminal" to do script "cd '$PWD'/frontend && npm run dev"' 2>/dev/null || \
        echo "Please manually run: cd frontend && npm run dev"
        ;;
    2)
        echo "🐳 Starting with Docker Compose..."
        docker-compose up --build
        ;;
    3)
        echo "🎯 Starting with Tilt..."
        if ! command_exists tilt; then
            echo "❌ Tilt is not installed"
            echo "Run: ./install-tilt.sh"
            exit 1
        fi
        echo "Starting Tilt development environment..."
        echo "Tilt UI will be available at: http://localhost:10350"
        tilt up
        ;;
    4)
        echo "📦 Installing dependencies..."
        echo "Installing Go dependencies..."
        cd backend && go mod tidy
        echo "Installing Node dependencies..."
        cd ../frontend && npm install
        echo "✅ Dependencies installed"
        ;;
    5)
        echo "🧪 Testing API endpoints..."
        echo "Health check:"
        curl -s http://localhost:8080/api/health | jq 2>/dev/null || curl -s http://localhost:8080/api/health
        echo
        echo "Hello endpoint:"
        curl -s http://localhost:8080/api/hello
        echo
        ;;
    6)
        echo "🛠️  Installing Tilt..."
        ./install-tilt.sh
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac
