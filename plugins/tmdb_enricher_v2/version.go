package main

import (
	"fmt"
	"runtime"
	"time"
)

// Build information, populated at build time
var (
	Version   = "2.0.0"
	GitCommit = "dev"
	BuildTime = "unknown"
	GoVersion = runtime.Version()
)

// PluginVersion contains detailed version information
type PluginVersion struct {
	Version   string    `json:"version"`
	GitCommit string    `json:"git_commit"`
	BuildTime string    `json:"build_time"`
	GoVersion string    `json:"go_version"`
	Arch      string    `json:"arch"`
	OS        string    `json:"os"`
	BuildDate time.Time `json:"build_date"`
}

// GetVersion returns detailed version information
func GetVersion() *PluginVersion {
	buildDate, _ := time.Parse(time.RFC3339, BuildTime)
	if BuildTime == "unknown" {
		buildDate = time.Now()
	}

	return &PluginVersion{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
		BuildDate: buildDate,
	}
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	v := GetVersion()
	if v.GitCommit != "dev" && len(v.GitCommit) > 7 {
		return fmt.Sprintf("%s (%s)", v.Version, v.GitCommit[:7])
	}
	return v.Version
}

// GetFullVersionString returns a detailed version string
func GetFullVersionString() string {
	v := GetVersion()
	return fmt.Sprintf("TMDb Enricher v%s\nCommit: %s\nBuilt: %s\nGo: %s\nPlatform: %s/%s",
		v.Version, v.GitCommit, v.BuildTime, v.GoVersion, v.OS, v.Arch)
}
