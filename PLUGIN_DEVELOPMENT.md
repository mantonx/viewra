# Plugin Development Guide

This guide covers the streamlined development workflow for Viewra transcoding plugins.

## 🚀 Quick Start

### 1. Setup Development Environment
```bash
# Option 1: Use the comprehensive setup
make plugin-dev

# Option 2: Manual setup
make plugin-setup
```

This will:
- Start Docker Compose if not running
- Build all transcoding plugins in container environment
- Refresh plugin discovery
- List available plugins with status

### 2. Develop a Plugin

**Build specific plugin:**
```bash
make plugin-build p=ffmpeg_software
# or
./scripts/plugin-dev.sh build ffmpeg_software
```

**Hot reload plugin (disable → rebuild → enable):**
```bash
make plugin-reload p=ffmpeg_software
# or  
./scripts/plugin-dev.sh reload ffmpeg_software
```

**Enable/disable plugins:**
```bash
make plugin-enable p=ffmpeg_software
make plugin-disable p=ffmpeg_software
```

**Test plugin functionality:**
```bash
make plugin-test p=ffmpeg_software
```

**List all transcoding plugins:**
```bash
make plugin-list
# or
./scripts/plugin-dev.sh list
```

## 🔧 Available Transcoding Plugins

| Plugin | Hardware | Priority | Description |
|--------|----------|----------|-------------|
| `ffmpeg_software` | CPU | 10 | High-quality software transcoding |
| `ffmpeg_nvidia` | NVENC | 90 | NVIDIA hardware acceleration |
| `ffmpeg_vaapi` | Intel GPU | 80 | Intel VAAPI acceleration |
| `ffmpeg_qsv` | Intel QSV | 85 | Intel Quick Sync Video |

## 📂 Plugin Structure

Each plugin follows this structure:
```
plugins/ffmpeg_software/
├── main.go           # Main plugin implementation
├── go.mod           # Go module definition
├── go.sum           # Go module dependencies
└── plugin.cue       # Plugin configuration
```

## 🛠️ Development Workflow

### Method 1: Command Line
```bash
# 1. Setup environment
./scripts/plugin-dev.sh setup

# 2. Build and test plugin
./scripts/plugin-dev.sh build ffmpeg_software
./scripts/plugin-dev.sh enable ffmpeg_software
./scripts/plugin-dev.sh test ffmpeg_software

# 3. Make changes to code...

# 4. Hot reload
./scripts/plugin-dev.sh reload ffmpeg_software
```

### Method 2: Make Commands
```bash
# Setup
make plugin-dev

# Build specific plugin  
make plugin-build p=ffmpeg_software

# Hot reload after changes
make plugin-reload p=ffmpeg_software

# List status
make plugin-list
```

### Method 3: VS Code Tasks
- Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on Mac)
- Type "Tasks: Run Task"
- Select from available plugin tasks:
  - `Plugin: Setup Development Environment`
  - `Plugin: Build Software Transcoder`
  - `Plugin: Hot Reload Software Transcoder`
  - `Plugin: List All Transcoders`
  - `Plugin: Test Software Transcoder`

## 🔄 Hot Reload Process

The hot reload system automatically:
1. **Disables** the plugin (if running)
2. **Rebuilds** the plugin binary in container environment
3. **Copies** binary and config to plugins directory
4. **Refreshes** plugin discovery
5. **Enables** the plugin with new code

This ensures changes are immediately available without restarting the entire system.

## 🏗️ Build System Details

### Container-Based Building
- All plugins are built inside the Docker container for compatibility
- Eliminates host/container library mismatch issues
- Ensures plugins work correctly in production environment

### Automatic Dependency Management
- `go mod tidy` is run automatically
- Dependencies are resolved in container context
- No manual dependency management needed

### Smart Discovery
- Plugin discovery is refreshed after builds
- Plugins are automatically detected when deployed
- Status is updated in real-time

## 🧪 Testing Plugins

### Basic Testing
```bash
# Test plugin is working
./scripts/plugin-dev.sh test ffmpeg_software
```

### Manual API Testing
```bash
# List plugins
curl -s "http://localhost:8080/api/v1/plugins/" | jq '.data[] | select(.type == "transcoder")'

# Enable plugin
curl -X POST "http://localhost:8080/api/admin/plugins/ffmpeg_software/enable"

# Check plugin status
curl -s "http://localhost:8080/api/admin/plugins/ffmpeg_software"
```

### Transcoding Test
```bash
# Check available transcoding endpoints
curl -s "http://localhost:8080/api" | jq '.registered_routes[] | select(.path | contains("transcode"))'

# Start transcoding (requires media file)
curl -X POST "http://localhost:8080/api/playback/start" \
  -H "Content-Type: application/json" \
  -d '{
    "media_id": "your-media-id",
    "container": "dash",
    "quality": 70
  }'
```

## 🐛 Troubleshooting

### Plugin Won't Enable
```bash
# Check plugin binary exists
docker-compose exec backend ls -la /app/data/plugins/ffmpeg_software/

# Check plugin logs
docker-compose logs backend | grep -i "plugin\|ffmpeg"

# Rebuild plugin
./scripts/plugin-dev.sh reload ffmpeg_software
```

### Build Failures
```bash
# Check Go module status
cd plugins/ffmpeg_software && go mod tidy

# Manual container build
docker-compose exec backend sh -c "cd /tmp && go build your-plugin"

# Check container logs
docker-compose logs backend --tail=50
```

### Plugin Not Discovered
```bash
# Refresh plugin discovery
./scripts/plugin-dev.sh refresh

# Check plugin directory
docker-compose exec backend ls -la /app/data/plugins/

# Verify plugin.cue format
docker-compose exec backend cat /app/data/plugins/ffmpeg_software/plugin.cue
```

## 📋 Development Checklist

When developing a new plugin:

- [ ] Plugin compiles without errors
- [ ] Plugin implements all required interfaces
- [ ] Plugin.cue configuration is valid
- [ ] Plugin enables successfully via API
- [ ] Plugin appears in plugin list
- [ ] Plugin can be disabled/enabled repeatedly
- [ ] Hot reload works correctly
- [ ] Basic functionality test passes

## 🔗 Related Documentation

- [Main README](README.md) - Project overview
- [CLAUDE.md](CLAUDE.md) - Development instructions for Claude
- [Docker Compose](docker-compose.yml) - Container configuration
- [Plugin SDK](sdk/) - Plugin development SDK
- [Plugin Examples](plugins/) - Example plugin implementations

## 💡 Pro Tips

1. **Use hot reload frequently** - It's fast and safe
2. **Check plugin logs** - Use `docker-compose logs backend | grep plugin`
3. **Test in container** - Always verify plugins work in the Docker environment
4. **Use VS Code tasks** - Fastest way to develop interactively
5. **List plugins often** - Keep track of plugin status during development

## 🎯 Next Steps

After setting up the development environment:

1. Try building and enabling the software transcoder
2. Make a small change to the plugin code
3. Use hot reload to test the change
4. Explore the other hardware-specific plugins
5. Create your own custom plugin using the existing ones as templates

Happy plugin development! 🚀