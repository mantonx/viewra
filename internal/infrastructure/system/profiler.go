package system

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// ProfileSystem quickly detects system characteristics and returns a profile
// This function is designed to be fast (<100ms) and safe for startup
func ProfileSystem(ctx context.Context) (*Profile, error) {
	profile := &Profile{}

	// Use context with timeout to ensure we never hang on startup
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Profile CPU (fast: ~1ms)
	if err := profileCPU(profile); err != nil {
		// Non-fatal: use defaults
		profile.CPU.NumCPU = runtime.NumCPU()
		profile.CPU.NumPhysical = runtime.NumCPU() / 2 // Assume hyperthreading
	}

	// Profile memory (fast: ~1ms)
	if err := profileMemory(profile); err != nil {
		// Non-fatal: use defaults
		profile.Memory.TotalBytes = 8 * 1024 * 1024 * 1024 // Default to 8GB
	}

	// Profile GPU (fast: ~10ms)
	if err := profileGPU(profile); err != nil {
		// Non-fatal: assume no GPU acceleration
		profile.GPU = GPUProfile{
			Type:      GPUTypeNone,
			Available: false,
		}
	}

	// Storage profiling is deferred until library scan
	// We can't profile storage without knowing the library path
	// This will be done per-library when scanning starts

	return profile, nil
}

// ProfileStorage detects storage characteristics for a specific path
// This should be called once per library, not at startup
// Fast operation: <50ms typically
func ProfileStorage(ctx context.Context, path string) StorageProfile {
	profile := StorageProfile{
		Type:         StorageTypeUnknown,
		IsRemote:     false,
		DetectedPath: path,
		Latency:      0,
	}

	// Use context with timeout
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Check if path is a network mount (CIFS, NFS, etc.)
	if isNetworkMount(path) {
		profile.Type = StorageTypeNetwork
		profile.IsRemote = true
		// Network latency is typically 5-20ms
		profile.Latency = 10 * time.Millisecond
		return profile
	}

	// For local storage, try to detect SSD vs HDD
	// This is best-effort and may not work on all systems
	if isSSD(path) {
		profile.Type = StorageTypeLocalSSD
		profile.Latency = 1 * time.Millisecond
	} else {
		profile.Type = StorageTypeLocalHDD
		profile.Latency = 10 * time.Millisecond
	}

	return profile
}

// profileCPU detects CPU information
func profileCPU(profile *Profile) error {
	profile.CPU.NumCPU = runtime.NumCPU()

	// Try to read /proc/cpuinfo for more details (Linux only)
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		// Non-Linux system or permission issue
		// Assume hyperthreading: physical cores = logical / 2
		profile.CPU.NumPhysical = profile.CPU.NumCPU / 2
		if profile.CPU.NumPhysical < 1 {
			profile.CPU.NumPhysical = 1
		}
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	physicalCores := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()

		// Get CPU model name
		if strings.HasPrefix(line, "model name") {
			if parts := strings.Split(line, ":"); len(parts) == 2 {
				profile.CPU.Model = strings.TrimSpace(parts[1])
			}
		}

		// Count unique physical cores
		if strings.HasPrefix(line, "core id") {
			if parts := strings.Split(line, ":"); len(parts) == 2 {
				coreID := strings.TrimSpace(parts[1])
				physicalCores[coreID] = true
			}
		}
	}

	// Set physical core count
	if len(physicalCores) > 0 {
		profile.CPU.NumPhysical = len(physicalCores)
	} else {
		// Fallback: assume hyperthreading
		profile.CPU.NumPhysical = profile.CPU.NumCPU / 2
		if profile.CPU.NumPhysical < 1 {
			profile.CPU.NumPhysical = 1
		}
	}

	return nil
}

// profileMemory detects memory information
func profileMemory(profile *Profile) error {
	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		return err
	}

	// Total RAM
	profile.Memory.TotalBytes = sysinfo.Totalram * uint64(sysinfo.Unit)

	// Available RAM (free + buffers + cached)
	profile.Memory.AvailableBytes = sysinfo.Freeram * uint64(sysinfo.Unit)

	return nil
}

// isNetworkMount checks if a path is on a network filesystem
func isNetworkMount(path string) bool {
	// Read /proc/mounts to check filesystem type
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		mountPoint := fields[1]
		fsType := fields[2]

		// Check if our path starts with this mount point
		if strings.HasPrefix(path, mountPoint) {
			// Check for network filesystem types
			networkFS := []string{"cifs", "nfs", "nfs4", "smb", "smbfs", "fuse.sshfs"}
			for _, nfs := range networkFS {
				if fsType == nfs {
					return true
				}
			}
		}
	}

	return false
}

