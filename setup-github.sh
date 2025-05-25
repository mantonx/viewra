#!/bin/bash

# GitHub Setup Script for Viewra
# Run this after creating the repository on GitHub

echo "🚀 Setting up GitHub remote for Viewra..."

# You'll need to replace 'mantonx' with your actual GitHub username
read -p "Enter your GitHub username: " username

if [ -z "$username" ]; then
    echo "❌ Username cannot be empty"
    exit 1
fi

echo "Setting up remote origin..."
git remote add origin https://github.com/$username/viewra.git

echo "Pushing to GitHub..."
git push -u origin main

echo "✅ Repository successfully pushed to GitHub!"
echo "🌐 Your repository is available at: https://github.com/$username/viewra"
echo ""
echo "Next steps:"
echo "1. Visit your repository on GitHub"
echo "2. Add collaborators if needed"
echo "3. Set up branch protection rules"
echo "4. Configure any GitHub Actions (optional)"
