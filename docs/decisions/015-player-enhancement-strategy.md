# ADR 015: Player Enhancement Strategy

**Status**: Accepted
**Date**: 2025-01-20
**Deciders**: Development Team
**Related**: Phase 5.7 (Video Player), Phase 5.8 (Audio Player)

## Context

After comprehensive analysis of both video and audio players, we've identified that while both have solid technical foundations, they require significant enhancements to reach production quality comparable to industry standards. This ADR documents the findings and establishes the enhancement strategy.

## Analysis Summary

### Video Player Current State
- **Maturity**: ~40% complete vs industry standards
- **Critical Issues**: 3 memory leaks, aspect ratio distortion, no keyboard shortcuts
- **Performance**: 240-600 re-renders per minute, excessive timeupdate events
- **Visual Quality**: Missing `object-fit: contain`, poor contrast ratios, no responsive design
- **Missing Features**: Custom controls, Picture-in-Picture, playback speed, subtitles

### Audio Player Current State
- **Maturity**: Functional MVP with solid architecture
- **Critical Issues**: Context re-render cascade (240x/min), emoji icons, no queue UI, no playlists
- **Performance**: Unmemoized context value, inline style recalculation, excessive event handlers
- **Visual Quality**: Unprofessional emoji icons, missing album artwork, poor mobile UX
- **Missing Features**: Queue management UI, playlist system, expandable player, Media Session API

## Decision

We will implement a **three-tier enhancement strategy** for both players:

### Tier 1: Critical Fixes & MVP (Must Do First)
**Video Player** (44 hours):
1. Fix memory leak in progress tracking hook
2. Fix aspect ratio distortion (add `object-fit: contain`)
3. Throttle timeupdate events to 1/second
4. Add transcoding progress indicator
5. Implement keyboard shortcuts (Space, arrows, volume, etc.)
6. Add playback speed control
7. Improve error UI with retry functionality
8. Add buffering indicator
9. Implement Picture-in-Picture
10. Mobile touch optimizations

**Audio Player** (32 hours):
1. Memoize context value to prevent cascade re-renders
2. Remove currentTime from useEffect dependencies
3. Throttle time updates to 1/second
4. Replace emoji icons with SVG library (Lucide/Heroicons)
5. Build Queue Drawer UI with drag-to-reorder
6. Add album artwork to player (48x48px thumbnail)
7. Mobile responsive design (44x44px touch targets)
8. Implement Media Session API for OS controls

### Tier 2: Production Features (Recommended)
**Video Player** (42 hours):
- Custom control bar with auto-hide
- Timeline with seek preview
- Quality selector in controls (shows available qualities with "Original" badge) ✅
- Volume persistence
- Next episode auto-play
- Timeline thumbnails (backend + frontend)
- Skip intro/outro detection
- Subtitle support (VTT/SRT)

**Audio Player** (24 hours):
- Complete playlist system (backend + frontend)
  - Database schema: `playlists`, `playlist_tracks`
  - CRUD API endpoints
  - Frontend components: PlaylistList, PlaylistDetail, PlaylistForm
- Expandable full player with large album artwork
- Loading and error states
- Code refactoring (split into subcomponents)
- Keyboard shortcuts expansion
- State persistence (queue to localStorage)

### Tier 3: Advanced Features (Future)
**Video Player**:
- Advanced subtitle styling
- Multi-audio track selection
- Chromecast/AirPlay support
- Watch party synchronization
- HDR/Dolby support

**Audio Player**:
- Gapless playback (Web Audio API)
- Crossfade between tracks
- Lyrics display with sync
- Audio normalization (ReplayGain)
- Equalizer (BiquadFilterNode)

## Performance Optimizations

### Critical Performance Fixes
Both players require immediate performance attention:

1. **Context Memoization** (Audio Player)
   - Issue: Context value recreated 240x per minute
   - Fix: Wrap in `useMemo` with proper dependencies
   - Impact: 90% reduction in unnecessary consumer re-renders

2. **Event Throttling** (Both Players)
   - Issue: timeupdate fires 4-15 times per second
   - Fix: Only update state when second changes
   - Impact: 75-90% reduction in re-renders

3. **Memory Leak** (Video Player)
   - Issue: Progress updater creates multiple intervals
   - Fix: Use `useRef` instead of local variables
   - Impact: Prevents unbounded memory growth

4. **CSS Optimization** (Audio Player)
   - Issue: Inline gradient recalculated 240x per minute
   - Fix: Use CSS custom properties
   - Impact: 50% reduction in style recalculation

### Performance Targets
**Video Player**:
- Time to first frame: < 1 second (cached)
- Re-renders during playback: < 100/minute (vs current 240-600)
- Memory: Stable over 1-hour playback
- CPU usage: < 25% during playback

