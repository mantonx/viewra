# Transcoding Module Refactoring Analysis

## Overview
This document analyzes the transcoding module code to identify duplicate work, refactoring opportunities, and areas for improvement.

## Key Issues Identified

### 1. Duplicate Session Management

**Problem**: The module has THREE different session management implementations:
- `manager.go`: Has its own session tracking with `sessions` and `sessionHandles` maps
- `core/session/manager.go`: Implements a full session management system with state transitions
- `core/session/coordinator.go`: Yet another layer that wraps the session manager

**Impact**: 
- Triple the complexity for session management
- Race conditions due to multiple sources of truth
- Inconsistent state management

**Solution**: Use only the SessionStore for database-backed persistence and simplify the Manager to delegate all session operations to it.

### 2. Redundant Provider Management

**Problem**: 
- The Manager maintains its own provider maps (`providers`, `pipelineProvider`)
- Complex provider selection logic duplicated across files
- Plugin manager integration is convoluted

**Solution**: Simplify to a single provider registry with clear interfaces.

### 3. Overly Complex Storage System

**Problem**: The ContentStore has streaming-specific complexity that's no longer needed:
- Segment management methods (`AddSegment`, `GetSegments`)
- Streaming metadata (segment count, quality levels, streaming status)
- Complex directory organization for segments

**Solution**: Simplify to basic file storage since we're doing file-based transcoding.

### 4. Circular Dependencies and Poor Separation

**Problem**:
- Manager depends on too many internal components directly
- Session components have circular dependencies
- Storage wrapper pattern adds unnecessary indirection

**Solution**: Clear service boundaries with well-defined interfaces.

### 5. Inconsistent Error Handling

**Problem**: 
- Some methods log and continue, others return errors
- No consistent error wrapping strategy
- Silent failures in background goroutines

**Solution**: Establish clear error handling patterns.

### 6. Missing Documentation

**Problem**:
- Many complex methods lack documentation
- No clear explanation of component relationships
- Missing usage examples

**Solution**: Add comprehensive documentation to all public methods.

## Refactoring Plan

### Phase 1: Simplify Session Management
1. Remove `core/session/manager.go` and `core/session/coordinator.go`
2. Keep only `core/session/store.go` for database operations
3. Update Manager to use SessionStore directly
4. Remove duplicate session tracking from Manager

### Phase 2: Streamline Storage
1. Remove streaming-specific code from ContentStore
2. Simplify to basic file storage operations
3. Remove the wrapper pattern and use interfaces directly
4. Move hash generation to utils

### Phase 3: Clean Up Provider Management
1. Create a single provider registry
2. Remove duplicate provider tracking
3. Simplify provider selection logic
4. Better plugin integration

### Phase 4: Improve Code Organization
1. Move shared utilities to module utils or parent utils
2. Remove circular dependencies
3. Establish clear service boundaries
4. Add comprehensive documentation

### Phase 5: Error Handling and Logging
1. Establish consistent error handling patterns
2. Add context to all errors
3. Improve logging with structured fields
4. Handle background goroutine errors properly

## Code That Can Be Removed

1. **Entire files**:
   - `core/session/manager.go` - Duplicate session management
   - `core/session/coordinator.go` - Unnecessary abstraction
   - `core/session/state_validator.go` - Over-engineered state validation
   - `core/storage/wrapper.go` - Unnecessary wrapper pattern

2. **Methods/Features**:
   - Streaming-specific methods in ContentStore
   - Duplicate session tracking in Manager
   - Complex provider selection logic
   - Multiple logger instances

## Utilities to Extract

1. **To module utils**:
   - Session ID generation
   - Path manipulation helpers
   - Content hash generation

2. **To parent utils**:
   - Generic file operations
   - HTTP header utilities (if used elsewhere)
   - Generic hash functions

## Documentation Improvements Needed

1. **Package-level documentation** explaining the purpose and relationships
2. **Method documentation** with examples
3. **Architecture diagram** showing component relationships
4. **Usage examples** for common scenarios
5. **Migration guide** from old to new APIs