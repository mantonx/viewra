# MCP (Model Context Protocol) Setup Guide

This guide explains how to configure MCP servers for enhanced AI assistant capabilities in Viewra development.

## Overview

MCP (Model Context Protocol) provides Claude with additional tools and capabilities through server integrations. Viewra includes configurations for several MCP servers that enhance development workflows.

## Available MCP Servers

### 1. GitHub Server
Provides GitHub operations directly within Claude:
- Repository management
- Pull request creation and review
- Issue tracking
- Code search across repositories
- Branch and fork operations

### 2. Filesystem Server
Enhanced file operations with better performance:
- Fast file reading and writing
- Multi-file operations
- Directory tree visualization
- File search and pattern matching
- Atomic file edits

### 3. Sequential Thinking Server
Structured problem-solving capabilities:
- Step-by-step reasoning
- Dynamic thought revision
- Context maintenance
- Hypothesis generation and verification
- Complex problem decomposition

### 4. Memory Server
Persistent knowledge graph across sessions:
- Entity and relationship tracking
- Project decision history
- User preference storage
- Contextual information retrieval

### 5. Context7 Server
Real-time library documentation:
- Current API documentation
- Version-specific information
- Framework and library references
- Accurate, up-to-date examples

## Setup Instructions

### 1. Environment Configuration

Create a `.env` file in the project root:
```bash
# GitHub Token for MCP
GITHUB_TOKEN=your_github_personal_access_token

# Optional: Configure other MCP servers
MCP_MEMORY_PATH=/path/to/memory/storage
MCP_FILESYSTEM_ALLOWED_PATHS=/home/user/projects
```

### 2. MCP Configuration File

The `.mcp.json` file is pre-configured for Viewra. Key sections:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-github"],
      "env": {
        "GITHUB_TOKEN": "${env:GITHUB_TOKEN}"
      }
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-filesystem"],
      "env": {
        "MCP_FILESYSTEM_ALLOWED_PATHS": "${env:MCP_FILESYSTEM_ALLOWED_PATHS}"
      }
    },
    "sequential-thinking": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-sequential-thinking"]
    }
  }
}
```

### 3. GitHub Token Setup

#### Creating a GitHub Personal Access Token:
1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Click "Generate new token" → "Generate new token (classic)"
3. Select scopes:
   - `repo` - Full repository access
   - `workflow` - GitHub Actions workflows
   - `read:org` - Organization membership
   - `gist` - Gist access (optional)
4. Generate and copy the token

#### Setting the Token:
```bash
# Option 1: Environment variable (recommended)
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx

# Option 2: Add to shell profile
echo 'export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx' >> ~/.bashrc
source ~/.bashrc

# Option 3: Use .env file
echo "GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx" >> .env
```

### 4. Verify Setup

After configuration, restart Claude Code and verify MCP servers are loaded:

1. Ask Claude: "Can you check if MCP servers are working?"
2. Test specific servers:
   - GitHub: "Can you list my recent GitHub repositories?"
   - Filesystem: "Can you show me the directory structure?"
   - Sequential Thinking: "Can you help me plan a complex feature?"

## Using MCP Servers

### GitHub Operations

```bash
# Claude can now:
- Create pull requests with proper formatting
- Search for code across the repository
- Create and manage issues
- Review pull request changes
- Manage branches and forks
```

Example commands:
- "Create a PR for the current changes"
- "Search for uses of the TranscodingService interface"
- "List open issues related to playback"

### Filesystem Operations

Enhanced file operations:
- "Read all Go files in the playback module"
- "Search for files containing 'TODO'"
- "Show me the directory structure of internal/modules"
- "Edit multiple files to update import paths"

### Sequential Thinking

For complex problems:
- "Help me design a new caching system for transcoded content"
- "Debug why sessions are not cleaning up properly"
- "Plan the implementation of distributed transcoding"

### Memory Operations

Persistent context:
- "Remember that we decided to use clean architecture for all modules"
- "What was our decision about error handling?"
- "Store this API design for future reference"

## Security Best Practices

### 1. Token Security
- **Never** commit tokens to version control
- Use environment variables over hardcoded values
- Rotate tokens regularly
- Use minimal required permissions

### 2. File Access
- Limit filesystem access to project directories
- Review allowed paths in MCP configuration
- Be cautious with write operations

### 3. Repository Access
- Only grant access to necessary repositories
- Use read-only tokens when possible
- Monitor token usage in GitHub settings

## Troubleshooting

### MCP Servers Not Loading

1. Check environment variables:
```bash
echo $GITHUB_TOKEN
```

2. Verify `.mcp.json` exists and is valid JSON

3. Check Claude Code logs for MCP errors

4. Restart Claude Code after configuration changes

### GitHub Token Issues

Common errors and solutions:
- "Bad credentials" - Token is invalid or expired
- "Resource not accessible" - Token lacks required permissions
- "Rate limit exceeded" - Too many API calls, wait or upgrade token

### Filesystem Access Denied

1. Check allowed paths configuration
2. Verify file permissions
3. Use absolute paths when configuring

## Advanced Configuration

### Custom MCP Servers

Add custom servers to `.mcp.json`:
```json
{
  "mcpServers": {
    "custom-tool": {
      "command": "python",
      "args": ["path/to/custom_mcp_server.py"],
      "env": {
        "CUSTOM_CONFIG": "${env:CUSTOM_CONFIG}"
      }
    }
  }
}
```

### Per-Project Configuration

Override global MCP settings:
1. Create project-specific `.mcp.json`
2. Set project-specific environment variables
3. Use direnv or similar tools for automatic loading

### Conditional Server Loading

Load servers based on environment:
```json
{
  "mcpServers": {
    "production-db": {
      "command": "mcp-postgres",
      "enabled": "${env:ENABLE_PROD_DB_MCP}"
    }
  }
}
```

## Best Practices

### 1. Use MCP for Repetitive Tasks
- File searches across the codebase
- Creating consistent PR descriptions
- Generating boilerplate code
- Managing project documentation

### 2. Combine MCP Servers
- Use Sequential Thinking with Filesystem for complex refactoring
- Combine GitHub with Memory for tracking decisions
- Use Context7 with Filesystem for implementing new libraries

### 3. Optimize Performance
- Batch file operations when possible
- Use specific search patterns
- Cache frequently accessed information in Memory

## Additional Resources

- [MCP Documentation](https://github.com/anthropics/mcp)
- [Creating Custom MCP Servers](https://github.com/anthropics/mcp/docs/custom-servers)
- [MCP Security Guide](https://github.com/anthropics/mcp/docs/security)

## Getting Help

If you encounter issues:
1. Check the troubleshooting section
2. Review Claude Code logs
3. Verify environment configuration
4. Ask Claude to diagnose MCP server status