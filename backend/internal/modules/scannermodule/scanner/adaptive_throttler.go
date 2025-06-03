package scanner

import (
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/logger"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

// ContainerLimits represents container resource limits from cgroups
type ContainerLimits struct {
	MemoryLimitBytes   int64     `json:"memory_limit_bytes"`
	CPUQuota           int64     `json:"cpu_quota_us"`             // CPU quota in microseconds
	CPUPeriod          int64     `json:"cpu_period_us"`            // CPU period in microseconds
	CPUShares          int64     `json:"cpu_shares"`               // CPU shares (relative weight)
	MaxCPUPercent      float64   `json:"max_cpu_percent"`          // Calculated max CPU percentage
	BlkioThrottleRead  int64     `json:"blkio_throttle_read_bps"`  // Block I/O read throttle
	BlkioThrottleWrite int64     `json:"blkio_throttle_write_bps"` // Block I/O write throttle
	DetectedAt         time.Time `json:"detected_at"`
}

// ContainerMetrics represents current container resource usage from cgroups
type ContainerMetrics struct {
	MemoryUsageBytes int64     `json:"memory_usage_bytes"`
	MemoryPercent    float64   `json:"memory_percent"`
	CPUUsagePercent  float64   `json:"cpu_usage_percent"`
	BlkioReadBytes   int64     `json:"blkio_read_bytes"`
	BlkioWriteBytes  int64     `json:"blkio_write_bytes"`
	ThrottledTime    int64     `json:"throttled_time_ns"` // Time spent throttled
	LastUpdated      time.Time `json:"last_updated"`
}

// AdaptiveThrottler provides intelligent throttling for the scanner based on system conditions
type AdaptiveThrottler struct {
	mu sync.RWMutex

	// Configuration
	config ThrottleConfig

	// System metrics
	metrics SystemMetrics

	// Throttling state
	currentLimits    ThrottleLimits
	lastAdjustment   time.Time
	adjustmentHyster int // Prevents oscillation

	// Performance tracking
	performanceHistory []PerformanceSnapshot
	maxHistorySize     int

	// Network monitoring (for NFS/network storage)
	networkStats NetworkStats

	// Historical metrics for delta calculations
	lastNetworkStats    []psnet.IOCountersStat
	lastDiskStats       map[string]disk.IOCountersStat
	lastMetricsTime     time.Time
	metricsHistoryMutex sync.RWMutex

	// Container awareness
	isContainerized bool
	cgroupVersion   int // 1 or 2
	cgroupBasePath  string
	containerLimits ContainerLimits

	// Context for shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Events
	eventCallbacks []ThrottleEventCallback

	// Auto-adjustment
	autoAdjustEnabled atomic.Bool
	monitoringTicker  *time.Ticker

	// Emergency brake
	emergencyThrottled atomic.Bool
}

// ThrottleConfig holds configuration for the adaptive throttling system
type ThrottleConfig struct {
	// Worker limits
	MinWorkers     int `json:"min_workers" default:"1"`
	MaxWorkers     int `json:"max_workers" default:"16"`
	InitialWorkers int `json:"initial_workers" default:"4"`

	// Performance thresholds
	TargetCPUPercent    float64 `json:"target_cpu_percent" default:"70.0"`
	MaxCPUPercent       float64 `json:"max_cpu_percent" default:"85.0"`
	TargetMemoryPercent float64 `json:"target_memory_percent" default:"80.0"`
	MaxMemoryPercent    float64 `json:"max_memory_percent" default:"90.0"`

	// Network thresholds (MB/s)
	TargetNetworkThroughput float64 `json:"target_network_mbps" default:"80.0"` // 80 MB/s for 1Gbps
	MaxNetworkThroughput    float64 `json:"max_network_mbps" default:"100.0"`   // Leave headroom

	// I/O thresholds
	MaxIOWaitPercent    float64 `json:"max_io_wait_percent" default:"30.0"`
	TargetIOWaitPercent float64 `json:"target_io_wait_percent" default:"20.0"`

	// Batch processing limits
	MinBatchSize     int `json:"min_batch_size" default:"10"`
	MaxBatchSize     int `json:"max_batch_size" default:"200"`
	DefaultBatchSize int `json:"default_batch_size" default:"50"`

	// Processing delays
	MinProcessingDelay     time.Duration `json:"min_processing_delay" default:"0ms"`
	MaxProcessingDelay     time.Duration `json:"max_processing_delay" default:"500ms"`
	DefaultProcessingDelay time.Duration `json:"default_processing_delay" default:"10ms"`

	// Adjustment parameters
	AdjustmentInterval     time.Duration `json:"adjustment_interval" default:"5s"`
	SampleWindow           time.Duration `json:"sample_window" default:"30s"`
	MinAdjustmentThreshold float64       `json:"min_adjustment_threshold" default:"5.0"` // percent

	// Emergency brake settings
	EmergencyBrakeThreshold float64       `json:"emergency_brake_threshold" default:"95.0"`
	EmergencyBrakeDuration  time.Duration `json:"emergency_brake_duration" default:"10s"`

	// DNS/Network health
	DNSTimeoutMs          int    `json:"dns_timeout_ms" default:"1000"`
	NetworkHealthCheckURL string `json:"network_health_check_url" default:"8.8.8.8:53"`
}

// SystemMetrics represents current system performance metrics
type SystemMetrics struct {
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryPercent   float64   `json:"memory_percent"`
	MemoryUsedMB    float64   `json:"memory_used_mb"`
	IOWaitPercent   float64   `json:"io_wait_percent"`
	LoadAverage     float64   `json:"load_average"`
	NetworkUtilMBps float64   `json:"network_util_mbps"`
	DiskReadMBps    float64   `json:"disk_read_mbps"`
	DiskWriteMBps   float64   `json:"disk_write_mbps"`
	TimestampUTC    time.Time `json:"timestamp_utc"`
}

// ThrottleLimits represents current throttling limits
type ThrottleLimits struct {
	WorkerCount      int           `json:"worker_count"`
	BatchSize        int           `json:"batch_size"`
	ProcessingDelay  time.Duration `json:"processing_delay"`
	NetworkBandwidth float64       `json:"network_bandwidth_mbps"`
	IOThrottle       float64       `json:"io_throttle_percent"`
	Enabled          bool          `json:"enabled"`
}

// PerformanceSnapshot captures performance metrics at a point in time
type PerformanceSnapshot struct {
	Timestamp        time.Time      `json:"timestamp"`
	Metrics          SystemMetrics  `json:"metrics"`
	Limits           ThrottleLimits `json:"limits"`
	ThroughputMBps   float64        `json:"throughput_mbps"`
	FilesPerSecond   float64        `json:"files_per_second"`
	AdjustmentReason string         `json:"adjustment_reason"`
}

// NetworkStats tracks network-specific metrics for NFS/network storage
type NetworkStats struct {
	DNSLatencyMs      float64   `json:"dns_latency_ms"`
	NetworkLatencyMs  float64   `json:"network_latency_ms"`
	PacketLossPercent float64   `json:"packet_loss_percent"`
	ConnectionErrors  int64     `json:"connection_errors"`
	LastHealthCheck   time.Time `json:"last_health_check"`
	IsHealthy         bool      `json:"is_healthy"`
}

// ThrottleEventCallback defines the interface for throttling event notifications
type ThrottleEventCallback interface {
	OnThrottleAdjustment(reason string, oldLimits, newLimits ThrottleLimits, metrics SystemMetrics)
	OnEmergencyBrake(reason string, metrics SystemMetrics)
	OnEmergencyBrakeRelease(metrics SystemMetrics)
}

// NewAdaptiveThrottler creates a new adaptive throttling system
func NewAdaptiveThrottler(config ThrottleConfig) *AdaptiveThrottler {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults if not set
	if config.MinWorkers == 0 {
		config.MinWorkers = 1
	}
	if config.MaxWorkers == 0 {
		config.MaxWorkers = min(16, runtime.NumCPU()*2)
	}
	if config.InitialWorkers == 0 {
		config.InitialWorkers = min(config.MaxWorkers, max(config.MinWorkers, runtime.NumCPU()))
	}
	if config.TargetCPUPercent == 0 {
		config.TargetCPUPercent = 70.0
	}
	if config.MaxCPUPercent == 0 {
		config.MaxCPUPercent = 85.0
	}
	if config.TargetMemoryPercent == 0 {
		config.TargetMemoryPercent = 80.0
	}
	if config.MaxMemoryPercent == 0 {
		config.MaxMemoryPercent = 90.0
	}
	if config.TargetNetworkThroughput == 0 {
		config.TargetNetworkThroughput = 80.0 // 80 MB/s for 1Gbps with overhead
	}
	if config.MaxNetworkThroughput == 0 {
		config.MaxNetworkThroughput = 100.0
	}
	if config.DefaultBatchSize == 0 {
		config.DefaultBatchSize = 50
	}
	if config.MinBatchSize == 0 {
		config.MinBatchSize = 10
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 200
	}
	if config.AdjustmentInterval == 0 {
		config.AdjustmentInterval = 5 * time.Second
	}
	if config.DefaultProcessingDelay == 0 {
		config.DefaultProcessingDelay = 10 * time.Millisecond
	}
	if config.MaxProcessingDelay == 0 {
		config.MaxProcessingDelay = 500 * time.Millisecond
	}

	throttler := &AdaptiveThrottler{
		config:         config,
		ctx:            ctx,
		cancel:         cancel,
		maxHistorySize: 100,
		currentLimits: ThrottleLimits{
			WorkerCount:     config.InitialWorkers,
			BatchSize:       config.DefaultBatchSize,
			ProcessingDelay: config.DefaultProcessingDelay,
			Enabled:         true,
		},
		performanceHistory: make([]PerformanceSnapshot, 0, 100),
		eventCallbacks:     make([]ThrottleEventCallback, 0),
	}

	throttler.autoAdjustEnabled.Store(true)

	// Detect container environment
	throttler.detectContainerEnvironment()

	return throttler
}

// Start begins the adaptive throttling monitoring
func (at *AdaptiveThrottler) Start() error {
	logger.Info("Starting adaptive throttling system",
		"min_workers", at.config.MinWorkers,
		"max_workers", at.config.MaxWorkers,
		"target_cpu", at.config.TargetCPUPercent)

	// Start monitoring goroutine
	go at.monitoringLoop()

	// Start network health monitoring
	go at.networkHealthLoop()

	return nil
}

// Stop stops the adaptive throttling system
func (at *AdaptiveThrottler) Stop() {
	logger.Info("Stopping adaptive throttling system")

	at.cancel()

	if at.monitoringTicker != nil {
		at.monitoringTicker.Stop()
	}
}

// GetCurrentLimits returns the current throttling limits
func (at *AdaptiveThrottler) GetCurrentLimits() ThrottleLimits {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.currentLimits
}

// GetSystemMetrics returns the latest system metrics
func (at *AdaptiveThrottler) GetSystemMetrics() SystemMetrics {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.metrics
}

// ShouldThrottle returns whether processing should be throttled
func (at *AdaptiveThrottler) ShouldThrottle() (bool, time.Duration) {
	if at.emergencyThrottled.Load() {
		return true, at.config.EmergencyBrakeDuration
	}

	at.mu.RLock()
	defer at.mu.RUnlock()

	return at.currentLimits.ProcessingDelay > 0, at.currentLimits.ProcessingDelay
}

// ApplyDelay applies the current processing delay if throttling is enabled
func (at *AdaptiveThrottler) ApplyDelay() {
	shouldThrottle, delay := at.ShouldThrottle()
	if shouldThrottle && delay > 0 {
		time.Sleep(delay)
	}
}

// RegisterEventCallback registers a callback for throttling events
func (at *AdaptiveThrottler) RegisterEventCallback(callback ThrottleEventCallback) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.eventCallbacks = append(at.eventCallbacks, callback)
}

