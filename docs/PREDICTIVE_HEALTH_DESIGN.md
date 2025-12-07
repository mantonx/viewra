# Predictive Hardware Health Monitoring - Design Document

## Overview

Move from **reactive** fallback (wait for crash) to **predictive** fallback (detect degradation before failure).

## Current State

```
┌─────────────────────────────────────────────────────────────┐
│                    Current Flow                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  FFmpeg starts → ... → GPU fails → Error detected → Fallback│
│                         ↑                                   │
│                         │                                   │
│                    User sees glitch                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Proposed State

```
┌─────────────────────────────────────────────────────────────┐
│                    Predictive Flow                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  FFmpeg starts → Health monitor detects degradation →       │
│                  Preemptive fallback → Seamless transition  │
│                                                             │
│  No user-visible glitch!                                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Data Sources

### Level 1: FFmpeg Output (Zero Dependencies)

FFmpeg already outputs this to stderr:

```
frame=1000 fps=120.5 q=23.0 size=5000kB time=00:00:40.00 speed=4.8x
```

**Metrics we can extract:**

| Metric | Healthy | Warning | Critical |
|--------|---------|---------|----------|
| `speed` | > 2.0x | 1.0-2.0x | < 1.0x |
| `fps` | Stable | Dropping | Erratic |
| `q` (quantizer) | Stable | Rising | Spiking |
| Frame time variance | Low | Medium | High |

### Level 2: Segment Timing (Already Have Data)

Your `SegmentWatcher` tracks when segments appear:

```go
// Expected: 2s segment at 5x speed = 0.4s generation time
// Warning:  2s segment taking > 1s
// Critical: 2s segment taking > 2s (can't keep up)
```

### Level 3: GPU Metrics (Optional, NVIDIA-only initially)

```bash
nvidia-smi --query-gpu=utilization.encoder,memory.used,temperature.gpu --format=csv
# Output: 85 %, 4000 MiB, 72
```

## Implementation Plan

### Phase 1: FFmpeg Progress Parser Enhancement

Currently, your code parses stderr for `time=` to update the watchdog. Enhance to capture more metrics:

```go
// internal/infrastructure/transcoding/ffmpeg/progress_parser.go

type ProgressMetrics struct {
    Frame     int64         // Current frame number
    FPS       float64       // Frames per second
    Speed     float64       // Encoding speed (1.0 = realtime)
    Quantizer float64       // Quality metric (lower = better)
    Size      int64         // Output size so far
    Time      time.Duration // Output time position
    Timestamp time.Time     // When this was parsed
}

// Parse FFmpeg stderr line like:
// frame=1000 fps=120.5 q=23.0 size=5000kB time=00:00:40.00 speed=4.8x
func ParseProgressLine(line string) (*ProgressMetrics, error) {
    // Use regex or strings.Split to extract values
    // Return nil if line doesn't match progress format
}
```

### Phase 2: Health Score Calculator

```go
// internal/infrastructure/transcoding/ffmpeg/health_monitor.go

type HealthMonitor struct {
    samples    []ProgressMetrics // Rolling window of recent samples
    windowSize int               // e.g., 10 samples
    thresholds HealthThresholds
    logger     *slog.Logger

    // Callbacks
    onWarning  func(HealthStatus)
    onCritical func(HealthStatus)
}

type HealthThresholds struct {
    // Speed thresholds
    SpeedWarning  float64 // < 2.0x = warning
    SpeedCritical float64 // < 1.0x = critical

    // FPS stability (coefficient of variation)
    FPSVarianceWarning  float64 // > 0.2 = warning
    FPSVarianceCritical float64 // > 0.5 = critical

    // Quantizer spike detection
    QuantizerSpikeThreshold float64 // > 20% increase = stress
}

type HealthStatus struct {
    Score       float64   // 0.0 (critical) to 1.0 (healthy)
    State       HealthState // Healthy, Degraded, Critical
    Metrics     HealthMetrics
    Suggestion  HealthAction
    Timestamp   time.Time
}

type HealthState string
const (
    HealthStateHealthy  HealthState = "healthy"
    HealthStateDegraded HealthState = "degraded"
    HealthStateCritical HealthState = "critical"
)

type HealthAction string
const (
    ActionNone           HealthAction = "none"
    ActionReduceBitrate  HealthAction = "reduce_bitrate"
    ActionReduceRes      HealthAction = "reduce_resolution"
    ActionFallbackSW     HealthAction = "fallback_software"
)
```

