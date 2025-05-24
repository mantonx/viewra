# Viewra Tiltfile
# Modern development orchestration for media management system

print('🎬 Starting Viewra Development Environment')
print('=========================================')

# Load extensions
load('ext://restart_process', 'docker_build_with_restart')

# Configuration
config.define_string('env', args=True, usage='Environment (dev/prod)')
cfg = config.parse()
env = cfg.get('env', 'dev')

# Backend Service
print('🚀 Setting up Go Backend...')

docker_build_with_restart(
    'viewra-backend',
    context='./backend',
    dockerfile='./backend/Dockerfile',
    entrypoint=['go', 'run', 'cmd/viewra/main.go'],
    only=['./backend'],
    live_update=[
        sync('./backend', '/app'),
        run('go mod tidy', trigger=['./backend/go.mod', './backend/go.sum']),
    ]
)

# Frontend Service  
print('⚛️  Setting up React Frontend...')

docker_build_with_restart(
    'viewra-frontend', 
    context='./frontend',
    dockerfile='./frontend/Dockerfile',
    entrypoint=['npm', 'run', 'dev', '--', '--host', '0.0.0.0'],
    only=['./frontend'],
    live_update=[
        sync('./frontend/src', '/app/src'),
        sync('./frontend/public', '/app/public'),
        sync('./frontend/package.json', '/app/package.json'),
        run('npm install', trigger=['./frontend/package.json']),
    ]
)

# Use Docker Compose
docker_compose('./docker-compose.yml')

# Configure resources
dc_resource('backend', labels=['api'])
dc_resource('frontend', labels=['ui'])

# Development URLs
print('')
print('🌐 Development URLs:')
print('   Frontend: http://localhost:5173')
print('   Backend:  http://localhost:8080/api/health')
print('')
print('💡 Pro Tips:')
print('   - Edit files and see instant updates')
print('   - Check logs in Tilt UI')
print('   - Press "r" to manually rebuild a service')
print('')

# Watch for config changes
watch_file('./docker-compose.yml')
watch_file('./Tiltfile')

# Future expansion ready
if env == 'full':
    print('🔮 Future services can be enabled here:')
    # docker_compose('./docker-compose.full.yml')
    # dc_resource('postgres', labels=['database'])
    # dc_resource('redis', labels=['cache'])
    # dc_resource('media-scanner', labels=['services'])
