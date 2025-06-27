// Package ffmpeg provides utilities for FFmpeg-based video transcoding.
// This file handles resource management and optimization for FFmpeg processes.
package ffmpeg

import (
	"fmt"
	"runtime"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
)

// ResourceConfig contains FFmpeg resource settings
type ResourceConfig struct {
	ProbeSize       string
	AnalyzeDuration string
	ThreadCount     string
	MuxingQueueSize string
	MaxDelay        string
	FilterThreads   string
	DecodeThreads   string
	RCLookahead     string
}

// ResourceManager optimizes FFmpeg settings based on system resources
type ResourceManager struct {
	logger hclog.Logger
}

// NewResourceManager creates a new resource manager
func NewResourceManager(logger hclog.Logger) *ResourceManager {
	return &ResourceManager{logger: logger}
}

// GetOptimalResources returns optimized resource settings
func (rm *ResourceManager) GetOptimalResources(isABR bool, streamCount int, speedPriority types.SpeedPriority) ResourceConfig {
	cpuCount := runtime.NumCPU()

	// Base thread allocation
	threadsPerStream := 2
	if speedPriority == types.SpeedPriorityFastest {
		threadsPerStream = 1
	} else if speedPriority == types.SpeedPriorityQuality {
		threadsPerStream = 4
	}

	totalThreads := threadsPerStream * streamCount
	if totalThreads > cpuCount {
		totalThreads = cpuCount
	}

	// Adjust for ABR which needs more resources
	if isABR {
		totalThreads = cpuCount / 2 // Use half CPUs for encoding
	}

	return ResourceConfig{
		ProbeSize:       "32M",
		AnalyzeDuration: "10M",
		ThreadCount:     intToString(totalThreads),
		MuxingQueueSize: "1024",
		MaxDelay:        "1000000",
		FilterThreads:   intToString(totalThreads / 2),
		DecodeThreads:   "2",
		RCLookahead:     "24",
	}
}

// GetEncodingPreset returns the optimal encoding preset
func (rm *ResourceManager) GetEncodingPreset(speedPriority types.SpeedPriority, numCPU int) string {
	switch speedPriority {
	case types.SpeedPriorityFastest:
		return "ultrafast"
	case types.SpeedPriorityBalanced:
		return "medium"
	case types.SpeedPriorityQuality:
		return "slow"
	default:
		return "medium"
	}
}

func intToString(i int) string {
	if i < 1 {
		return "1"
	}
	return fmt.Sprintf("%d", i)
}