// EnableAutoAdjustment enables automatic throttling adjustments
func (at *AdaptiveThrottler) EnableAutoAdjustment() {
	at.autoAdjustEnabled.Store(true)
	logger.Info("Auto-adjustment enabled")
}

// DisableAutoAdjustment disables automatic throttling adjustments
func (at *AdaptiveThrottler) DisableAutoAdjustment() {
	at.autoAdjustEnabled.Store(false)
	logger.Info("Auto-adjustment disabled")
}

// monitoringLoop runs the main monitoring and adjustment loop
func (at *AdaptiveThrottler) monitoringLoop() {
	at.monitoringTicker = time.NewTicker(at.config.AdjustmentInterval)
	defer at.monitoringTicker.Stop()

	for {
		select {
		case <-at.ctx.Done():
			return
		case <-at.monitoringTicker.C:
			if at.autoAdjustEnabled.Load() {
				at.updateMetricsAndAdjust()
			}
		}
	}
}

// updateMetricsAndAdjust updates system metrics and adjusts throttling if needed
func (at *AdaptiveThrottler) updateMetricsAndAdjust() {
	// Update system metrics
	metrics := at.gatherSystemMetrics()

	at.mu.Lock()
	at.metrics = metrics
	at.mu.Unlock()

	// Check for emergency conditions
	if at.shouldTriggerEmergencyBrake(metrics) {
		at.triggerEmergencyBrake("High system load detected", metrics)
		return
	}

	// Release emergency brake if conditions improved
	if at.emergencyThrottled.Load() && at.shouldReleaseEmergencyBrake(metrics) {
		at.releaseEmergencyBrake(metrics)
	}

	// Skip adjustment if in emergency brake
	if at.emergencyThrottled.Load() {
		return
	}

	// Calculate optimal throttling limits
	newLimits := at.calculateOptimalLimits(metrics)

	// Apply adjustments if significant change is needed
	if at.shouldAdjustLimits(newLimits) {
		at.adjustLimits(newLimits, metrics)
	}

	// Record performance snapshot
	at.recordPerformanceSnapshot(metrics, newLimits)
}