// isSSD attempts to detect if a path is on an SSD
// This is best-effort and may not work on all systems
func isSSD(path string) bool {
	// Try to check rotational flag in /sys/block
	// This only works on Linux and requires the path to be on a block device

	// For now, return false (assume HDD) as a safe default
	// A more sophisticated implementation could:
	// 1. Find the device for the path
	// 2. Check /sys/block/<device>/queue/rotational (0 = SSD, 1 = HDD)
	// 3. Handle edge cases like RAID, LVM, etc.

	return false
}

// GetRecommendedSettings is a convenience function that profiles the system
// and returns recommended settings
func GetRecommendedSettings(ctx context.Context) (RecommendedSettings, error) {
	profile, err := ProfileSystem(ctx)
	if err != nil {
		// Return conservative defaults on error
		return RecommendedSettings{
			HashWorkers:         8,
			ProcessingWorkers:   2,
			TranscodeWorkers:    4,
			CheckpointBatchSize: 10,
			ChannelBufferSize:   100,
			EnableLargeBuffers:  false,
			ImageCacheSizeMB:    128,
		}, err
	}

	return profile.Calculate(), nil
}

// UpdateForLibraryPath updates the storage profile based on a library path
// Call this when starting a scan to get path-specific optimizations
func (p *Profile) UpdateForLibraryPath(ctx context.Context, libraryPath string) {
	p.Storage = ProfileStorage(ctx, libraryPath)
}

// profileGPU detects GPU hardware acceleration capabilities
func profileGPU(profile *Profile) error {
	profile.GPU = GPUProfile{
		Type:        GPUTypeNone,
		Available:   false,
		DeviceCount: 0,
		DeviceNames: []string{},
		HasVAAPI:    false,
		HasOpenCL:   false,
		HasVulkan:   false,
	}

	// Check for NVIDIA GPUs via nvidia-smi
	if devices := detectNVIDIAGPUs(); len(devices) > 0 {
		profile.GPU.Type = GPUTypeNVIDIA
		profile.GPU.Available = true
		profile.GPU.DeviceCount = len(devices)
		profile.GPU.DeviceNames = devices
	}

	// Check for AMD GPUs via rocm-smi or lspci
	if profile.GPU.Type == GPUTypeNone {
		if devices := detectAMDGPUs(); len(devices) > 0 {
			profile.GPU.Type = GPUTypeAMD
			profile.GPU.Available = true
			profile.GPU.DeviceCount = len(devices)
			profile.GPU.DeviceNames = devices
		}
	}

	// Check for Intel GPUs via lspci
	if profile.GPU.Type == GPUTypeNone {
		if devices := detectIntelGPUs(); len(devices) > 0 {
			profile.GPU.Type = GPUTypeIntel
			profile.GPU.Available = true
			profile.GPU.DeviceCount = len(devices)
			profile.GPU.DeviceNames = devices
		}
	}

	// Detect transcoding capabilities
	profile.GPU.HasVAAPI = detectVAAPI()
	profile.GPU.HasOpenCL = detectOpenCL()
	profile.GPU.HasVulkan = detectVulkan()

	return nil
}

// detectNVIDIAGPUs attempts to detect NVIDIA GPUs using nvidia-smi
func detectNVIDIAGPUs() []string {
	// Try nvidia-smi to get GPU names
	file, err := os.Open("/proc/driver/nvidia/gpus")
	if err != nil {
		return nil
	}
	defer file.Close()

	// If the directory exists, we have NVIDIA GPUs
	// Read GPU directories
	entries, err := os.ReadDir("/proc/driver/nvidia/gpus")
	if err != nil || len(entries) == 0 {
		return nil
	}

	devices := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			// Try to read the GPU info
			infoPath := "/proc/driver/nvidia/gpus/" + entry.Name() + "/information"
			data, err := os.ReadFile(infoPath)
			if err == nil {
				// Parse GPU model from info file
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "Model:") {
						parts := strings.SplitN(line, ":", 2)
						if len(parts) == 2 {
							devices = append(devices, strings.TrimSpace(parts[1]))
							break
						}
					}
				}
			} else {
				// Fallback: just use the directory name
				devices = append(devices, "NVIDIA GPU")
			}
		}
	}

	return devices
}

// detectAMDGPUs attempts to detect AMD GPUs
func detectAMDGPUs() []string {
	// Check for AMD GPUs via lspci
	file, err := os.Open("/sys/class/drm")
	if err != nil {
		return nil
	}
	defer file.Close()

	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}

	devices := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "card") && !strings.Contains(entry.Name(), "-") {
			// Check if it's an AMD GPU
			vendorPath := "/sys/class/drm/" + entry.Name() + "/device/vendor"
			vendor, err := os.ReadFile(vendorPath)
			if err == nil && strings.TrimSpace(string(vendor)) == "0x1002" {
				// AMD vendor ID is 0x1002
				devices = append(devices, "AMD GPU")
			}
		}
	}

	return devices
}

