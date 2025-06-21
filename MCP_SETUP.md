# MCP (Model Context Protocol) Setup

This project includes MCP server configurations for enhanced AI assistant capabilities.

## Available MCP Servers

1. **GitHub** - For GitHub repository operations
2. **Filesystem** - For enhanced file operations within the project

## Setup Instructions

### 1. Environment Variables

Set the following environment variable in your shell or add it to your `.env` file:

```bash
export GITHUB_TOKEN=your_github_personal_access_token
```

### 2. Claude Code Configuration

The `.mcp.json` file is already configured, but if you need to add the GitHub token:

1. Copy `.mcp.json.example` to `.mcp.json` if it doesn't exist
2. Replace `YOUR_GITHUB_TOKEN_HERE` with your actual GitHub personal access token

### 3. Restart Claude Code

After setting up the environment variables, restart Claude Code to load the MCP servers.

## Security Note

Never commit your GitHub token to version control. The `.mcp.json` file in this repository has the token removed for security. You'll need to either:
- Set the `GITHUB_TOKEN` environment variable before starting Claude Code
- Manually add the token to your local `.mcp.json` file (not recommended)