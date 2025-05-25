#!/bin/bash
# filepath: /home/fictional/Projects/viewra/frontend/docker-startup.sh

echo "🔍 Starting frontend container setup..."

# Ensure node_modules is properly built
echo "📦 Installing npm dependencies..."
npm install

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
npm run dev -- --host 0.0.0.0