### Phase 3: Integration with Session

```go
// Enhance TranscodeSession to use health monitoring

type TranscodeSession struct {
    // ... existing fields ...

    healthMonitor *HealthMonitor
    healthChan    chan HealthStatus // Receives health updates
}

func (s *TranscodeSession) Start(params StartParams) error {
    // ... existing setup ...

    // Create health monitor
    s.healthMonitor = NewHealthMonitor(HealthMonitorConfig{
        WindowSize: 10,
        Thresholds: DefaultThresholds(),
        OnWarning: func(status HealthStatus) {
            s.logger.Warn("Encoder health degraded",
                "score", status.Score,
                "speed", status.Metrics.AvgSpeed,
                "suggestion", status.Suggestion)
        },
        OnCritical: func(status HealthStatus) {
            s.logger.Error("Encoder health critical",
                "score", status.Score,
                "suggestion", status.Suggestion)
            // Trigger preemptive fallback
            s.requestFallback(status.Suggestion)
        },
    })

    // Start FFmpeg with enhanced stderr parsing
    go s.parseFFmpegOutput(stderr, s.healthMonitor)

    return nil
}

func (s *TranscodeSession) parseFFmpegOutput(stderr io.Reader, health *HealthMonitor) {
    scanner := bufio.NewScanner(stderr)
    for scanner.Scan() {
        line := scanner.Text()

        // Existing: log to file
        if s.logWriter != nil {
            s.logWriter.WriteString(line + "\n")
        }

        // NEW: Parse progress metrics
        if metrics, err := ParseProgressLine(line); err == nil {
            // Update watchdog (existing)
            s.watchdog.UpdateProgress()

            // NEW: Feed health monitor
            health.AddSample(*metrics)
        }
    }
}
```

### Phase 4: Health Score Calculation

```go
func (m *HealthMonitor) calculateHealthScore() HealthStatus {
    if len(m.samples) < 2 {
        return HealthStatus{Score: 1.0, State: HealthStateHealthy}
    }

    // Calculate metrics over window
    avgSpeed := m.avgSpeed()
    fpsVariance := m.fpsCoeffOfVariation()
    quantizerTrend := m.quantizerTrend()

    // Score each dimension (0.0 to 1.0)
    speedScore := m.scoreSpeed(avgSpeed)
    stabilityScore := m.scoreStability(fpsVariance)
    qualityScore := m.scoreQuality(quantizerTrend)

    // Weighted combination
    // Speed is most important (can't stream if too slow)
    score := speedScore*0.5 + stabilityScore*0.3 + qualityScore*0.2

    // Determine state and suggestion
    var state HealthState
    var suggestion HealthAction

    switch {
    case score < 0.3:
        state = HealthStateCritical
        suggestion = ActionFallbackSW
    case score < 0.6:
        state = HealthStateDegraded
        if avgSpeed < 1.5 {
            suggestion = ActionReduceRes
        } else {
            suggestion = ActionReduceBitrate
        }
    default:
        state = HealthStateHealthy
        suggestion = ActionNone
    }

    return HealthStatus{
        Score:      score,
        State:      state,
        Suggestion: suggestion,
        Metrics: HealthMetrics{
            AvgSpeed:     avgSpeed,
            FPSVariance:  fpsVariance,
            QuantizerTrend: quantizerTrend,
        },
        Timestamp: time.Now(),
    }
}

func (m *HealthMonitor) scoreSpeed(speed float64) float64 {
    // speed >= 3.0x = 1.0 (excellent)
    // speed == 1.0x = 0.3 (barely keeping up)
    // speed < 1.0x  = 0.0 (falling behind)
    switch {
    case speed >= 3.0:
        return 1.0
    case speed >= 2.0:
        return 0.8
    case speed >= 1.5:
        return 0.6
    case speed >= 1.0:
        return 0.3
    default:
        return 0.0
    }
}
```

### Phase 5: Preemptive Fallback

```go
func (s *TranscodeSession) requestFallback(suggestion HealthAction) {
    switch suggestion {
    case ActionFallbackSW:
        // Stop current FFmpeg, restart with software encoding
        s.logger.Info("Preemptive fallback to software encoding",
            "session_id", s.ID)

        // Signal to session manager
        s.fallbackRequested.Store(true)
        s.fallbackReason = "health_critical"

        // The session manager will handle restart with new params

    case ActionReduceRes:
        // Future: Dynamic resolution reduction
        // For now, just log and let client ABR handle it
        s.logger.Warn("Consider reducing resolution",
            "session_id", s.ID)

    case ActionReduceBitrate:
        // Future: Dynamic bitrate reduction
        s.logger.Warn("Consider reducing bitrate",
            "session_id", s.ID)
    }
}
```

