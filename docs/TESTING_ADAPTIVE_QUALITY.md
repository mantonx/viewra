# Testing Adaptive Quality Recommendations

This guide covers how to test the adaptive quality recommendation system.

## Quick Start

1. **Start the servers:**
   ```bash
   make dev  # Backend
   cd web && npm run dev  # Frontend
   ```

2. **Open browser to**: `http://localhost:5173` (or your dev URL)

3. **Play a video** and watch the console

## Manual Testing Checklist

### ✅ Basic Functionality

- [ ] **Quality detection happens on video load**
  - Open DevTools Console (F12)
  - Play any video
  - Look for: `Applied recommended quality: [height]p ([name])`
  - Verify quality selector shows ★ next to recommended quality

- [ ] **Hover tooltip shows recommendation details**
  - Hover over the quality selector
  - Verify tooltip appears with:
    - Recommended quality name (e.g., "Full HD")
    - Reasoning (e.g., "High-speed network and 1080p screen")
    - Data usage estimate (e.g., "~2250 MB/hr")

- [ ] **User can override recommendation**
  - Click quality selector
  - Choose a different quality
  - Verify video switches to selected quality
  - Verify ★ remains on recommended option

### ✅ Network Condition Testing

**Chrome DevTools: Network tab → Throttling**

| Network Condition | Expected Recommendation | How to Verify |
|------------------|------------------------|---------------|
| Fast 3G (750 Kbps) | 480p or 720p | Check console log reasoning |
| 4G (4 Mbps) | 720p or 1080p | Check ★ position in selector |
| WiFi (No throttle) | 1080p or 4K | Check applied quality level |

**Steps:**
1. Throttle network in DevTools
2. Refresh page or play new video
3. Check console for reasoning
4. Verify quality matches expectation

### ✅ Device/Screen Size Testing

**Chrome DevTools: Toggle device toolbar (Ctrl+Shift+M)**

| Device | Screen Size | Expected Quality |
|--------|-------------|------------------|
| iPhone SE | 375×667 | 480p or 720p |
| iPhone 12 Pro | 390×844 | 720p |
| iPad Air | 820×1180 | 1080p |
| Desktop | 1920×1080 | 1080p or 4K |
| 4K Display | 3840×2160 | 4K |

**Steps:**
1. Toggle device emulation
2. Select device
3. Play video
4. Verify recommended quality matches screen size

### ✅ Edge Cases

- [ ] **Slow network + high-res screen**
  - Throttle to Slow 3G
  - Desktop browser (1920×1080)
  - Expected: Lower quality despite large screen
  - Reasoning should mention network speed

- [ ] **Low battery mode** (if detectable)
  - Mobile device with <20% battery
  - Not charging
  - Expected: Lower quality to save power
  - Reasoning should mention battery

- [ ] **Multiple quality changes**
  - Change quality manually 3 times
  - Verify each change works
  - Verify ★ stays on original recommendation

## Console Debug Output

Expected console logs during video playback:

```javascript
// Capability detection (from hook)
{
  networkSpeedMbps: 50,
  deviceType: "desktop",
  screenWidth: 1920,
  screenHeight: 1080,
  // ...more capabilities
}

// Recommendation received
{
  height: 1080,
  displayName: "Full HD",
  videoBitrate: 5000000,
  reason: "High-speed network and 1080p screen",
  score: 95.5,
  // ...more details
}

// Quality applied (from VideoPlayer)
"Applied recommended quality: 1080p (Full HD)"
"Reason: High-speed network and 1080p screen"
```

## Automated Testing

### Unit Tests

```bash
cd web
npm test useQualityRecommendation
```

Tests verify:
- Hook initializes correctly
- Handles loading states
- Detects capabilities
- Fetches recommendations
- Handles errors gracefully
- Supports manual refresh

### Integration Tests (Future)

Currently pending. Will require:
- Mock backend API server
- Mock capability detection APIs
- Browser environment setup
- E2E test framework (Playwright/Cypress)

## Network Speed Test

To verify network speed detection:

```javascript
// In browser console
const caps = await window.capabilityDetector.detectCapabilities()
console.log('Network speed:', caps.networkSpeedMbps, 'Mbps')
```

Expected values:
- **WiFi**: 20-100 Mbps
- **4G**: 5-20 Mbps
- **3G**: 1-5 Mbps
- **Throttled**: Matches throttling setting

## Troubleshooting

### No recommendation shown
1. Check backend is running (`/api/adaptive/recommend` endpoint)
2. Check console for errors
3. Verify network request succeeded (Network tab)
4. Check `qualityRecommendation.isReady` is true

### Wrong quality recommended
1. Check console for reasoning
2. Verify capability detection values
3. Test with different network conditions
4. Check backend has 34 quality profiles loaded

### Star (★) not visible
1. Verify recommendation has `height` field
2. Check VideoControls received `recommendedQuality` prop
3. Inspect quality selector options in Elements tab

## Performance Metrics

Monitor these during testing:

- **Capability detection time**: Should be <2 seconds
- **API request time**: Should be <500ms
- **Total time to recommendation**: Should be <3 seconds
- **Video start delay**: Should add <3 seconds to playback start

## Success Criteria

✅ All manual tests pass
✅ Recommendations match network/device capabilities
✅ UI clearly indicates recommended quality
✅ User can override recommendations
✅ No console errors during normal operation
✅ Performance within acceptable ranges
