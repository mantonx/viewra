# Tech Debt Cleanup Plan

## Overview

Post-plugin restoration analysis reveals several tech debt areas that need attention for production readiness.

## High Priority Items

### 1. Complete Schema Migration ⚡

**Timeline: 1-2 days**

**Issues:**

- `MusicMetadata` model marked as TEMPORARY stub
- Multiple TODO items for Artist/Album/Track schema
- Legacy code compatibility issues

**Action Items:**

- [ ] Remove legacy `MusicMetadata` model
- [ ] Update all references to use Artist/Album/Track relationships
- [ ] Update plugin compatibility for new schema
- [ ] Test external plugins with new schema

**Files to Update:**

- `backend/internal/database/models.go:402-417`
- `backend/internal/server/handlers/music.go:28`
- `backend/internal/server/handlers/media.go:149`
- `backend/internal/server/handlers/admin.go:227`

### 2. Asset System Cleanup ⚡

**Timeline: 1 day**

**Issues:**

- Asset system uses placeholder album IDs
- Missing proper entity relationships
- Incomplete asset deletion

**Action Items:**

- [ ] Replace placeholder album ID generation with proper relationships
- [ ] Implement proper asset-to-entity linking
- [ ] Complete asset deletion functionality
- [ ] Update enrichment plugin to use proper album IDs

**Files to Update:**

- `backend/internal/plugins/media_manager.go:110`
- `backend/internal/plugins/manager.go:796`
- `backend/internal/modules/mediamodule/routes.go:348,369`
- `backend/internal/plugins/enrichment/core_plugin.go:238`

## Medium Priority Items

### 3. Configuration Management 📝

**Timeline: 2-3 days**

**Issues:**

- Hardcoded configurations scattered throughout
- No centralized config system
- Environment variable patterns missing

**Action Items:**

- [ ] Create centralized configuration system
- [ ] Add environment variable support for all settings
- [ ] Implement configuration validation
- [ ] Add configuration reload capability
- [ ] Document configuration options

**Current Config Systems:**

- Module config: `backend/internal/modules/modulemanager/config.go`
- Scanner config: `backend/internal/modules/scannermodule/scanner/config.go`
- Plugin configs: Individual `.cue` files

### 4. Error Handling Standardization 🚨

**Timeline: 1-2 days**

**Issues:**

- Mix of `log.Printf` and structured logging
- Inconsistent error context
- Debug logging in production

**Action Items:**

- [ ] Standardize on structured logging throughout
- [ ] Add proper error context and stack traces
- [ ] Implement log level configuration
- [ ] Remove debug logging from production paths
- [ ] Add error metrics and monitoring

### 5. Plugin System Hardening 🔌

**Timeline: 1-2 days**

**Issues:**

- Plugin directory detection fragile
- Missing dependency management
- Configuration validation could be improved

**Action Items:**

- [ ] Improve plugin directory detection logic
- [ ] Add plugin dependency validation
- [ ] Enhance plugin configuration schema validation
- [ ] Test all external plugins with new schema
- [ ] Add plugin health monitoring

## Low Priority Items

### 6. Code Quality Improvements 🧹

**Timeline: 2-3 days**

**Issues:**

- Some functions too large
- Magic numbers and hardcoded values
- Missing documentation

**Action Items:**

- [ ] Refactor large functions
- [ ] Replace magic numbers with constants
- [ ] Add comprehensive API documentation
- [ ] Improve code comments
- [ ] Add more unit tests

### 7. Performance Optimizations ⚡

**Timeline: 1-2 days**

**Issues:**

- Database query optimization opportunities
- Caching improvements
- Memory usage optimization

**Action Items:**

- [ ] Add database query indexes where needed
- [ ] Implement query result caching
- [ ] Optimize memory usage in scanner
- [ ] Add performance monitoring

## Implementation Strategy

### Phase 1: Critical Fixes (Week 1)

1. Complete schema migration
2. Fix asset system placeholders
3. Update plugin compatibility

### Phase 2: Infrastructure (Week 2)

1. Implement centralized configuration
2. Standardize error handling
3. Harden plugin system

### Phase 3: Polish (Week 3)

1. Code quality improvements
2. Performance optimizations
3. Documentation updates

## Testing Strategy

### For Each Phase:

- [ ] Unit tests for modified components
- [ ] Integration tests for plugin system
- [ ] End-to-end tests for scanner workflow
- [ ] Load testing for performance changes

### External Plugin Testing:

- [ ] Test TMDb enricher with new schema
- [ ] Test AudioDB enricher with new schema
- [ ] Test MusicBrainz enricher with new schema
- [ ] Verify asset downloading works correctly

## Success Metrics

### Technical:

- ✅ All TODO/TEMPORARY comments resolved
- ✅ Consistent logging throughout codebase
- ✅ Proper error context everywhere
- ✅ Centralized configuration system
- ✅ All tests passing

### Functional:

- ✅ All plugins work with new schema
- ✅ Asset management fully functional
- ✅ Scanner performance maintained/improved
- ✅ No regression in existing features

## Risk Mitigation

### Database Changes:

- Create migration scripts for schema changes
- Backup database before major changes
- Test migration on sample data first

### Plugin Compatibility:

- Test each plugin individually
- Maintain backward compatibility where possible
- Document breaking changes clearly

### Performance Impact:

- Benchmark before/after changes
- Monitor resource usage during testing
- Have rollback plan for performance regressions

## Notes

The core functionality is solid after the plugin restoration. The remaining tech debt is mainly about:

1. **Completing the schema migration** (highest impact)
2. **Improving maintainability** (configuration, logging, error handling)
3. **Polish and optimization** (performance, code quality)

Most issues are non-blocking for basic functionality but important for production stability and maintainability.
