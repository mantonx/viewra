# MCP (Model Context Protocol) Setup

This project includes MCP server configurations for enhanced AI assistant capabilities.

## Available MCP Servers

1. **GitHub** - For GitHub repository operations
2. **Filesystem** - For enhanced file operations within the project
3. **Sequential Thinking** - For structured problem-solving and step-by-step reasoning

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

## Sequential Thinking MCP Server

The Sequential Thinking MCP server enables Claude to engage in structured, step-by-step problem-solving. It's particularly useful for:

- Breaking down complex problems into manageable steps
- Planning and design tasks that may require revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Tasks requiring context maintenance across multiple reasoning steps

### How It Works

The server provides a `sequential_thinking` tool that allows Claude to:
1. Generate thoughts step-by-step
2. Track progress through numbered thoughts
3. Revise previous thoughts as understanding deepens
4. Branch into alternative reasoning paths
5. Maintain context throughout the problem-solving process

### When to Use

Sequential Thinking is ideal for:
- Complex architectural decisions
- Debugging intricate issues
- Planning multi-step implementations
- Analyzing problems with unclear requirements
- Tasks requiring iterative refinement

## Security Note

Never commit your GitHub token to version control. The `.mcp.json` file in this repository has the token removed for security. You'll need to either:
- Set the `GITHUB_TOKEN` environment variable before starting Claude Code
- Manually add the token to your local `.mcp.json` file (not recommended)