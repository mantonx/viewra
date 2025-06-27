# Hooks Organization

This directory contains all custom React hooks organized by their primary concern:

## Directory Structure

```
hooks/
├── session/          # Session and transcoding management  
├── media/            # Media loading and navigation
├── ui/               # UI interactions and state
└── index.ts          # Main export file
```

## Hook Categories

### Session (`/session`) 
Transcoding session management and seek-ahead functionality:
- `useSessionManager` - Session lifecycle, cleanup, validation
- `useSeekAhead` - Seek-ahead transcoding and manifest switching

### Media (`/media`)
Media loading, metadata, and navigation:
- `useMediaNavigation` - Media loading, routing, position management

### UI (`/ui`)
User interface interactions and visual state:
- `useKeyboardShortcuts` - Keyboard event handling
- `useControlsVisibility` - Auto-hide controls behavior
- `usePositionSaving` - Position persistence in localStorage
- `useFullscreenManager` - Fullscreen API management

## Usage

Import hooks from the main index or specific categories:

```typescript
// Import all hooks
import { useSessionManager, useKeyboardShortcuts } from '../hooks';

// Import from specific categories
import { useSessionManager, useSeekAhead } from '../hooks/session';
import { useKeyboardShortcuts, useControlsVisibility } from '../hooks/ui';
import { useMediaNavigation } from '../hooks/media';
```

## Guidelines

### Adding New Hooks
1. Determine the primary concern (session, media, ui)
2. Place in the appropriate directory
3. Update the category's index.ts export
4. Follow naming convention: `use[Feature][Action/Manager]`

### Hook Dependencies
- Hooks can depend on hooks from other categories
- Use relative imports for same-category dependencies
- Use category imports for cross-category dependencies

### State Management
- Use Jotai atoms for shared state
- Keep local state in hooks when appropriate
- Prefer derived state over duplicated state

### Error Handling
- Include proper error boundaries in hooks
- Log errors with appropriate context
- Graceful degradation for non-critical features