// detectIntelGPUs attempts to detect Intel GPUs with Quick Sync
func detectIntelGPUs() []string {
	// Check for Intel GPUs via /sys/class/drm
	file, err := os.Open("/sys/class/drm")
	if err != nil {
		return nil
	}
	defer file.Close()

	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}

	devices := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "card") && !strings.Contains(entry.Name(), "-") {
			// Check if it's an Intel GPU
			vendorPath := "/sys/class/drm/" + entry.Name() + "/device/vendor"
			vendor, err := os.ReadFile(vendorPath)
			if err == nil && strings.TrimSpace(string(vendor)) == "0x8086" {
				// Intel vendor ID is 0x8086
				devices = append(devices, "Intel GPU")
			}
		}
	}

	return devices
}

// detectVAAPI checks for VAAPI (Video Acceleration API) support
func detectVAAPI() bool {
	// Check for VAAPI render nodes
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return true
	}
	// Check for older VAAPI devices
	if _, err := os.Stat("/dev/dri/card0"); err == nil {
		return true
	}
	return false
}

// detectOpenCL checks for OpenCL support
func detectOpenCL() bool {
	// Check for common OpenCL ICD loader paths
	paths := []string{
		"/etc/OpenCL/vendors",
		"/usr/lib/x86_64-linux-gnu/libOpenCL.so",
		"/usr/lib64/libOpenCL.so",
		"/usr/lib/libOpenCL.so",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// detectVulkan checks for Vulkan support
func detectVulkan() bool {
	// Check for Vulkan ICD loader
	paths := []string{
		"/usr/share/vulkan/icd.d",
		"/etc/vulkan/icd.d",
		"/usr/lib/x86_64-linux-gnu/libvulkan.so",
		"/usr/lib64/libvulkan.so",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// String returns a human-readable summary of the profile
func (p *Profile) String() string {
	return fmt.Sprintf(
		"CPU: %d cores (%d logical), %s | Memory: %.1f GB | Storage: %s | GPU: %s",
		p.CPU.NumPhysical,
		p.CPU.NumCPU,
		p.CPU.Model,
		float64(p.Memory.TotalBytes)/(1024*1024*1024),
		p.Storage.Type,
		p.GPU.Type,
	)
}

// PrintTable writes a formatted table of the system profile to the given writer.
func (p *Profile) PrintTable(w io.Writer, title string, settings *RecommendedSettings) {
	// Print title if provided
	if title != "" {
		fmt.Fprintf(w, "\n%s\n", title)
	}

	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.Border{
				Left:   tw.On,
				Right:  tw.On,
				Top:    tw.On,
				Bottom: tw.On,
			},
			Symbols: tw.NewSymbols(tw.StyleLight),
			Settings: tw.Settings{
				Separators: tw.Separators{
					BetweenColumns: tw.On,
				},
				Lines: tw.Lines{
					ShowHeaderLine: tw.On,
				},
			},
		})),
	)

	table.Header("Component", "Value")

	// CPU information
	table.Append([]string{"CPU Model", p.CPU.Model})
	table.Append([]string{"CPU Cores", fmt.Sprintf("%d physical / %d logical", p.CPU.NumPhysical, p.CPU.NumCPU)})

	// Memory information
	memGB := float64(p.Memory.TotalBytes) / (1024 * 1024 * 1024)
	table.Append([]string{"Memory", fmt.Sprintf("%.1f GB", memGB)})

	// GPU information
	if p.GPU.Available {
		gpuInfo := string(p.GPU.Type)
		if p.GPU.DeviceCount > 1 {
			gpuInfo = fmt.Sprintf("%s (%d devices)", p.GPU.Type, p.GPU.DeviceCount)
		}
		table.Append([]string{"GPU", gpuInfo})

		// Hardware acceleration capabilities
		var caps []string
		if p.GPU.HasVAAPI {
			caps = append(caps, "VAAPI")
		}
		if p.GPU.HasOpenCL {
			caps = append(caps, "OpenCL")
		}
		if p.GPU.HasVulkan {
			caps = append(caps, "Vulkan")
		}
		if len(caps) > 0 {
			table.Append([]string{"HW Capabilities", strings.Join(caps, ", ")})
		}
	} else {
		table.Append([]string{"GPU", "None detected"})
	}

	// Recommended settings (if provided)
	if settings != nil {
		table.Append([]string{"Hash Workers", fmt.Sprintf("%d", settings.HashWorkers)})
		table.Append([]string{"Transcode Workers", fmt.Sprintf("%d", settings.TranscodeWorkers)})
		table.Append([]string{"HW Acceleration", settings.HardwareAccel})
	}

	table.Render()
}
