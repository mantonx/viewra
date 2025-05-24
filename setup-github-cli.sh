#!/bin/bash

# GitHub CLI Setup Script for Viewra

echo "🚀 Setting up Viewra repository with GitHub CLI..."

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "📦 Installing GitHub CLI..."
    
    # For Ubuntu/Debian
    if command -v apt &> /dev/null; then
        curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
        echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
        sudo apt update
        sudo apt install gh
    # For Arch Linux
    elif command -v pacman &> /dev/null; then
        sudo pacman -S github-cli
    # For other systems
    else
        echo "❌ Please install GitHub CLI manually: https://cli.github.com/manual/installation"
        exit 1
    fi
fi

echo "🔐 Please authenticate with GitHub:"
gh auth login

echo "📝 Creating repository on GitHub..."
gh repo create viewra --public --description "A modern media management system built with React and Go, similar to Emby or Jellyfin" --source=.

echo "📤 Pushing to GitHub..."
git push -u origin main

echo "✅ Repository successfully created and pushed!"
echo "🌐 Opening repository in browser..."
gh repo view --web
