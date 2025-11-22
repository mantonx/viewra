package system

import "time"

// StorageType represents the type of storage backing a library
type StorageType string

const (
	StorageTypeUnknown StorageType = "unknown"
	StorageTypeLocalSSD StorageType = "local-ssd"
	StorageTypeLocalHDD StorageType = "local-hdd"
	StorageTypeNetwork  StorageType = "network" // CIFS, NFS, SMB
)

// GPUType represents the type of GPU hardware acceleration available
type GPUType string

const (
	GPUTypeNone   GPUType = "none"
	GPUTypeNVIDIA GPUType = "nvidia" // NVIDIA NVENC
	GPUTypeAMD    GPUType = "amd"    // AMD VCE/AMF
	GPUTypeIntel  GPUType = "intel"  // Intel Quick Sync Video
)

// Profile contains detected system characteristics
type Profile struct {
	CPU     CPUProfile
	Memory  MemoryProfile
	Storage StorageProfile
	GPU     GPUProfile
}

// CPUProfile contains CPU information
type CPUProfile struct {
	NumCPU      int      // Logical CPUs (includes hyperthreading)
	NumPhysical int      // Physical CPU cores
	Model       string   // CPU model name
	Features    []string // CPU features (e.g., AVX2, SSE4.2)
}

// MemoryProfile contains memory information
type MemoryProfile struct {
	TotalBytes     uint64 // Total system memory in bytes
	AvailableBytes uint64 // Available memory in bytes
}

// StorageProfile contains storage information
type StorageProfile struct {
	Type         StorageType   // Detected storage type
	IsRemote     bool          // True for network storage
	DetectedPath string        // Path used for detection
	Latency      time.Duration // Estimated I/O latency
}

// GPUProfile contains GPU hardware acceleration information
type GPUProfile struct {
	Type         GPUType  // Detected GPU type
	Available    bool     // True if hardware acceleration is available
	DeviceCount  int      // Number of GPU devices detected
	DeviceNames  []string // Names of detected GPU devices

	// Transcoding capabilities
	HasVAAPI     bool // Intel/AMD Video Acceleration API (Linux)
	HasOpenCL    bool // OpenCL compute support
	HasVulkan    bool // Vulkan compute support
}

// RecommendedSettings contains optimized settings based on system profile
type RecommendedSettings struct {
	// Scanner settings
	ScanWalkers          int // Number of concurrent directory walkers (0 = sequential)
	HashWorkers          int // Number of concurrent file hash workers
	ProcessingWorkers    int // Number of concurrent metadata processing workers
	CheckpointBatchSize  int // Batch size for checkpoint inserts
	ChannelBufferSize    int // Buffer size for channels

	// Transcode settings
	TranscodeWorkers     int    // Number of concurrent transcode workers
	HardwareAccel        string // Hardware acceleration type for transcoding

	// Memory settings
	EnableLargeBuffers   bool // Enable larger buffers if sufficient memory
	ImageCacheSizeMB     int  // Size of image cache in MB
}

// Calculate returns recommended settings based on the system profile
func (p *Profile) Calculate() RecommendedSettings {
	settings := RecommendedSettings{
		CheckpointBatchSize: 10,   // Good default for immediate availability
		ChannelBufferSize:   100,  // Good default for most systems
	}

	// Calculate scan walkers based on storage type
	if p.Storage.IsRemote {
		// Network storage: Enable parallel walking to saturate network bandwidth
		// Use aggressive parallelism to overcome network latency
		settings.ScanWalkers = min(p.CPU.NumCPU, 20) // 20 concurrent directory walks
	} else {
		// Local storage: Sequential walking is often faster
		// Parallel walking can cause excessive disk seeking on HDDs
		if p.Storage.Type == StorageTypeLocalSSD {
			// SSDs can handle some parallelism
			settings.ScanWalkers = max(p.CPU.NumPhysical/4, 4)
		} else {
			// HDDs prefer sequential access
			settings.ScanWalkers = 0 // Sequential walking
		}
	}

	// Calculate hash workers based on CPU and storage type
	if p.Storage.IsRemote {
		// Network storage: I/O latency is bottleneck, use more workers
		// Use more workers to keep network saturated
		settings.HashWorkers = min(p.CPU.NumCPU, 24)
	} else {
		// Local storage: Balance between CPU and disk I/O
		settings.HashWorkers = max(p.CPU.NumPhysical/2, 4)
	}

	// Processing workers: FFprobe is I/O-bound, can run many concurrent processes
	// Network storage benefits from high parallelism to overcome latency
	if p.Storage.IsRemote {
		// Network storage: I/O latency dominates, use more workers
		settings.ProcessingWorkers = max(p.CPU.NumPhysical/2, 8)
	} else {
		// Local storage: Balance CPU and I/O
		settings.ProcessingWorkers = max(p.CPU.NumPhysical/4, 4)
	}

	// Transcode workers: CPU-intensive, limit based on physical cores
	// Reserve cores for system and other tasks
	// If GPU encoding is available, we can handle more concurrent transcodes
	if p.GPU.Available {
		// GPU encoding is much faster and less CPU-intensive
		// Can handle more concurrent transcodes (2-3x more)
		settings.TranscodeWorkers = max(p.CPU.NumPhysical, 8)
	} else {
		// CPU-only encoding: conservative worker count
		settings.TranscodeWorkers = max(p.CPU.NumPhysical/2, 4)
	}

	// Determine hardware acceleration for transcoding
	settings.HardwareAccel = p.GetHardwareAccelType()

	// Memory-based optimizations
	totalGB := p.Memory.TotalBytes / (1024 * 1024 * 1024)
	if totalGB >= 16 {
		settings.EnableLargeBuffers = true
		settings.ImageCacheSizeMB = 512
	} else if totalGB >= 8 {
		settings.ImageCacheSizeMB = 256
	} else {
		settings.ImageCacheSizeMB = 128
	}

	return settings
}

// GetHardwareAccelType returns the hardware acceleration type string for transcoding.
// Maps GPU types to transcoding acceleration values: nvenc, qsv, vaapi, or none.
func (p *Profile) GetHardwareAccelType() string {
	if !p.GPU.Available {
		return "none"
	}

	switch p.GPU.Type {
	case GPUTypeNVIDIA:
		return "nvenc"
	case GPUTypeIntel:
		// Prefer QSV for Intel, but could also use VAAPI
		return "qsv"
	case GPUTypeAMD:
		return "vaapi"
	default:
		return "none"
	}
}
