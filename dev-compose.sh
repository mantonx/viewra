#!/bin/bash

# Viewra Development Environment with Docker Compose
# Simple and easy development setup

# Add local bin to PATH for docker-compose
export PATH=$PATH:~/.local/bin

echo "🎬 Starting Viewra development environment with Docker Compose..."

# Function to show help
show_help() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  up      Start the development environment (default)"
    echo "  down    Stop the development environment"
    echo "  logs    Show logs from all services"
    echo "  restart Restart all services"
    echo "  build   Rebuild all images"
    echo "  clean   Stop and remove all containers and volumes"
    echo "  help    Show this help message"
    echo ""
    echo "Services will be available at:"
    echo "  🌐 Frontend: http://localhost:5175"
    echo "  🔧 Backend:  http://localhost:8081"
}

# Parse command
COMMAND=${1:-up}

case $COMMAND in
    "up")
        echo "🚀 Starting services..."
        ~/.local/bin/docker-compose up -d
        echo ""
        echo "✅ Development environment is running!"
        echo "🌐 Frontend: http://localhost:5175"
        echo "🔧 Backend:  http://localhost:8081"
        echo ""
        echo "📋 Useful commands:"
        echo "  ./dev-compose.sh logs    - View logs"
        echo "  ./dev-compose.sh down    - Stop services"
        echo "  docker-compose logs -f   - Follow logs"
        ;;
    "down")
        echo "🛑 Stopping services..."
        ~/.local/bin/docker-compose down
        echo "✅ Services stopped"
        ;;
    "logs")
        echo "📋 Showing logs (Ctrl+C to exit)..."
        ~/.local/bin/docker-compose logs -f
        ;;
    "restart")
        echo "🔄 Restarting services..."
        ~/.local/bin/docker-compose restart
        echo "✅ Services restarted"
        ;;
    "build")
        echo "🔨 Rebuilding images..."
        ~/.local/bin/docker-compose build
        echo "✅ Images rebuilt"
        ;;
    "clean")
        echo "🧹 Cleaning up all containers and volumes..."
        ~/.local/bin/docker-compose down -v --remove-orphans
        docker system prune -f
        echo "✅ Cleanup complete"
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        echo "❌ Unknown command: $COMMAND"
        echo ""
        show_help
        exit 1
        ;;
esac