**Audio Player**:
- Time to first audio: < 100ms
- Re-renders during playback: < 60/minute (vs current 240)
- Memory: Stable over 3-hour session
- CPU usage: < 5% during playback

## Visual Quality Standards

### Accessibility (WCAG 2.1 AA)
- All controls keyboard navigable
- Focus indicators visible (focus:ring-2)
- Color contrast ratios meet 4.5:1 minimum
- ARIA labels comprehensive
- Screen reader tested (NVDA/VoiceOver)

### Mobile UX
- Touch targets minimum 44x44px
- Responsive breakpoints: sm:, md:, lg:
- Bottom-aligned controls for thumb reach
- Swipe gestures where appropriate
- Orientation change handling

### Professional Polish
- Replace all emoji icons with SVG
- Consistent theming and color palette
- Smooth 60fps animations
- Loading skeletons and error states
- Album artwork with blur effects

## Implementation Strategy

### Phase Sequencing
1. **Week 1-2**: Critical fixes and Tier 1 features
2. **Week 3-4**: Tier 2 production features
3. **Week 5+**: Tier 3 advanced features (as needed)

### Code Organization
**Video Player Structure**:
```
VideoPlayer/
├── VideoPlayer.tsx (main container)
├── VideoControls.tsx (custom control bar)
├── Timeline.tsx (seek bar with preview)
├── QualitySelector.tsx (quality dropdown)
├── VolumeControl.tsx (volume slider)
├── hooks/
│   ├── useKeyboardShortcuts.ts
│   └── useHLSPlayer.ts
└── utils/
    └── videoHelpers.ts
```

**Audio Player Structure**:
```
AudioPlayer/
├── AudioPlayer.tsx (main container)
├── AudioPlayerControls.tsx (playback buttons)
├── AudioPlayerSeekBar.tsx (progress bar)
├── AudioPlayerVolumeControl.tsx (volume)
├── AudioPlayerTrackInfo.tsx (metadata)
├── QueueDrawer.tsx (queue management)
├── ExpandedPlayer.tsx (full-screen view)
└── Playlists/
    ├── PlaylistList.tsx
    ├── PlaylistDetail.tsx
    └── PlaylistForm.tsx
```

## Dependencies

### Video Player
**Required**:
- `hls.js` (already installed)
- Icon library: `lucide-react` or `@heroicons/react`

**Optional**:
- `framer-motion` (smooth animations)

### Audio Player
**Required**:
- Icon library: `lucide-react` or `@heroicons/react`
- Drag and drop: `@dnd-kit/core` + `@dnd-kit/sortable`

**Optional**:
- `framer-motion` (smooth animations)

## Testing Requirements

### Functional Testing
- Playback scenarios (play, pause, seek, skip)
- Quality switching (video) - triggers FFmpeg restart at current position ✅
- Queue management (audio)
- Keyboard shortcuts
- Mobile gestures
- Error recovery

### Performance Testing
- Chrome DevTools Performance profiling
- Memory leak detection (heap snapshots)
- React DevTools Profiler (re-render analysis)
- Lighthouse audits
- Network throttling scenarios

### Accessibility Testing
- Screen reader testing (NVDA, VoiceOver)
- Keyboard-only navigation
- Color contrast validation
- Focus indicator visibility
- ARIA label verification

### Cross-Browser Testing
- Chrome (desktop + mobile)
- Firefox (desktop + mobile)
- Safari (desktop + iOS)
- Edge (desktop)

## Consequences

### Positive
- **User Experience**: Players match industry standards
- **Performance**: 75-90% reduction in re-renders, stable memory usage
- **Accessibility**: WCAG 2.1 AA compliant, keyboard navigable
- **Maintainability**: Modular component structure, reduced technical debt
- **Mobile**: Touch-optimized, responsive design
- **Professional**: No emoji icons, proper visual polish

### Negative
- **Development Time**: 76-154 hours total (Tier 1 + Tier 2)
- **Bundle Size**: Additional dependencies (~30-50KB gzipped)
- **Complexity**: More components to maintain
- **Testing Overhead**: Increased surface area for testing

### Risks
1. **Scope Creep**: Advanced features may expand beyond estimates
   - Mitigation: Strict tier boundaries, defer Tier 3 to future phases
2. **Browser Compatibility**: Custom controls may have edge cases
   - Mitigation: Fallback to native controls if needed
3. **Performance Regression**: New features may introduce overhead
   - Mitigation: Performance budgets, continuous profiling

## Alternatives Considered

