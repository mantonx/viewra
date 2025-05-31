package scanner

// ScanConfig holds configuration options for the scanner
type ScanConfig struct {
	// Enable parallel scanning for better performance
	ParallelScanningEnabled bool

	// Number of worker goroutines for parallel scanning
	// If 0, will use runtime.NumCPU()
	WorkerCount int

	// Batch size for database operations
	BatchSize int

	// Buffer size for file processing channels
	ChannelBufferSize int

	// Skip hash calculation for files that haven't changed
	// (based on modification time and size)
	SmartHashEnabled bool

	// Use separate workers for metadata extraction
	AsyncMetadataEnabled bool

	// Number of metadata extraction workers
	MetadataWorkerCount int
}

// DefaultScanConfig returns the default scanning configuration
func DefaultScanConfig() *ScanConfig {
	return &ScanConfig{
		ParallelScanningEnabled: true,
		WorkerCount:             0, // Use CPU count
		BatchSize:               50,
		ChannelBufferSize:       100,
		SmartHashEnabled:        true,
		AsyncMetadataEnabled:    true,
		MetadataWorkerCount:     2,
	}
}

// ConservativeScanConfig returns a conservative configuration for slower systems
func ConservativeScanConfig() *ScanConfig {
	return &ScanConfig{
		ParallelScanningEnabled: true,
		WorkerCount:             2,
		BatchSize:               25,
		ChannelBufferSize:       50,
		SmartHashEnabled:        true,
		AsyncMetadataEnabled:    true,
		MetadataWorkerCount:     1,
	}
}

// AggressiveScanConfig returns an aggressive configuration for powerful systems
func AggressiveScanConfig() *ScanConfig {
	return &ScanConfig{
		ParallelScanningEnabled: true,
		WorkerCount:             8,
		BatchSize:               100,
		ChannelBufferSize:       200,
		SmartHashEnabled:        true,
		AsyncMetadataEnabled:    true,
		MetadataWorkerCount:     4,
	}
}

// UltraAggressiveScanConfig returns an ultra-aggressive configuration for very large directories
func UltraAggressiveScanConfig() *ScanConfig {
	return &ScanConfig{
		ParallelScanningEnabled: true,
		WorkerCount:             16,
		BatchSize:               500,
		ChannelBufferSize:       10000, // Much larger buffer for massive directories
		SmartHashEnabled:        true,
		AsyncMetadataEnabled:    true,
		MetadataWorkerCount:     8,
	}
}