## File Structure

```
internal/infrastructure/transcoding/
├── ffmpeg/
│   ├── health_monitor.go      # NEW: Health monitoring logic
│   ├── progress_parser.go     # NEW: FFmpeg output parser
│   └── fallback.go            # ENHANCE: Add preemptive fallback
└── session/
    ├── session.go             # ENHANCE: Integrate health monitor
    └── watchdog.go            # EXISTING: Keep for stall detection
```

## Optional: GPU Metrics (Phase 2)

For NVIDIA GPUs, add optional GPU-level monitoring:

```go
// internal/infrastructure/transcoding/gpu/nvidia.go

type NvidiaMonitor struct {
    available bool
    interval  time.Duration
    stopChan  chan struct{}
}

type GPUMetrics struct {
    EncoderUtil   int     // 0-100%
    MemoryUsed    int64   // Bytes
    MemoryTotal   int64   // Bytes
    Temperature   int     // Celsius
    EncoderSessions int   // Active NVENC sessions
}

func (m *NvidiaMonitor) Query() (*GPUMetrics, error) {
    // Option 1: Parse nvidia-smi output
    cmd := exec.Command("nvidia-smi",
        "--query-gpu=utilization.encoder,memory.used,memory.total,temperature.gpu",
        "--format=csv,noheader,nounits")

    // Option 2: Use NVML bindings (faster, no subprocess)
    // Requires: github.com/NVIDIA/go-nvml

    return parseOutput(cmd.Output())
}
```

## Benefits

1. **No user-visible glitches** - Fallback happens before failure
2. **Smarter quality decisions** - Reduce bitrate/resolution before full fallback
3. **Observability** - Health metrics for debugging and monitoring
4. **Zero new dependencies for Phase 1** - Just parse what FFmpeg already outputs

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| False positives (fallback when not needed) | Conservative thresholds, hysteresis |
| Overhead of parsing every line | Regex compiled once, ~1μs per line |
| GPU metrics polling overhead | Optional, 5-second intervals |

## Metrics to Expose

```go
// For Prometheus/observability
type HealthMetricsExporter struct{}

func (e *HealthMetricsExporter) Collect() []Metric {
    return []Metric{
        {Name: "viewra_transcode_health_score", Value: 0.85},
        {Name: "viewra_transcode_speed", Value: 4.2},
        {Name: "viewra_transcode_fps_variance", Value: 0.05},
        {Name: "viewra_fallback_count", Value: 2, Labels: map[string]string{"reason": "health_critical"}},
    }
}
```

## Implementation Order

1. **Week 1**: `progress_parser.go` - Parse speed, fps, q from FFmpeg output
2. **Week 1**: `health_monitor.go` - Calculate health score from samples
3. **Week 2**: Integrate with `TranscodeSession` - Feed parsed data to monitor
4. **Week 2**: Add preemptive fallback trigger to `HardwareFallbackManager`
5. **Week 3**: Add health metrics endpoint to API
6. **Future**: GPU-level metrics (NVIDIA-only initially)

## Example: Health Degradation Timeline

```
Time    Speed   FPS     Score   State      Action
0:00    5.2x    120.0   1.00    Healthy    -
0:10    4.8x    118.5   0.95    Healthy    -
0:20    3.1x    115.2   0.82    Healthy    -
0:30    2.2x    98.7    0.65    Degraded   Log warning
0:40    1.4x    72.3    0.45    Degraded   Suggest reduce resolution
0:50    0.9x    45.1    0.20    Critical   PREEMPTIVE FALLBACK TO SOFTWARE
                                           (before user sees any glitch!)
```

## Questions for Implementation

1. **Hysteresis**: How long should health stay degraded before acting?
   - Suggestion: 3 consecutive samples (6-10 seconds)

2. **Recovery**: If health improves, should we switch back to hardware?
   - Suggestion: Not automatically (too risky), but reset failure count

3. **Per-session vs global**: Should health be tracked per-session or globally?
   - Suggestion: Both - per-session for decisions, global for dashboard