### Alternative 1: Keep Browser Default Controls (Video)
**Rejected**: Limits customization, branding, and feature set. Cannot implement PiP button, quality selector in controls, or skip intro without custom UI.

### Alternative 2: Third-Party Player Libraries
**Rejected**:
- Video: Video.js, Plyr.js add 100-200KB, opinionated styling
- Audio: Howler.js, Amplitude.js unnecessary for our needs
- HTML5 Audio/Video APIs sufficient for our use cases

### Alternative 3: Defer Performance Fixes
**Rejected**: Memory leaks and excessive re-renders impact user experience now. Must fix before adding features to avoid compounding technical debt.

### Alternative 4: Build Playlists as Separate Feature (Audio)
**Accepted for Tier 2**: Playlists are essential but not blocking. Can be developed in parallel with player enhancements or sequentially after Tier 1.

## Success Metrics

### Video Player
- Time to first frame: < 1 second (cached), < 5 seconds (transcode)
- Keyboard shortcuts: 100% coverage of industry standard shortcuts
- Accessibility: WCAG 2.1 AA compliant (Lighthouse audit)
- Performance: < 100 re-renders per minute during playback
- Memory: < 500MB after 1 hour of playback
- User feedback: Positive comparison to industry standards

### Audio Player
- Time to first audio: < 100ms
- Queue management: Visible, reorderable, removable
- Playlists: Full CRUD operations working
- Accessibility: WCAG 2.1 AA compliant (Lighthouse audit)
- Performance: < 60 re-renders per minute during playback
- Memory: < 300MB after 3 hours of listening
- User feedback: Comparable to Spotify/Apple Music UX

## References

### Deep Dive Analysis Documents
- Video Player Analysis: Phase 5.7 planning (389 lines analyzed)
- Audio Player Analysis: Phase 5.8 planning (630 lines analyzed)
- Performance Profiling: Chrome DevTools findings
- Visual Quality Audit: WCAG compliance assessment

### Industry Standards

- Netflix player controls and keyboard shortcuts
- Spotify audio player design patterns
- Apple Music interface guidelines

### Technical Resources
- HLS.js documentation and best practices
- Web Audio API specifications (for advanced features)
- WCAG 2.1 guidelines
- React performance optimization patterns
- Chrome DevTools performance profiling

## Implementation Notes

### Critical Path Items
1. **Video Player Memory Leak** (30 min) - Must fix first, blocking
2. **Audio Player Context Memoization** (30 min) - Must fix first, blocking
3. **Video Aspect Ratio** (5 min) - Quick win, high visibility
4. **Audio Emoji Icons** (4 hours) - High visibility, professional appearance

### Quick Wins (< 2 hours each)
- Video: Add `object-fit: contain`, playback speed, volume persistence
- Audio: Album artwork display, memoize formatted times, CSS optimization
- Both: Throttle time updates, add loading indicators

### Complex Features (> 8 hours)
- Video: Custom control bar, timeline thumbnails, skip intro detection
- Audio: Queue drawer with drag-to-reorder, complete playlist system
- Both: Comprehensive accessibility audit and fixes

### Backend Requirements
- Timeline thumbnails: FFmpeg thumbnail generation at intervals
- Skip intro: Audio fingerprinting or ML-based detection
- Playlists: New database tables and API endpoints
- Subtitles: VTT/SRT parsing and storage

## Revision History

- 2025-01-20: Initial version documenting player enhancement strategy
- 2025-12-19: Updated to reflect single-quality playback model implementation

## Addendum: Single-Quality Playback Model (Implemented)

The video player now uses a **single-quality playback model** instead of traditional ABR:

### Key Changes from Original Plan

1. **Quality Selection**: Backend picks optimal quality based on client capabilities (screen, bandwidth, device type). Master playlist returns a **single variant**, not all variants.

2. **Quality Picker**: Shows all available qualities filtered by source resolution. User can override at any time, which triggers FFmpeg restart at current position.

3. **"Original" Badge**: Quality options that match source quality show an "Original" badge via `isOriginalQuality` flag.

4. **Preference Persistence**: Quality, audio track, and subtitle preferences are saved per-video in `watch_progress` table and restored on resume.

5. **No ABR**: Removed HLS.js ABR complexity since only one FFmpeg process runs at a time. `startLevel: 0` is set since only one variant exists.

### Removed Components

- `useAutoQuality.ts` - Auto mode toggle (not needed)
- `useNetworkMonitor.ts` - Continuous network monitoring (not needed)
- `NetworkMonitor.ts` - Bandwidth sampling (not needed)

### See Also

- [Single-Quality Playback Refactor](../guides/single-quality-playback-refactor.md)
- [Quality Selection Plan](../planning/quality-selection-plan.md)
