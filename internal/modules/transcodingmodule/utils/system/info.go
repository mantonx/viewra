// Package system provides system information and resource monitoring utilities.
// It detects available CPU cores, memory, and other system capabilities to help
// optimize transcoding performance.
package system

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// SystemInfo provides real system resource information
type SystemInfo struct {
	CPUCores      int
	TotalMemoryMB int64
	FreeMemoryMB  int64
	LoadAverage   []float64
}

// GetSystemInfo retrieves actual system resource information
func GetSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{
		CPUCores: runtime.NumCPU(),
	}

	// Get memory info based on OS
	switch runtime.GOOS {
	case "linux":
		if err := info.getMemoryInfoLinux(); err != nil {
			return nil, fmt.Errorf("failed to get Linux memory info: %w", err)
		}
		if err := info.getLoadAverageLinux(); err != nil {
			// Non-fatal, just log
			info.LoadAverage = []float64{0, 0, 0}
		}
	case "darwin":
		if err := info.getMemoryInfoDarwin(); err != nil {
			return nil, fmt.Errorf("failed to get Darwin memory info: %w", err)
		}
		if err := info.getLoadAverageDarwin(); err != nil {
			info.LoadAverage = []float64{0, 0, 0}
		}
	default:
		// For other OS, use default values
		info.setDefaultMemoryInfo()
	}

	return info, nil
}

// getMemoryInfoLinux reads memory information from /proc/meminfo
func (si *SystemInfo) getMemoryInfoLinux() error {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "MemTotal:":
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				si.TotalMemoryMB = val / 1024 // Convert KB to MB
			}
		case "MemAvailable:":
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				si.FreeMemoryMB = val / 1024 // Convert KB to MB
			}
		}

		// Stop if we have both values
		if si.TotalMemoryMB > 0 && si.FreeMemoryMB > 0 {
			break
		}
	}

	if si.TotalMemoryMB == 0 {
		return fmt.Errorf("could not determine total memory")
	}

	return scanner.Err()
}

// getLoadAverageLinux reads load average from /proc/loadavg
func (si *SystemInfo) getLoadAverageLinux() error {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return fmt.Errorf("invalid loadavg format")
	}

	si.LoadAverage = make([]float64, 3)
	for i := 0; i < 3; i++ {
		if val, err := strconv.ParseFloat(fields[i], 64); err == nil {
			si.LoadAverage[i] = val
		}
	}

	return nil
}

// getMemoryInfoDarwin uses sysctl for macOS
func (si *SystemInfo) getMemoryInfoDarwin() error {
	// This would use sysctl commands
	// For now, use defaults
	si.setDefaultMemoryInfo()
	return nil
}

// getLoadAverageDarwin uses sysctl for macOS
func (si *SystemInfo) getLoadAverageDarwin() error {
	// This would use sysctl
	// For now, return empty
	si.LoadAverage = []float64{0, 0, 0}
	return nil
}

// setDefaultMemoryInfo sets reasonable default values
func (si *SystemInfo) setDefaultMemoryInfo() {
	// Conservative defaults
	si.TotalMemoryMB = 8192 // 8GB
	si.FreeMemoryMB = 4096  // 4GB
	si.LoadAverage = []float64{1.0, 1.0, 1.0}
}

// GetCPUUsagePercent attempts to get current CPU usage
// Returns a value between 0-100 representing total CPU usage
func GetCPUUsagePercent() (float64, error) {
	// This is simplified - in production, you'd track CPU time deltas
	info, err := GetSystemInfo()
	if err != nil {
		return 0, err
	}

	// Use 1-minute load average as rough CPU usage estimate
	// If load > cores, system is overloaded
	if len(info.LoadAverage) > 0 && info.CPUCores > 0 {
		usage := (info.LoadAverage[0] / float64(info.CPUCores)) * 100
		if usage > 100 {
			usage = 100
		}
		return usage, nil
	}

	return 0, fmt.Errorf("unable to determine CPU usage")
}

// GetAvailableMemoryMB returns currently available memory in MB
func GetAvailableMemoryMB() (int64, error) {
	info, err := GetSystemInfo()
	if err != nil {
		return 0, err
	}
	return info.FreeMemoryMB, nil
}

// IsSystemOverloaded checks if the system is under heavy load
func IsSystemOverloaded() bool {
	info, err := GetSystemInfo()
	if err != nil {
		// If we can't determine, assume not overloaded
		return false
	}

	// Check if 1-minute load average exceeds CPU cores
	if len(info.LoadAverage) > 0 && info.LoadAverage[0] > float64(info.CPUCores)*0.8 {
		return true
	}

	// Check if free memory is very low (less than 10%)
	if info.TotalMemoryMB > 0 && info.FreeMemoryMB > 0 {
		freePercent := float64(info.FreeMemoryMB) / float64(info.TotalMemoryMB)
		if freePercent < 0.1 {
			return true
		}
	}

	return false
}
