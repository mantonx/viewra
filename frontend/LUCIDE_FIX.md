# Lucide React Fix

This document explains the fix for the Lucide React import issue in Docker environment.

## Problem

When running the frontend in Docker, the following error occurs:

```
[plugin:vite:import-analysis] Failed to resolve import "lucide-react" from "src/components/AudioPlayer.tsx". Does the file exist?
```

## Solution

We've implemented the following fixes:

1. Created a wrapper component for Lucide icons in `/src/components/ui/icons.tsx`
2. Added type declarations in `/src/lucide-react.d.ts`
3. Updated Docker configuration and startup script
4. Added a new `rebuild` command to `dev-compose.sh`

## How to Use

When you encounter the Lucide React import error after restarting Docker, use the rebuild command:

```bash
./dev-compose.sh rebuild
```

This will:

1. Stop all containers
2. Remove the frontend node_modules volume
3. Rebuild all images with --no-cache
4. Restart the services

## Icon Usage

Always import icons from our wrapper component instead of directly from lucide-react:

```tsx
// ❌ Don't do this
import { Play, Pause } from 'lucide-react';

// ✅ Do this instead
import { Play, Pause } from '@/components/ui/icons';
```

This ensures compatibility across all environments, including Docker.
