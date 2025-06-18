# PlaybackModule Documentation Index

> **Technical Documentation Hub for the Viewra PlaybackModule**

This directory contains detailed technical documentation for developers working with the PlaybackModule transcoding system.

## 📚 Documentation Structure

```
docs/
├── README.md           # This index file
├── api.md             # Complete API reference & examples
└── implementation.md  # Implementation details & architecture
```

## 📖 Available Documentation

### 🔌 [API Reference](api.md)
**Complete API documentation with examples and schemas**

- 🌐 REST API endpoints  
- 📋 Request/response schemas
- 🎯 Streaming endpoints (DASH/HLS)
- 💻 Code examples and curl commands
- 🚨 Error codes and troubleshooting

**Best for**: Frontend developers, API integration, testing

### 🏗️ [Implementation Guide](implementation.md)
**Deep dive into system architecture and implementation details**

- 🔧 Core architecture overview
- 📊 Session management internals
- 🔌 Plugin system design
- 🐳 Docker integration details
- ⚡ Performance considerations

**Best for**: Backend developers, system architecture, debugging

### 🧪 [E2E Testing Guide](../e2e/README.md)
**Comprehensive testing framework documentation**

- 🎯 Test categories and organization
- 🚀 Running and debugging tests
- 📊 Test results analysis
- 🔍 Critical issues discovered
- 🛠️ Test development guidelines

**Best for**: QA engineers, developers adding tests, CI/CD

## 🎯 Quick Navigation

### For New Developers
1. Start with the [main README](../README.md) for overview
2. Review [API documentation](api.md) for endpoint understanding  
3. Read [implementation guide](implementation.md) for architecture
4. Run [E2E tests](../e2e/README.md) to validate setup

### For API Integration
1. [API Reference](api.md) - Complete endpoint documentation
2. [E2E Testing](../e2e/README.md) - Example API usage patterns
3. [Main README](../README.md) - Quick start guide

### For System Debugging
1. [Implementation Guide](implementation.md) - System internals
2. [E2E Testing Results](../e2e/README.md) - Known issues and fixes
3. [API Reference](api.md) - Error codes and troubleshooting

### For Production Deployment
1. [Main README](../README.md) - Production checklist
2. [E2E Testing](../e2e/README.md) - Critical issues to address
3. [Implementation Guide](implementation.md) - Performance considerations

## 🚨 Critical Information

### Security & Validation Issues
Our E2E testing has identified **critical security issues** that must be addressed:

- ⚠️ **Request validation gaps** - System accepts invalid requests
- ⚠️ **Content-Type validation missing** - API robustness concerns
- ⚠️ **HTTP method handling incorrect** - Standards compliance issues

**See**: [E2E Critical Findings](../e2e/README.md#critical-findings)

### Plugin Integration Status
- ✅ **Mock environment**: 100% functional for development
- ❌ **Real plugin environment**: Requires FFmpeg plugin setup

**See**: [Implementation Guide](implementation.md) for plugin architecture details

## 📋 Documentation Maintenance

### Updating Documentation
- **API changes**: Update `api.md`
- **Architecture changes**: Update `implementation.md`
- **Test additions**: Update `../e2e/README.md`
- **General updates**: Update main `../README.md`

### Documentation Standards
- ✅ Include code examples
- ✅ Provide clear navigation
- ✅ Link to related sections
- ✅ Update status indicators (✅❌⚠️)
- ✅ Include troubleshooting info

## 🔗 External References

- **Main Project**: [Viewra Media Server Repository](../../../../)
- **Docker Setup**: [docker-compose.yml](../../../../docker-compose.yml)
- **Plugin Development**: See plugin SDK documentation
- **Frontend Integration**: See frontend proxy configuration

---

**Documentation Status**: 📚 **Comprehensive & Current**

All documentation is actively maintained and reflects the current state of the PlaybackModule system. 