#!/bin/bash
# filepath: /home/fictional/Projects/viewra/frontend/docker-startup.sh

echo "🔍 Starting frontend container setup..."

# Check if node_modules exists and has proper permissions
if [ ! -d "node_modules" ] || [ ! -w "node_modules" ]; then
    echo "📦 Node modules missing or not writable, installing dependencies..."
    npm install
else
    echo "✅ Node modules directory exists and is writable"
    # Check if package.json is newer than node_modules
    if [ "package.json" -nt "node_modules" ] || [ "package-lock.json" -nt "node_modules" ]; then
        echo "📦 Package files updated, reinstalling dependencies..."
        npm install
    else
        echo "✅ Dependencies are up to date"
    fi
fi

# Fix any issues with lucide-react
echo "🛠️ Fixing lucide-react imports..."
if [ -d "node_modules/lucide-react" ]; then
  echo "✅ lucide-react package found"
else
  echo "⚠️ lucide-react package not found, installing..."
  npm install --save lucide-react
fi

# Run the development server
echo "🚀 Starting development server..."
npm run dev -- --host 0.0.0.0 --config vite.config.docker.ts
