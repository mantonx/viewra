// Package ffmpeg provides FFmpeg command generation with intelligent resource management.
// The resource manager dynamically adjusts FFmpeg parameters based on system capabilities
// to optimize performance while preventing resource exhaustion.
package ffmpeg

import (
	"runtime"
	"strconv"
	"time"

	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// ResourceManager intelligently calculates optimal FFmpeg resource parameters
// based on system capabilities. It considers CPU cores, available memory,
// encoding type (single stream vs ABR), and quality priorities to determine
// the best configuration for transcoding operations.
//
// The manager aims to:
// - Maximize encoding performance without overloading the system
// - Reserve resources for OS and other processes
// - Adapt to different hardware configurations automatically
// - Balance quality vs speed based on user preferences
type ResourceManager struct {
	logger     types.Logger
	systemInfo *SystemInfo
	lastUpdate int64 // Unix timestamp of last system info update
}

// NewResourceManager creates a new resource manager
func NewResourceManager(logger types.Logger) *ResourceManager {
	rm := &ResourceManager{
		logger: logger,
	}
	
	// Get initial system info
	if err := rm.updateSystemInfo(); err != nil {
		logger.Warn("failed to get system info, using defaults", "error", err)
	}
	
	return rm
}

// ResourceConfig contains all calculated resource parameters for FFmpeg.
// Each field is optimized based on system capabilities and encoding requirements.
type ResourceConfig struct {
	// ThreadCount is the main FFmpeg thread limit (-threads flag)
	ThreadCount string
	
	// CPUPercentage is the target CPU usage (for future cgroup/nice integration)
	CPUPercentage float64
	
	// MemoryLimitMB is the maximum memory FFmpeg should use
	MemoryLimitMB int
	
	// MuxingQueueSize controls the muxer queue size (-max_muxing_queue_size)
	// Larger values prevent "queue full" errors but use more memory
	MuxingQueueSize string
	
	// MaxDelay is the maximum demuxer delay in microseconds (-max_delay)
	// Affects latency and memory usage
	MaxDelay string
	
	// ProbeSize is the number of bytes to analyze (-probesize)
	// Larger values improve format detection accuracy
	ProbeSize string
	
	// AnalyzeDuration is how long to analyze the input in microseconds (-analyzeduration)
	// Longer analysis improves stream detection
	AnalyzeDuration string
	
	// RCLookahead is the number of frames for rate control lookahead
	// Higher values improve quality but increase latency and CPU usage
	RCLookahead string
	
	// RCBufferSize is the rate control buffer size in kilobits
	// Affects quality consistency and memory usage
	RCBufferSize string
	
	// EncoderPreset is the encoding speed preset (ultrafast to veryslow)
	// Faster presets reduce CPU usage but may reduce quality
	EncoderPreset string
	
	// SwscaleThreads controls threads for scaling operations
	SwscaleThreads string
	
	// FilterThreads controls threads for filter graphs (-filter_threads)
	FilterThreads string
	
	// DecodeThreads controls input decoding threads (-threads:0)
	DecodeThreads string
}

// GetOptimalResources calculates the optimal resource configuration for FFmpeg
// based on system capabilities and encoding requirements.
//
// Parameters:
//   - isABR: Whether this is adaptive bitrate encoding (multiple quality levels)
//   - streamCount: Number of streams being encoded (for ABR)
//   - speedPriority: User preference for speed vs quality trade-off
//
// Returns a ResourceConfig with all parameters optimized for the current system.
// The configuration adapts to available CPU cores, memory, and the specific
// encoding scenario to maximize performance without overloading the system.
func (rm *ResourceManager) GetOptimalResources(isABR bool, streamCount int, speedPriority types.SpeedPriority) ResourceConfig {
	// Refresh system info if stale (older than 30 seconds)
	if rm.shouldUpdateSystemInfo() {
		_ = rm.updateSystemInfo()
	}
	
	numCPU := rm.getCPUCores()
	totalMemoryGB := rm.getTotalMemoryGB()
	availableMemoryGB := rm.getAvailableMemoryGB()
	
	config := ResourceConfig{}
	
	// Thread allocations
	config.ThreadCount = rm.calculateThreadCount(isABR, streamCount)
	config.SwscaleThreads = rm.calculateSwscaleThreads(numCPU)
	config.FilterThreads = rm.calculateFilterThreads(numCPU, isABR)
	config.DecodeThreads = rm.calculateDecodeThreads(numCPU)
	
	// Memory and buffer settings (use available memory for dynamic adjustment)
	config.MemoryLimitMB = rm.calculateMemoryLimit(isABR, streamCount, totalMemoryGB, availableMemoryGB)
	config.MuxingQueueSize = rm.calculateMuxingQueueSize(totalMemoryGB, availableMemoryGB, isABR)
	config.RCBufferSize = rm.calculateRateControlBuffer(totalMemoryGB, availableMemoryGB, isABR)
	
	// Timing and analysis settings
	config.MaxDelay = rm.calculateMaxDelay(isABR)
	config.ProbeSize = rm.calculateProbeSize(totalMemoryGB)
	config.AnalyzeDuration = rm.calculateAnalyzeDuration(numCPU)
	config.RCLookahead = rm.calculateRCLookahead(numCPU, speedPriority)
	
	// Encoding preset
	config.EncoderPreset = rm.GetEncodingPreset(speedPriority, numCPU)
	
	// CPU percentage for external limiting
	config.CPUPercentage = rm.calculateCPUPercentage(isABR, streamCount)
	
	return config
}

// calculateThreadCount determines the optimal number of threads for FFmpeg
// based on available CPU cores and encoding type.
//
// For single stream encoding:
//   - Uses 70% of available cores (reserves 30% for system)
//   - Caps at 16 threads (diminishing returns beyond this)
//
// For ABR encoding:
//   - Divides available cores among streams
//   - Ensures minimum 2 threads per stream
//   - Caps at 8 threads per stream
//
// This prevents CPU overload while maintaining good performance.
func (rm *ResourceManager) calculateThreadCount(isABR bool, streamCount int) string {
	numCPU := rm.getCPUCores()
	
	// Adjust available cores based on system load
	cpuReservation := rm.getCPUReservationFactor()
	availableCores := int(float64(numCPU) * cpuReservation)
	if availableCores < 1 {
		availableCores = 1
	}
	
	if isABR && streamCount > 0 {
		return rm.calculateABRThreads(availableCores, streamCount, numCPU)
	}
	
	return rm.calculateSingleStreamThreads(availableCores, numCPU)
}

// calculateABRThreads calculates thread allocation for ABR encoding.
// It divides available cores among streams while ensuring each stream
// gets enough threads for efficient encoding (min 2, max 8 per stream).
func (rm *ResourceManager) calculateABRThreads(availableCores, streamCount, totalCPU int) string {
	// Each stream gets a portion of available cores
	threadsPerStream := availableCores / streamCount
	
	// Ensure minimum efficiency (2 threads per stream)
	if threadsPerStream < 2 {
		threadsPerStream = 2
	}
	
	// Cap at 4 threads per stream (diminishing returns beyond this)
	// This is more conservative to prevent overload
	if threadsPerStream > 4 {
		threadsPerStream = 4
	}
	
	// Calculate total threads
	totalThreads := threadsPerStream * streamCount
	
	// Ensure we don't exceed available cores
	if totalThreads > availableCores {
		totalThreads = availableCores
	}
	
	rm.logger.Debug("calculated ABR thread allocation",
		"cpu_cores", totalCPU,
		"available_cores", availableCores,
		"stream_count", streamCount,
		"threads_per_stream", threadsPerStream,
		"total_threads", totalThreads)
	
	return strconv.Itoa(totalThreads)
}

// calculateSingleStreamThreads calculates threads for single stream encoding.
// Uses most available cores but caps at 16 to avoid diminishing returns
// and excessive memory usage from too many threads.
func (rm *ResourceManager) calculateSingleStreamThreads(availableCores, totalCPU int) string {
	// Use most available cores but cap at 8 (more conservative)
	// This prevents system overload on high-core-count machines
	threads := availableCores
	if threads > 8 {
		threads = 8
	}
	
	rm.logger.Debug("calculated single stream thread allocation",
		"cpu_cores", totalCPU,
		"available_cores", availableCores,
		"threads", threads)
	
	return strconv.Itoa(threads)
}

// calculateCPUPercentage determines target CPU usage percentage.
// This can be used with external tools (nice, cgroups) to limit CPU usage.
// ABR uses lower percentage to prevent system overload from multiple streams.
func (rm *ResourceManager) calculateCPUPercentage(isABR bool, streamCount int) float64 {
	if isABR && streamCount > 1 {
		// For ABR, allow up to 70% CPU usage
		return 70.0
	}
	// For single streams, allow up to 80% CPU usage
	return 80.0
}

// updateSystemInfo refreshes the cached system information
func (rm *ResourceManager) updateSystemInfo() error {
	info, err := GetSystemInfo()
	if err != nil {
		return err
	}
	
	rm.systemInfo = info
	rm.lastUpdate = time.Now().Unix()
	return nil
}

// shouldUpdateSystemInfo checks if system info needs refreshing
func (rm *ResourceManager) shouldUpdateSystemInfo() bool {
	if rm.systemInfo == nil {
		return true
	}
	
	// Update every 30 seconds
	return time.Now().Unix()-rm.lastUpdate > 30
}

// getCPUCores returns the number of CPU cores
func (rm *ResourceManager) getCPUCores() int {
	if rm.systemInfo != nil {
		return rm.systemInfo.CPUCores
	}
	return runtime.NumCPU()
}

// getTotalMemoryGB returns total system memory in GB
func (rm *ResourceManager) getTotalMemoryGB() int {
	if rm.systemInfo != nil && rm.systemInfo.TotalMemoryMB > 0 {
		return int(rm.systemInfo.TotalMemoryMB / 1024)
	}
	return 8 // Default fallback
}

// getAvailableMemoryGB returns available system memory in GB
func (rm *ResourceManager) getAvailableMemoryGB() int {
	if rm.systemInfo != nil && rm.systemInfo.FreeMemoryMB > 0 {
		return int(rm.systemInfo.FreeMemoryMB / 1024)
	}
	// If unknown, assume 50% of total is available
	return rm.getTotalMemoryGB() / 2
}

// getCPUReservationFactor returns how much CPU to reserve for FFmpeg
// based on current system load
func (rm *ResourceManager) getCPUReservationFactor() float64 {
	if rm.systemInfo != nil && len(rm.systemInfo.LoadAverage) > 0 {
		// If system is already loaded, be more conservative
		load1min := rm.systemInfo.LoadAverage[0]
		cores := float64(rm.systemInfo.CPUCores)
		
		if load1min > cores*0.8 {
			// System heavily loaded, use only 25% of cores
			return 0.25
		} else if load1min > cores*0.5 {
			// Moderate load, use 40% of cores
			return 0.4
		}
	}
	
	// Normal load, use 50% of cores (more conservative)
	return 0.5
}

// calculateMemoryLimit determines the maximum memory FFmpeg should use.
//
// Allocation strategy:
//   - Uses up to 60% of system memory for transcoding
//   - Base allocation per stream: 512MB-1GB depending on total memory
//   - ABR: Multiplies by stream count
//   - Single stream: Can use 2x base allocation
//
// This prevents memory exhaustion while allowing efficient buffering.
func (rm *ResourceManager) calculateMemoryLimit(isABR bool, streamCount int, totalMemoryGB int, availableMemoryGB int) int {
	// Use up to 60% of available memory (not total) for transcoding
	// This is more conservative and considers other running processes
	availableMemoryMB := int(float64(availableMemoryGB*1024) * 0.6)
	
	// But also cap at 40% of total memory to be safe
	maxMemoryMB := int(float64(totalMemoryGB*1024) * 0.4)
	if availableMemoryMB > maxMemoryMB {
		availableMemoryMB = maxMemoryMB
	}
	
	// Base memory per stream (MB)
	baseMemoryPerStream := 512
	
	if totalMemoryGB >= 16 {
		// High memory system - be more generous
		baseMemoryPerStream = 1024
	} else if totalMemoryGB >= 8 {
		baseMemoryPerStream = 768
	}
	
	if isABR {
		// ABR needs memory for multiple streams
		totalNeeded := baseMemoryPerStream * streamCount
		// Cap at available memory
		if totalNeeded > availableMemoryMB {
			return availableMemoryMB
		}
		return totalNeeded
	}
	
	// Single stream can use more memory
	singleStreamMemory := baseMemoryPerStream * 2
	if singleStreamMemory > availableMemoryMB {
		return availableMemoryMB
	}
	return singleStreamMemory
}

// calculateMuxingQueueSize determines the optimal muxer queue size.
//
// The muxing queue buffers encoded packets before writing to output.
// Larger queues prevent "queue full" errors but use more memory.
//
// Sizing:
//   - Base: 1024 packets (low memory) to 4096 (high memory)
//   - ABR: Doubles the size due to multiple streams
//
// This balances reliability with memory usage.
func (rm *ResourceManager) calculateMuxingQueueSize(totalMemoryGB int, availableMemoryGB int, isABR bool) string {
	// Base size - consider available memory too
	queueSize := 1024
	
	// Use the lesser of total and available memory for sizing
	effectiveMemory := totalMemoryGB
	if availableMemoryGB < totalMemoryGB/2 {
		// If less than half memory is free, be conservative
		effectiveMemory = availableMemoryGB * 2
	}
	
	if effectiveMemory >= 16 {
		queueSize = 4096
	} else if effectiveMemory >= 8 {
		queueSize = 2048
	}
	
	// ABR needs larger queues
	if isABR {
		queueSize = queueSize * 2
	}
	
	return strconv.Itoa(queueSize)
}

// calculateMaxDelay determines the maximum demuxer-muxer delay in microseconds.
// Higher delays allow more buffering for smoother output but increase latency.
// ABR needs more buffering due to multiple streams.
func (rm *ResourceManager) calculateMaxDelay(isABR bool) string {
	// In microseconds
	if isABR {
		// ABR needs more buffering
		return "1000000" // 1 second
	}
	return "500000" // 0.5 seconds
}

// calculateProbeSize determines how many bytes FFmpeg analyzes to detect format.
//
// Larger probe sizes improve format detection accuracy but increase startup time
// and memory usage. This is especially important for files with late metadata
// or complex container formats.
//
// Sizing:
//   - Low memory (<4GB): 2MB - quick but may miss some streams
//   - Standard (4-16GB): 5MB - good balance
//   - High memory (>16GB): 10MB - thorough analysis
func (rm *ResourceManager) calculateProbeSize(totalMemoryGB int) string {
	// Base probe size in bytes
	probeSize := 5000000 // 5MB
	
	if totalMemoryGB >= 16 {
		// Can afford larger probe for better accuracy
		probeSize = 10000000 // 10MB
	} else if totalMemoryGB < 4 {
		// Limited memory, use smaller probe
		probeSize = 2000000 // 2MB
	}
	
	return strconv.Itoa(probeSize)
}

// calculateAnalyzeDuration determines how long FFmpeg analyzes input streams.
// Longer analysis improves codec/format detection but increases startup time.
// Faster CPUs can afford longer analysis without impacting user experience.
func (rm *ResourceManager) calculateAnalyzeDuration(numCPU int) string {
	// In microseconds
	if numCPU >= 8 {
		// Fast CPU can analyze more
		return "10000000" // 10 seconds
	} else if numCPU < 4 {
		// Slow CPU, quick analysis
		return "3000000" // 3 seconds
	}
	return "5000000" // 5 seconds default
}

// calculateRCLookahead determines the number of frames for rate control lookahead.
//
// Lookahead allows the encoder to make better bitrate decisions by analyzing
// future frames. More lookahead improves quality and bitrate consistency but
// increases latency and CPU usage.
//
// Strategy:
//   - Speed priority: 5 frames (minimal lookahead)
//   - Balanced: 10-20 frames
//   - Quality priority: 20-40 frames (depends on CPU)
func (rm *ResourceManager) calculateRCLookahead(numCPU int, speedPriority types.SpeedPriority) string {
	lookahead := 10 // Base lookahead
	
	if speedPriority == types.SpeedPriorityQuality {
		// Quality mode uses more lookahead
		if numCPU >= 8 {
			lookahead = 40
		} else {
			lookahead = 20
		}
	} else if speedPriority == types.SpeedPriorityFastest {
		// Speed mode uses minimal lookahead
		lookahead = 5
	}
	
	return strconv.Itoa(lookahead)
}

// calculateRateControlBuffer determines the VBV (Video Buffering Verifier) buffer size.
// Larger buffers allow better quality consistency but use more memory.
// ABR uses smaller buffers per stream to fit within memory constraints.
func (rm *ResourceManager) calculateRateControlBuffer(totalMemoryGB int, availableMemoryGB int, isABR bool) string {
	// In kilobits - base on available memory
	bufferSize := 2000 // 2 Mbit
	
	// Consider both total and available memory
	if totalMemoryGB >= 16 && availableMemoryGB >= 8 {
		bufferSize = 8000 // 8 Mbit
	} else if totalMemoryGB >= 8 && availableMemoryGB >= 4 {
		bufferSize = 4000 // 4 Mbit
	}
	
	if isABR {
		// ABR uses smaller buffers per stream
		bufferSize = bufferSize / 2
	}
	
	return strconv.Itoa(bufferSize)
}

// calculateSwscaleThreads determines threads for video scaling operations.
// Swscale has diminishing returns beyond 4 threads, so we cap there
// regardless of available CPU cores.
func (rm *ResourceManager) calculateSwscaleThreads(numCPU int) string {
	// Swscale benefits from multiple threads but has diminishing returns
	threads := numCPU / 4
	if threads < 1 {
		threads = 1
	}
	if threads > 4 {
		threads = 4
	}
	return strconv.Itoa(threads)
}

// calculateFilterThreads determines threads for complex filter operations.
// More threads help with complex filters like deinterlacing or effects.
// ABR reduces thread count to share resources among streams.
func (rm *ResourceManager) calculateFilterThreads(numCPU int, isABR bool) string {
	threads := numCPU / 4
	if threads < 1 {
		threads = 1
	}
	if threads > 8 {
		threads = 8
	}
	
	// ABR needs to share threads among streams
	if isABR {
		threads = threads / 2
		if threads < 1 {
			threads = 1
		}
	}
	
	return strconv.Itoa(threads)
}

// calculateDecodeThreads determines threads for input decoding.
// Decoding benefits from multiple threads, especially for high-resolution
// or complex codecs like H.265/HEVC. Capped at 8 for efficiency.
func (rm *ResourceManager) calculateDecodeThreads(numCPU int) string {
	// Decoding can use auto detection efficiently
	// But we can hint at a reasonable number
	threads := numCPU / 2
	if threads < 2 {
		threads = 2
	}
	if threads > 8 {
		threads = 8
	}
	return strconv.Itoa(threads)
}

// GetEncodingPreset selects the optimal x264/x265 encoding preset based on
// system capabilities and user preferences.
//
// Preset selection logic:
//   - Low-power systems (<4 cores): Always ultrafast to prevent overload
//   - Mid-range (4-8 cores): veryfast to faster based on priority
//   - High-performance (8+ cores): faster to fast for better quality
//
// Presets control the speed/quality trade-off:
//   - ultrafast: Minimal CPU, lowest quality
//   - veryfast/faster: Good balance for most systems
//   - fast/medium: Higher quality, more CPU intensive
func (rm *ResourceManager) GetEncodingPreset(speedPriority types.SpeedPriority, cpuCores int) string {
	// Adjust preset based on available CPU power
	if cpuCores < 4 {
		// Low-power system: always use fastest presets
		return "ultrafast"
	} else if cpuCores < 8 {
		// Mid-range system: balance quality and speed
		switch speedPriority {
		case types.SpeedPriorityFastest:
			return "veryfast"
		case types.SpeedPriorityQuality:
			return "faster"
		default:
			return "faster"
		}
	} else {
		// High-performance system: can afford better quality
		switch speedPriority {
		case types.SpeedPriorityFastest:
			return "faster"
		case types.SpeedPriorityQuality:
			return "fast"
		default:
			return "faster"
		}
	}
}