// gatherSystemMetrics collects current system performance metrics using container-aware monitoring
func (at *AdaptiveThrottler) gatherSystemMetrics() SystemMetrics {
	ctx := context.Background()

	// If running in a container, prioritize container metrics
	if at.isContainerized {
		return at.gatherContainerAwareMetrics(ctx)
	}

	// Non-containerized environment - use gopsutil for host metrics
	return at.gatherHostMetrics(ctx)
}

// gatherContainerAwareMetrics collects metrics specific to container environment
func (at *AdaptiveThrottler) gatherContainerAwareMetrics(ctx context.Context) SystemMetrics {
	// Get container-specific metrics from cgroups
	containerMetrics, err := at.getContainerMetrics()
	if err != nil {
		logger.Debug("Failed to get container metrics, falling back to host metrics", "error", err)
		return at.gatherHostMetrics(ctx)
	}

	// Start with container memory metrics
	var memoryPercent, memoryUsedMB float64
	if at.containerLimits.MemoryLimitBytes > 0 {
		memoryPercent = containerMetrics.MemoryPercent
		memoryUsedMB = float64(containerMetrics.MemoryUsageBytes) / (1024 * 1024)
		logger.Debug("Using container memory metrics",
			"usage_percent", memoryPercent,
			"limit_gb", float64(at.containerLimits.MemoryLimitBytes)/(1024*1024*1024))
	} else {
		// No container memory limit, use host metrics
		memStats, err := mem.VirtualMemoryWithContext(ctx)
		if err == nil {
			memoryPercent = memStats.UsedPercent
			memoryUsedMB = float64(memStats.Used) / (1024 * 1024)
		}
	}

	// CPU metrics - try container-aware approach first
	var cpuPercent float64
	if at.containerLimits.MaxCPUPercent > 0 && at.containerLimits.MaxCPUPercent < float64(runtime.NumCPU()*100) {
		// Container has CPU limits, try to get container CPU usage
		cpuPercent = containerMetrics.CPUUsagePercent
		if cpuPercent == 0 {
			// Fallback to gopsutil but adjust for container limits
			cpuPercents, err := cpu.PercentWithContext(ctx, time.Second, false)
			if err == nil && len(cpuPercents) > 0 {
				hostCPU := cpuPercents[0]
				// Scale host CPU usage by container limit ratio
				limitRatio := at.containerLimits.MaxCPUPercent / (float64(runtime.NumCPU()) * 100)
				cpuPercent = hostCPU / limitRatio
				cpuPercent = math.Min(cpuPercent, 100.0) // Cap at 100%
			}
		}
		logger.Debug("Using container-aware CPU metrics",
			"cpu_percent", cpuPercent,
			"max_cpu_percent", at.containerLimits.MaxCPUPercent)
	} else {
		// No CPU limits or unlimited, use host CPU
		cpuPercents, err := cpu.PercentWithContext(ctx, time.Second, false)
		if err == nil && len(cpuPercents) > 0 {
			cpuPercent = cpuPercents[0]
		}
	}

	// Load average - use host metrics as container doesn't typically limit this
	loadStats, err := load.AvgWithContext(ctx)
	var loadAverage float64
	if err == nil {
		loadAverage = loadStats.Load1
	} else {
		loadAverage = float64(runtime.NumGoroutine()) / float64(runtime.NumCPU())
	}

	// I/O wait - use host metrics but consider container I/O throttling
	var ioWaitPercent float64
	cpuTimes, err := cpu.TimesWithContext(ctx, false)
	if err == nil && len(cpuTimes) > 0 {
		total := cpuTimes[0].User + cpuTimes[0].System + cpuTimes[0].Idle + cpuTimes[0].Iowait
		if total > 0 {
			ioWaitPercent = (cpuTimes[0].Iowait / total) * 100
		}
	} else {
		ioWaitPercent = at.estimateIOWaitFallback()
	}

	// Check if container has I/O throttling enabled
	if at.containerLimits.BlkioThrottleRead > 0 || at.containerLimits.BlkioThrottleWrite > 0 {
		// Container has I/O throttling, be more conservative
		ioWaitPercent = math.Max(ioWaitPercent, 10.0) // Assume at least 10% I/O wait with throttling
		logger.Debug("Container I/O throttling detected",
			"read_throttle_bps", at.containerLimits.BlkioThrottleRead,
			"write_throttle_bps", at.containerLimits.BlkioThrottleWrite)
	}

	// Network and disk I/O - use host methods but consider container limits
	networkUtilMBps, diskReadMBps, diskWriteMBps := at.getNetworkAndDiskMetrics(ctx)

	// Apply container I/O throttling if configured
	if at.containerLimits.BlkioThrottleRead > 0 {
		maxDiskReadMBps := float64(at.containerLimits.BlkioThrottleRead) / (1024 * 1024)
		diskReadMBps = math.Min(diskReadMBps, maxDiskReadMBps)
	}
	if at.containerLimits.BlkioThrottleWrite > 0 {
		maxDiskWriteMBps := float64(at.containerLimits.BlkioThrottleWrite) / (1024 * 1024)
		diskWriteMBps = math.Min(diskWriteMBps, maxDiskWriteMBps)
	}

	return SystemMetrics{
		CPUPercent:      cpuPercent,
		MemoryPercent:   memoryPercent,
		MemoryUsedMB:    memoryUsedMB,
		IOWaitPercent:   ioWaitPercent,
		LoadAverage:     loadAverage,
		NetworkUtilMBps: networkUtilMBps,
		DiskReadMBps:    diskReadMBps,
		DiskWriteMBps:   diskWriteMBps,
		TimestampUTC:    time.Now().UTC(),
	}
}

