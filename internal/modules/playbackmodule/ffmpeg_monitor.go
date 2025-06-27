package playbackmodule

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterMonitoringRoutes registers FFmpeg monitoring endpoints
func RegisterMonitoringRoutes(api *gin.RouterGroup, handler *APIHandler) {
	monitor := api.Group("/monitor")

	// Add middleware to inject handler into context
	monitor.Use(func(c *gin.Context) {
		c.Set("handler", handler)
		c.Next()
	})

	{
		monitor.GET("/ffmpeg-processes", handleGetFFmpegProcesses)
		monitor.POST("/kill-zombies", handleKillZombies)
		monitor.POST("/emergency-cleanup", handleEmergencyCleanup)
	}
}

// Process info
type ProcessInfo struct {
	PID      int    `json:"pid"`
	PPID     int    `json:"ppid"`
	State    string `json:"state"`
	CMD      string `json:"cmd"`
	IsZombie bool   `json:"is_zombie"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

// handleGetFFmpegProcesses returns all FFmpeg processes
func handleGetFFmpegProcesses(c *gin.Context) {
	processes, err := getFFmpegProcesses()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"count":     len(processes),
		"processes": processes,
	})
}

// handleKillZombies kills all zombie FFmpeg processes
func handleKillZombies(c *gin.Context) {
	// Get handler from context
	handlerInterface, exists := c.Get("handler")
	if !exists {
		c.JSON(500, gin.H{"error": "handler not found in context"})
		return
	}

	handler, ok := handlerInterface.(*APIHandler)
	if !ok {
		c.JSON(500, gin.H{"error": "invalid handler type"})
		return
	}

	// Use the manager's zombie cleanup method
	if handler.manager == nil {
		c.JSON(500, gin.H{"error": "manager not initialized"})
		return
	}

	killed, err := handler.manager.KillZombieProcesses()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"killed":  killed,
		"message": fmt.Sprintf("Killed %d zombie processes", killed),
	})
}

// handleEmergencyCleanup performs emergency cleanup
func handleEmergencyCleanup(c *gin.Context) {
	// Kill all FFmpeg processes
	exec.Command("pkill", "-9", "-f", "ffmpeg").Run()

	// Kill plugin processes to clean zombies
	exec.Command("pkill", "-9", "-f", "ffmpeg_software").Run()

	c.JSON(200, gin.H{
		"message": "Emergency cleanup completed",
	})
}

// getFFmpegProcesses returns all FFmpeg processes
func getFFmpegProcesses() ([]ProcessInfo, error) {
	// Use ps with specific format to get process info
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var processes []ProcessInfo
	lines := strings.Split(string(output), "\n")

	for _, line := range lines[1:] { // Skip header
		if !strings.Contains(line, "ffmpeg") || strings.Contains(line, "grep") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid, _ := strconv.Atoi(fields[1])

		// Check if zombie
		isZombie := strings.Contains(line, "<defunct>") || strings.Contains(fields[7], "Z")

		// Get parent PID
		ppidCmd := exec.Command("ps", "-p", fields[1], "-o", "ppid=")
		ppidOut, _ := ppidCmd.Output()
		ppid, _ := strconv.Atoi(strings.TrimSpace(string(ppidOut)))

		processes = append(processes, ProcessInfo{
			PID:      pid,
			PPID:     ppid,
			State:    fields[7],
			CMD:      strings.Join(fields[10:], " "),
			IsZombie: isZombie,
			CPU:      fields[2],
			Memory:   fields[3],
		})
	}

	return processes, nil
}
