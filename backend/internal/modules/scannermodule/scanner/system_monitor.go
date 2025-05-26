package scanner

import (
	"runtime"
	"sync"
	"time"
)

// SystemLoadMonitor tracks system load metrics to help with adaptive scaling
type SystemLoadMonitor struct {
	mu           sync.RWMutex
	cpuUsage     float64 // CPU usage as percentage (0-100)
	memoryUsage  float64 // Memory usage as percentage (0-100)
	ioWait       float64 // I/O wait as percentage (0-100)
	updateTime   time.Time
	
	// System info
	numCPU       int
	maxThreads   int
}

// NewSystemLoadMonitor creates a new system load monitor
func NewSystemLoadMonitor() *SystemLoadMonitor {
	monitor := &SystemLoadMonitor{
		numCPU:     runtime.NumCPU(),
		maxThreads: runtime.GOMAXPROCS(0),
		updateTime: time.Now(),
	}
	
	// Start background monitoring
	go monitor.backgroundMonitor()
	
	return monitor
}

// backgroundMonitor periodically updates system load metrics
func (m *SystemLoadMonitor) backgroundMonitor() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		m.updateMetrics()
	}
}

// updateMetrics refreshes the system load metrics
func (m *SystemLoadMonitor) updateMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Get CPU and memory stats from runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Calculate memory usage percentage
	m.memoryUsage = float64(memStats.Alloc) / float64(memStats.Sys) * 100
	
	// CPU usage is more complex to get accurately in Go
	// In a real implementation, we might:
	// 1. Use a syscall to read /proc/stat (Linux)
	// 2. Use CGo to call into host OS libraries
	// 3. Use an external library like gopsutil
	//
	// For this implementation, we'll use numGoroutines as a proxy for CPU load
	numGoroutines := runtime.NumGoroutine()
	m.cpuUsage = float64(numGoroutines) / float64(m.maxThreads*10) * 100
	if m.cpuUsage > 100 {
		m.cpuUsage = 100
	}
	
	m.updateTime = time.Now()
}

// GetMetrics returns the current system load metrics
func (m *SystemLoadMonitor) GetMetrics() (cpuUsage, memoryUsage, ioWait float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.cpuUsage, m.memoryUsage, m.ioWait
}

// GetSystemInfo returns system hardware information
func (m *SystemLoadMonitor) GetSystemInfo() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"num_cpu":     m.numCPU,
		"max_threads": m.maxThreads,
		"goroutines":  runtime.NumGoroutine(),
	}
}

// ShouldScaleUp returns true if system conditions support adding workers
func (m *SystemLoadMonitor) ShouldScaleUp() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Don't scale up if CPU is already heavily loaded
	if m.cpuUsage > 80 {
		return false
	}
	
	// Don't scale up if memory usage is too high
	if m.memoryUsage > 90 {
		return false
	}
	
	return true
}

// GetLoadScore returns a load score between 0-100 where higher means more loaded
func (m *SystemLoadMonitor) GetLoadScore() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Weight the different metrics
	cpuWeight := 0.6
	memWeight := 0.3
	ioWeight := 0.1
	
	score := m.cpuUsage*cpuWeight + m.memoryUsage*memWeight + m.ioWait*ioWeight
	
	if score > 100 {
		score = 100
	}
	
	return int(score)
}