// gatherHostMetrics collects metrics for non-containerized environments
func (at *AdaptiveThrottler) gatherHostMetrics(ctx context.Context) SystemMetrics {
	// CPU metrics
	cpuPercents, err := cpu.PercentWithContext(ctx, time.Second, false)
	var cpuPercent float64
	if err != nil {
		logger.Debug("Failed to get CPU metrics", "error", err)
		// Fallback to goroutine-based estimation
		numGoroutines := runtime.NumGoroutine()
		maxGoroutines := float64(runtime.NumCPU() * 50)
		cpuPercent = math.Min(100.0, float64(numGoroutines)/maxGoroutines*100.0)
	} else if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	// Memory metrics
	memStats, err := mem.VirtualMemoryWithContext(ctx)
	var memoryPercent, memoryUsedMB float64
	if err != nil {
		logger.Debug("Failed to get memory metrics", "error", err)
		// Fallback to runtime memory stats
		var runtimeMemStats runtime.MemStats
		runtime.ReadMemStats(&runtimeMemStats)
		memoryPercent = float64(runtimeMemStats.Alloc) / float64(runtimeMemStats.Sys) * 100
		memoryUsedMB = float64(runtimeMemStats.Alloc) / (1024 * 1024)
	} else {
		memoryPercent = memStats.UsedPercent
		memoryUsedMB = float64(memStats.Used) / (1024 * 1024)
	}

	// Load average
	loadStats, err := load.AvgWithContext(ctx)
	var loadAverage float64
	if err != nil {
		logger.Debug("Failed to get load average", "error", err)
		// Fallback to goroutine-based estimation
		loadAverage = float64(runtime.NumGoroutine()) / float64(runtime.NumCPU())
	} else {
		loadAverage = loadStats.Load1
	}

	// I/O Wait - try to get from CPU stats
	var ioWaitPercent float64
	cpuTimes, err := cpu.TimesWithContext(ctx, false)
	if err != nil || len(cpuTimes) == 0 {
		logger.Debug("Failed to get CPU times for I/O wait", "error", err)
		// Fallback to estimation
		ioWaitPercent = at.estimateIOWaitFallback()
	} else {
		// Calculate I/O wait percentage from CPU times
		total := cpuTimes[0].User + cpuTimes[0].System + cpuTimes[0].Idle + cpuTimes[0].Iowait
		if total > 0 {
			ioWaitPercent = (cpuTimes[0].Iowait / total) * 100
		}
	}

	// Network and disk I/O metrics
	networkUtilMBps, diskReadMBps, diskWriteMBps := at.getNetworkAndDiskMetrics(ctx)

	return SystemMetrics{
		CPUPercent:      cpuPercent,
		MemoryPercent:   memoryPercent,
		MemoryUsedMB:    memoryUsedMB,
		IOWaitPercent:   ioWaitPercent,
		LoadAverage:     loadAverage,
		NetworkUtilMBps: networkUtilMBps,
		DiskReadMBps:    diskReadMBps,
		DiskWriteMBps:   diskWriteMBps,
		TimestampUTC:    time.Now().UTC(),
	}
}

// estimateIOWaitFallback provides a fallback I/O wait estimation when gopsutil fails
func (at *AdaptiveThrottler) estimateIOWaitFallback() float64 {
	// For NFS/network storage, I/O wait can be significant due to network latency
	// This is a very basic estimate based on goroutine count as a proxy for system load
	numGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()

	// Rough estimate: more goroutines relative to CPU cores suggests potential I/O blocking
	loadRatio := float64(numGoroutines) / float64(numCPU)
	estimatedIOWait := math.Min(loadRatio*5.0, 40.0) // Cap at 40%

	return estimatedIOWait
}

// shouldTriggerEmergencyBrake determines if emergency throttling should be activated
func (at *AdaptiveThrottler) shouldTriggerEmergencyBrake(metrics SystemMetrics) bool {
	if at.emergencyThrottled.Load() {
		return false // Already activated
	}

	// Check multiple criteria for emergency brake
	emergencyConditions := []bool{
		metrics.CPUPercent > at.config.EmergencyBrakeThreshold,
		metrics.MemoryPercent > at.config.EmergencyBrakeThreshold,
		metrics.IOWaitPercent > at.config.MaxIOWaitPercent*1.5, // 1.5x the normal max
		metrics.LoadAverage > float64(runtime.NumCPU())*4.0,    // 4x CPU count (increased from 2x)
	}

	// Trigger if any emergency condition is met
	for _, condition := range emergencyConditions {
		if condition {
			return true
		}
	}

	return false
}

// shouldReleaseEmergencyBrake determines if emergency throttling should be released
func (at *AdaptiveThrottler) shouldReleaseEmergencyBrake(metrics SystemMetrics) bool {
	if !at.emergencyThrottled.Load() {
		return false
	}

	// Require all metrics to be below emergency thresholds with some hysteresis
	safeThreshold := at.config.EmergencyBrakeThreshold * 0.8 // 20% below trigger

	return metrics.CPUPercent < safeThreshold &&
		metrics.MemoryPercent < safeThreshold &&
		metrics.IOWaitPercent < at.config.MaxIOWaitPercent &&
		metrics.LoadAverage < float64(runtime.NumCPU())*2.0 // Release when load < 2x CPU count
}

// triggerEmergencyBrake activates emergency throttling
func (at *AdaptiveThrottler) triggerEmergencyBrake(reason string, metrics SystemMetrics) {
	if at.emergencyThrottled.Load() {
		return // Already activated
	}

	logger.Warn("Emergency brake triggered", "reason", reason,
		"cpu_percent", metrics.CPUPercent,
		"memory_percent", metrics.MemoryPercent,
		"io_wait_percent", metrics.IOWaitPercent)

	at.emergencyThrottled.Store(true)

	// Drastically reduce limits
	at.mu.Lock()
	oldLimits := at.currentLimits
	at.currentLimits = ThrottleLimits{
		WorkerCount:     1, // Minimal workers
		BatchSize:       at.config.MinBatchSize,
		ProcessingDelay: at.config.EmergencyBrakeDuration,
		Enabled:         true,
	}
	newLimits := at.currentLimits
	at.mu.Unlock()

	// Notify callbacks
	for _, callback := range at.eventCallbacks {
		go callback.OnEmergencyBrake(reason, metrics)
		go callback.OnThrottleAdjustment("emergency_brake", oldLimits, newLimits, metrics)
	}
}

