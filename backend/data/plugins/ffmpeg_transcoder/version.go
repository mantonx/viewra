package main

// Version information for the FFmpeg transcoder plugin
const (
	PluginVersion = "1.0.0"
	PluginBuild   = "dev"
	GoVersion     = "1.24"
)

// GetVersionInfo returns version information for the plugin
func GetVersionInfo() map[string]string {
	return map[string]string{
		"version":    PluginVersion,
		"build":      PluginBuild,
		"go_version": GoVersion,
		"plugin_id":  "ffmpeg_transcoder",
		"type":       "transcoding",
	}
}