// releaseEmergencyBrake deactivates emergency throttling
func (at *AdaptiveThrottler) releaseEmergencyBrake(metrics SystemMetrics) {
	if !at.emergencyThrottled.Load() {
		return
	}

	logger.Info("Emergency brake released",
		"cpu_percent", metrics.CPUPercent,
		"memory_percent", metrics.MemoryPercent)

	at.emergencyThrottled.Store(false)

	// Restore reasonable limits
	at.mu.Lock()
	at.currentLimits = ThrottleLimits{
		WorkerCount:     at.config.MinWorkers,
		BatchSize:       at.config.DefaultBatchSize,
		ProcessingDelay: at.config.DefaultProcessingDelay,
		Enabled:         true,
	}
	at.mu.Unlock()

	// Notify callbacks
	for _, callback := range at.eventCallbacks {
		go callback.OnEmergencyBrakeRelease(metrics)
	}
}

// calculateOptimalLimits determines optimal throttling limits based on current metrics
func (at *AdaptiveThrottler) calculateOptimalLimits(metrics SystemMetrics) ThrottleLimits {
	limits := ThrottleLimits{Enabled: true}

	// Calculate optimal worker count based on resource utilization
	limits.WorkerCount = at.calculateOptimalWorkerCount(metrics)

	// Calculate optimal batch size
	limits.BatchSize = at.calculateOptimalBatchSize(metrics)

	// Calculate processing delay
	limits.ProcessingDelay = at.calculateOptimalDelay(metrics)

	// Calculate network bandwidth limit
	limits.NetworkBandwidth = at.calculateOptimalNetworkBandwidth(metrics)

	// Calculate I/O throttle
	limits.IOThrottle = at.calculateOptimalIOThrottle(metrics)

	return limits
}

// calculateOptimalWorkerCount determines the optimal number of workers
func (at *AdaptiveThrottler) calculateOptimalWorkerCount(metrics SystemMetrics) int {
	currentWorkers := at.currentLimits.WorkerCount

	// Start with current count
	optimalWorkers := currentWorkers

	// CPU-based adjustment
	if metrics.CPUPercent < at.config.TargetCPUPercent*0.7 && metrics.MemoryPercent < at.config.TargetMemoryPercent*0.7 {
		// System is underutilized, can increase workers
		optimalWorkers = min(at.config.MaxWorkers, currentWorkers+1)
	} else if metrics.CPUPercent > at.config.MaxCPUPercent || metrics.MemoryPercent > at.config.MaxMemoryPercent {
		// System is overloaded, reduce workers
		optimalWorkers = max(at.config.MinWorkers, currentWorkers-1)
	} else if metrics.IOWaitPercent > at.config.MaxIOWaitPercent {
		// High I/O wait, reduce workers to prevent I/O bottleneck
		optimalWorkers = max(at.config.MinWorkers, currentWorkers-1)
	}

	// Network consideration for NFS/network storage
	if metrics.NetworkUtilMBps > at.config.TargetNetworkThroughput {
		optimalWorkers = max(at.config.MinWorkers, optimalWorkers-1)
	}

	return optimalWorkers
}

// calculateOptimalBatchSize determines the optimal batch size
func (at *AdaptiveThrottler) calculateOptimalBatchSize(metrics SystemMetrics) int {
	currentBatch := at.currentLimits.BatchSize

	// Adjust batch size based on memory and I/O conditions
	if metrics.MemoryPercent > at.config.TargetMemoryPercent {
		// High memory usage, reduce batch size
		return max(at.config.MinBatchSize, int(float64(currentBatch)*0.8))
	} else if metrics.IOWaitPercent > at.config.TargetIOWaitPercent {
		// High I/O wait, reduce batch size to improve responsiveness
		return max(at.config.MinBatchSize, int(float64(currentBatch)*0.7))
	} else if metrics.CPUPercent < at.config.TargetCPUPercent*0.6 && metrics.MemoryPercent < at.config.TargetMemoryPercent*0.6 {
		// System underutilized, can increase batch size
		return min(at.config.MaxBatchSize, int(float64(currentBatch)*1.2))
	}

	return currentBatch
}

// calculateOptimalDelay determines the optimal processing delay
func (at *AdaptiveThrottler) calculateOptimalDelay(metrics SystemMetrics) time.Duration {
	// Base delay calculation
	baseDelay := at.config.DefaultProcessingDelay

	// Increase delay based on system stress
	stressFactor := 1.0

	if metrics.CPUPercent > at.config.TargetCPUPercent {
		stressFactor *= 1.0 + (metrics.CPUPercent-at.config.TargetCPUPercent)/100.0
	}

	if metrics.MemoryPercent > at.config.TargetMemoryPercent {
		stressFactor *= 1.0 + (metrics.MemoryPercent-at.config.TargetMemoryPercent)/100.0
	}

	if metrics.IOWaitPercent > at.config.TargetIOWaitPercent {
		stressFactor *= 1.0 + (metrics.IOWaitPercent-at.config.TargetIOWaitPercent)/50.0
	}

	// Network storage specific delay for NFS
	if metrics.NetworkUtilMBps > at.config.TargetNetworkThroughput {
		stressFactor *= 1.0 + (metrics.NetworkUtilMBps-at.config.TargetNetworkThroughput)/at.config.TargetNetworkThroughput
	}

	// Apply stress factor
	adjustedDelay := time.Duration(float64(baseDelay) * stressFactor)

	// Clamp to configured limits
	if adjustedDelay < at.config.MinProcessingDelay {
		adjustedDelay = at.config.MinProcessingDelay
	} else if adjustedDelay > at.config.MaxProcessingDelay {
		adjustedDelay = at.config.MaxProcessingDelay
	}

	return adjustedDelay
}

// calculateOptimalNetworkBandwidth determines optimal network bandwidth limit
func (at *AdaptiveThrottler) calculateOptimalNetworkBandwidth(metrics SystemMetrics) float64 {
	if metrics.NetworkUtilMBps > at.config.MaxNetworkThroughput {
		return at.config.TargetNetworkThroughput * 0.8 // Reduce to 80% of target
	} else if metrics.NetworkUtilMBps < at.config.TargetNetworkThroughput*0.5 {
		return at.config.MaxNetworkThroughput // Allow full utilization
	}

	return at.config.TargetNetworkThroughput
}

// calculateOptimalIOThrottle determines optimal I/O throttling percentage
func (at *AdaptiveThrottler) calculateOptimalIOThrottle(metrics SystemMetrics) float64 {
	if metrics.IOWaitPercent > at.config.MaxIOWaitPercent {
		return 50.0 // Throttle I/O to 50%
	} else if metrics.IOWaitPercent > at.config.TargetIOWaitPercent {
		return 80.0 // Moderate throttling
	}

	return 100.0 // No throttling
}

// shouldAdjustLimits determines if throttling limits should be adjusted
func (at *AdaptiveThrottler) shouldAdjustLimits(newLimits ThrottleLimits) bool {
	at.mu.RLock()
	current := at.currentLimits
	at.mu.RUnlock()

	// Check if enough time has passed since last adjustment
	if time.Since(at.lastAdjustment) < at.config.AdjustmentInterval {
		return false
	}

	// Check for significant changes
	workerChange := abs(newLimits.WorkerCount - current.WorkerCount)
	batchChange := abs(newLimits.BatchSize - current.BatchSize)

	// Use time.Duration directly for delay comparison
	delayDiff := newLimits.ProcessingDelay - current.ProcessingDelay
	if delayDiff < 0 {
		delayDiff = -delayDiff
	}
	significantDelayChange := delayDiff > at.config.DefaultProcessingDelay

	return workerChange > 0 || batchChange > at.config.MinBatchSize/5 || significantDelayChange
}

// adjustLimits applies new throttling limits
func (at *AdaptiveThrottler) adjustLimits(newLimits ThrottleLimits, metrics SystemMetrics) {
	at.mu.Lock()
	oldLimits := at.currentLimits
	at.currentLimits = newLimits
	at.lastAdjustment = time.Now()
	at.adjustmentHyster++
	at.mu.Unlock()

	reason := at.determineAdjustmentReason(oldLimits, newLimits, metrics)

	logger.Info("Throttling adjusted",
		"reason", reason,
		"workers", fmt.Sprintf("%d→%d", oldLimits.WorkerCount, newLimits.WorkerCount),
		"batch_size", fmt.Sprintf("%d→%d", oldLimits.BatchSize, newLimits.BatchSize),
		"delay", fmt.Sprintf("%v→%v", oldLimits.ProcessingDelay, newLimits.ProcessingDelay),
		"cpu_percent", metrics.CPUPercent,
		"memory_percent", metrics.MemoryPercent)

	// Notify callbacks
	for _, callback := range at.eventCallbacks {
		go callback.OnThrottleAdjustment(reason, oldLimits, newLimits, metrics)
	}
}

// determineAdjustmentReason analyzes the adjustment and provides a reason
func (at *AdaptiveThrottler) determineAdjustmentReason(old, new ThrottleLimits, metrics SystemMetrics) string {
	if new.WorkerCount > old.WorkerCount {
		return "scaling_up_workers"
	} else if new.WorkerCount < old.WorkerCount {
		if metrics.CPUPercent > at.config.MaxCPUPercent {
			return "reducing_workers_cpu"
		} else if metrics.MemoryPercent > at.config.MaxMemoryPercent {
			return "reducing_workers_memory"
		} else if metrics.IOWaitPercent > at.config.MaxIOWaitPercent {
			return "reducing_workers_io"
		} else if metrics.NetworkUtilMBps > at.config.TargetNetworkThroughput {
			return "reducing_workers_network"
		}
		return "reducing_workers_load"
	} else if new.ProcessingDelay > old.ProcessingDelay {
		return "increasing_delay"
	} else if new.ProcessingDelay < old.ProcessingDelay {
		return "reducing_delay"
	} else if new.BatchSize != old.BatchSize {
		return "adjusting_batch_size"
	}

	return "minor_adjustment"
}

// recordPerformanceSnapshot records a performance snapshot for analysis
func (at *AdaptiveThrottler) recordPerformanceSnapshot(metrics SystemMetrics, limits ThrottleLimits) {
	snapshot := PerformanceSnapshot{
		Timestamp:        time.Now(),
		Metrics:          metrics,
		Limits:           limits,
		AdjustmentReason: "monitoring",
	}

	at.mu.Lock()
	at.performanceHistory = append(at.performanceHistory, snapshot)

	// Keep only recent history
	if len(at.performanceHistory) > at.maxHistorySize {
		at.performanceHistory = at.performanceHistory[1:]
	}
	at.mu.Unlock()
}

// networkHealthLoop monitors network health for NFS/network storage
func (at *AdaptiveThrottler) networkHealthLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-at.ctx.Done():
			return
		case <-ticker.C:
			at.checkNetworkHealth()
		}
	}
}

// checkNetworkHealth performs network health checks
func (at *AdaptiveThrottler) checkNetworkHealth() {
	start := time.Now()

	// DNS health check
	dnsLatency := at.checkDNSHealth()

	// Network connectivity check
	networkLatency := at.checkNetworkConnectivity()

	at.mu.Lock()
	at.networkStats = NetworkStats{
		DNSLatencyMs:     dnsLatency,
		NetworkLatencyMs: networkLatency,
		LastHealthCheck:  start,
		IsHealthy:        dnsLatency < float64(at.config.DNSTimeoutMs) && networkLatency < 100.0,
	}
	at.mu.Unlock()
}

// checkDNSHealth checks DNS resolution performance
func (at *AdaptiveThrottler) checkDNSHealth() float64 {
	start := time.Now()

	_, err := net.LookupHost("google.com")
	if err != nil {
		logger.Warn("DNS health check failed", "error", err)
		return float64(at.config.DNSTimeoutMs) // Return timeout value on failure
	}

	return float64(time.Since(start).Milliseconds())
}

// checkNetworkConnectivity checks basic network connectivity
func (at *AdaptiveThrottler) checkNetworkConnectivity() float64 {
	start := time.Now()

	conn, err := net.DialTimeout("tcp", at.config.NetworkHealthCheckURL,
		time.Duration(at.config.DNSTimeoutMs)*time.Millisecond)
	if err != nil {
		logger.Warn("Network connectivity check failed", "error", err)
		return 1000.0 // Return high latency on failure
	}
	defer conn.Close()

	return float64(time.Since(start).Milliseconds())
}

// GetPerformanceHistory returns recent performance history
func (at *AdaptiveThrottler) GetPerformanceHistory() []PerformanceSnapshot {
	at.mu.RLock()
	defer at.mu.RUnlock()

	history := make([]PerformanceSnapshot, len(at.performanceHistory))
	copy(history, at.performanceHistory)
	return history
}

// GetNetworkStats returns current network statistics
func (at *AdaptiveThrottler) GetNetworkStats() NetworkStats {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.networkStats
}

// SetThrottleConfig updates the throttling configuration
func (at *AdaptiveThrottler) SetThrottleConfig(config ThrottleConfig) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.config = config
}

// GetThrottleConfig returns the current throttling configuration
func (at *AdaptiveThrottler) GetThrottleConfig() ThrottleConfig {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return at.config
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// getNetworkAndDiskMetrics collects network and disk I/O metrics using gopsutil with delta tracking
func (at *AdaptiveThrottler) getNetworkAndDiskMetrics(ctx context.Context) (networkMBps, diskReadMBps, diskWriteMBps float64) {
	currentTime := time.Now()

	// Network metrics
	netStats, err := psnet.IOCountersWithContext(ctx, false)
	if err != nil {
		logger.Debug("Failed to get network metrics", "error", err)
		// Fallback to estimation
		return at.estimateNetworkAndIOMetricsFallback()
	}

	// Disk I/O metrics
	diskStats, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		logger.Debug("Failed to get disk metrics", "error", err)
		// Fallback to basic estimation
		_, fallbackDiskRead, fallbackDiskWrite := at.estimateNetworkAndIOMetricsFallback()
		return 0, fallbackDiskRead, fallbackDiskWrite
	}

	at.metricsHistoryMutex.Lock()
	defer at.metricsHistoryMutex.Unlock()

	// Check if we have previous metrics for delta calculation
	if at.lastNetworkStats != nil && at.lastDiskStats != nil && !at.lastMetricsTime.IsZero() {
		timeDelta := currentTime.Sub(at.lastMetricsTime).Seconds()
		if timeDelta > 0 && timeDelta < 300 { // Only use deltas if reasonable time window (< 5 minutes)

			// Calculate network throughput from deltas
			var deltaBytesRecv, deltaBytesSent uint64
			for i, stat := range netStats {
				if i < len(at.lastNetworkStats) {
					lastStat := at.lastNetworkStats[i]
					if stat.BytesRecv >= lastStat.BytesRecv && stat.BytesSent >= lastStat.BytesSent {
						deltaBytesRecv += stat.BytesRecv - lastStat.BytesRecv
						deltaBytesSent += stat.BytesSent - lastStat.BytesSent
					}
				}
			}
			totalDeltaBytes := deltaBytesRecv + deltaBytesSent
			networkMBps = float64(totalDeltaBytes) / (1024 * 1024) / timeDelta
			networkMBps = math.Min(networkMBps, 1000.0) // Cap at 1000 MB/s

			// Calculate disk throughput from deltas
			var deltaReadBytes, deltaWriteBytes uint64
			for device, stat := range diskStats {
				if lastStat, exists := at.lastDiskStats[device]; exists {
					if stat.ReadBytes >= lastStat.ReadBytes && stat.WriteBytes >= lastStat.WriteBytes {
						deltaReadBytes += stat.ReadBytes - lastStat.ReadBytes
						deltaWriteBytes += stat.WriteBytes - lastStat.WriteBytes
					}
				}
			}
			diskReadMBps = float64(deltaReadBytes) / (1024 * 1024) / timeDelta
			diskWriteMBps = float64(deltaWriteBytes) / (1024 * 1024) / timeDelta
			diskReadMBps = math.Min(diskReadMBps, 1000.0) // Cap at reasonable values
			diskWriteMBps = math.Min(diskWriteMBps, 1000.0)
		} else {
			// Time delta too large or small, use fallback
			return at.estimateNetworkAndIOMetricsFallback()
		}
	} else {
		// No previous metrics, initialize with fallback values
		networkMBps, diskReadMBps, diskWriteMBps = at.estimateNetworkAndIOMetricsFallback()
	}

	// Store current metrics for next delta calculation
	at.lastNetworkStats = make([]psnet.IOCountersStat, len(netStats))
	copy(at.lastNetworkStats, netStats)

	at.lastDiskStats = make(map[string]disk.IOCountersStat)
	for device, stat := range diskStats {
		at.lastDiskStats[device] = stat
	}
	at.lastMetricsTime = currentTime

	return networkMBps, diskReadMBps, diskWriteMBps
}

// estimateNetworkAndIOMetricsFallback provides fallback estimates when gopsutil fails
func (at *AdaptiveThrottler) estimateNetworkAndIOMetricsFallback() (networkMBps, diskReadMBps, diskWriteMBps float64) {
	// Basic estimation based on runtime metrics - not accurate but prevents crashes
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Very rough estimates based on memory allocation patterns
	allocRate := float64(memStats.Alloc) / (1024 * 1024) // MB
	networkMBps = math.Min(allocRate/100, 50.0)          // Estimate based on allocation rate
	diskReadMBps = math.Min(allocRate/200, 25.0)         // Conservative disk read estimate
	diskWriteMBps = math.Min(allocRate/400, 10.0)        // Conservative disk write estimate

	return networkMBps, diskReadMBps, diskWriteMBps
}

// detectContainerEnvironment checks if we're running in a containerized environment
func (at *AdaptiveThrottler) detectContainerEnvironment() {
	at.mu.Lock()
	defer at.mu.Unlock()

	// Check for common container indicators
	containerIndicators := []string{
		"/.dockerenv",        // Docker
		"/run/.containerenv", // Podman
	}

	for _, indicator := range containerIndicators {
		if _, err := os.Stat(indicator); err == nil {
			at.isContainerized = true
			logger.Info("Container environment detected", "indicator", indicator)
			break
		}
	}

	// Check for cgroup mount and determine version
	if !at.isContainerized {
		// Check for container via cgroup
		if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
			content := string(data)
			if strings.Contains(content, "docker") || strings.Contains(content, "containerd") ||
				strings.Contains(content, "kubepods") || strings.Contains(content, "libpod") {
				at.isContainerized = true
				logger.Info("Container environment detected via cgroup")
			}
		}
	}

	if at.isContainerized {
		// Determine cgroup version
		if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
			at.cgroupVersion = 2
			at.cgroupBasePath = "/sys/fs/cgroup"
			logger.Info("Using cgroup v2 for container monitoring")
		} else if _, err := os.Stat("/sys/fs/cgroup/memory"); err == nil {
			at.cgroupVersion = 1
			at.cgroupBasePath = "/sys/fs/cgroup"
			logger.Info("Using cgroup v1 for container monitoring")
		} else {
			logger.Warn("Container detected but cgroup filesystem not found")
			at.isContainerized = false
		}

		if at.isContainerized {
			at.detectContainerLimits()
		}
	} else {
		logger.Info("Running in non-containerized environment, using host metrics")
	}
}

// detectContainerLimits reads container resource limits from cgroups
func (at *AdaptiveThrottler) detectContainerLimits() {
	limits := ContainerLimits{DetectedAt: time.Now()}

	if at.cgroupVersion == 2 {
		// cgroup v2
		limits.MemoryLimitBytes = at.readCgroupInt64("/sys/fs/cgroup/memory.max")
		cpuMax := at.readCgroupString("/sys/fs/cgroup/cpu.max")
		if cpuMax != "" && cpuMax != "max" {
			parts := strings.Fields(cpuMax)
			if len(parts) == 2 {
				limits.CPUQuota, _ = strconv.ParseInt(parts[0], 10, 64)
				limits.CPUPeriod, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}
		limits.CPUShares = at.readCgroupInt64("/sys/fs/cgroup/cpu.weight") * 1024 / 100 // Convert weight to shares approximation
	} else {
		// cgroup v1
		limits.MemoryLimitBytes = at.readCgroupInt64("/sys/fs/cgroup/memory/memory.limit_in_bytes")
		limits.CPUQuota = at.readCgroupInt64("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
		limits.CPUPeriod = at.readCgroupInt64("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
		limits.CPUShares = at.readCgroupInt64("/sys/fs/cgroup/cpu/cpu.shares")

		// Read block I/O throttle settings
		limits.BlkioThrottleRead = at.readBlkioThrottle("/sys/fs/cgroup/blkio/blkio.throttle.read_bps_device")
		limits.BlkioThrottleWrite = at.readBlkioThrottle("/sys/fs/cgroup/blkio/blkio.throttle.write_bps_device")
	}

	// Calculate maximum CPU percentage
	if limits.CPUQuota > 0 && limits.CPUPeriod > 0 {
		limits.MaxCPUPercent = float64(limits.CPUQuota) / float64(limits.CPUPeriod) * 100
		logger.Info("Container CPU limit detected", "max_cpu_percent", limits.MaxCPUPercent)
	} else {
		limits.MaxCPUPercent = float64(runtime.NumCPU()) * 100 // No limit, use host CPU count
	}

	// Check for unrealistic memory limits (often indicates no limit set)
	if limits.MemoryLimitBytes > (1 << 62) { // Very large number indicates no limit
		limits.MemoryLimitBytes = 0
		logger.Info("No container memory limit detected")
	} else if limits.MemoryLimitBytes > 0 {
		logger.Info("Container memory limit detected", "limit_gb", float64(limits.MemoryLimitBytes)/(1024*1024*1024))
	}

	at.containerLimits = limits
}

// readCgroupInt64 reads an integer value from a cgroup file
func (at *AdaptiveThrottler) readCgroupInt64(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}

	return value
}

// readCgroupString reads a string value from a cgroup file
func (at *AdaptiveThrottler) readCgroupString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// readBlkioThrottle reads block I/O throttle settings
func (at *AdaptiveThrottler) readBlkioThrottle(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				value, _ := strconv.ParseInt(parts[1], 10, 64)
				return value // Return first non-zero throttle found
			}
		}
	}

	return 0
}

// getContainerMetrics reads current container resource usage from cgroups
func (at *AdaptiveThrottler) getContainerMetrics() (*ContainerMetrics, error) {
	if !at.isContainerized {
		return nil, fmt.Errorf("not running in container")
	}

	metrics := &ContainerMetrics{LastUpdated: time.Now()}

	if at.cgroupVersion == 2 {
		// cgroup v2
		metrics.MemoryUsageBytes = at.readCgroupInt64("/sys/fs/cgroup/memory.current")

		// For CPU usage in cgroup v2, we rely on gopsutil fallback in the calling function
		// as it provides more reliable cross-platform CPU percentage calculations

		// Read I/O stats
		ioStat := at.readCgroupString("/sys/fs/cgroup/io.stat")
		if ioStat != "" {
			for _, line := range strings.Split(ioStat, "\n") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					for _, part := range parts[1:] {
						if strings.HasPrefix(part, "rbytes=") {
							value, _ := strconv.ParseInt(strings.TrimPrefix(part, "rbytes="), 10, 64)
							metrics.BlkioReadBytes += value
						} else if strings.HasPrefix(part, "wbytes=") {
							value, _ := strconv.ParseInt(strings.TrimPrefix(part, "wbytes="), 10, 64)
							metrics.BlkioWriteBytes += value
						}
					}
				}
			}
		}
	} else {
		// cgroup v1
		metrics.MemoryUsageBytes = at.readCgroupInt64("/sys/fs/cgroup/memory/memory.usage_in_bytes")

		// For CPU usage in cgroup v1, we also rely on gopsutil fallback for consistency

		// Block I/O
		blkioStat := at.readCgroupString("/sys/fs/cgroup/blkio/blkio.io_service_bytes_recursive")
		if blkioStat != "" {
			for _, line := range strings.Split(blkioStat, "\n") {
				parts := strings.Fields(line)
				if len(parts) == 3 {
					if parts[1] == "Read" {
						value, _ := strconv.ParseInt(parts[2], 10, 64)
						metrics.BlkioReadBytes += value
					} else if parts[1] == "Write" {
						value, _ := strconv.ParseInt(parts[2], 10, 64)
						metrics.BlkioWriteBytes += value
					}
				}
			}
		}
	}

	// Calculate memory percentage
	if at.containerLimits.MemoryLimitBytes > 0 {
		metrics.MemoryPercent = float64(metrics.MemoryUsageBytes) / float64(at.containerLimits.MemoryLimitBytes) * 100
	}

	return metrics, nil
}

// DisableThrottling completely disables all throttling for maximum performance
func (at *AdaptiveThrottler) DisableThrottling() {
	at.mu.Lock()
	defer at.mu.Unlock()

	// Set to maximum performance settings
	at.currentLimits = ThrottleLimits{
		WorkerCount:      at.config.MaxWorkers,
		BatchSize:        at.config.MaxBatchSize,
		ProcessingDelay:  0, // No delay
		NetworkBandwidth: at.config.MaxNetworkThroughput,
		IOThrottle:       100.0, // No I/O throttling
		Enabled:          false, // Disable throttling entirely
	}

	// Disable auto-adjustment
	at.autoAdjustEnabled.Store(false)

	logger.Info("Throttling completely disabled for maximum performance",
		"workers", at.config.MaxWorkers,
		"batch_size", at.config.MaxBatchSize)
}

// EnableThrottling re-enables throttling with default settings
func (at *AdaptiveThrottler) EnableThrottling() {
	at.mu.Lock()
	defer at.mu.Unlock()

	// Restore default settings
	at.currentLimits = ThrottleLimits{
		WorkerCount:      at.config.InitialWorkers,
		BatchSize:        at.config.DefaultBatchSize,
		ProcessingDelay:  at.config.DefaultProcessingDelay,
		NetworkBandwidth: at.config.TargetNetworkThroughput,
		IOThrottle:       100.0,
		Enabled:          true,
	}

	// Re-enable auto-adjustment
	at.autoAdjustEnabled.Store(true)

	logger.Info("Throttling re-enabled with default settings")
}
