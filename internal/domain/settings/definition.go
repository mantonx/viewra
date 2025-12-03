package settings

// Definition describes a setting for UI generation and validation.
type Definition struct {
	Key         string      `json:"key"`
	Type        ValueType   `json:"type"`
	Category    Category    `json:"category"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Default     any         `json:"default"`
	Options     []Option    `json:"options,omitempty"`
	Validation  *Validation `json:"validation,omitempty"`
	AdminOnly   bool        `json:"adminOnly"`
	Restartable bool        `json:"restartable"` // Requires server restart to take effect

	// Environment variable awareness
	EnvVar   string `json:"envVar,omitempty"` // Associated env var name (if any)
	ReadOnly bool   `json:"readOnly"`         // True for display-only values (detected/computed)
}

// Option represents a selectable option for a setting.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Validation defines validation rules for a setting.
type Validation struct {
	Min      *int   `json:"min,omitempty"`
	Max      *int   `json:"max,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// SystemSettingDefinitions contains all known system settings.
// Settings are organized by category and include environment variable awareness.
var SystemSettingDefinitions = []Definition{
	// ============================================================
	// SYSTEM INFO (Read-only, detected values)
	// ============================================================
	{
		Key:         "system.cpu_model",
		Type:        TypeString,
		Category:    CategorySystem,
		Label:       "CPU Model",
		Description: "Detected CPU model name",
		Default:     "",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "system.cpu_cores",
		Type:        TypeString,
		Category:    CategorySystem,
		Label:       "CPU Cores",
		Description: "Physical and logical CPU cores",
		Default:     "",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "system.memory_total",
		Type:        TypeString,
		Category:    CategorySystem,
		Label:       "Total Memory",
		Description: "Total system RAM",
		Default:     "",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "system.gpu_type",
		Type:        TypeString,
		Category:    CategorySystem,
		Label:       "GPU Type",
		Description: "Detected GPU vendor and acceleration type",
		Default:     "",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "system.gpu_devices",
		Type:        TypeJSON,
		Category:    CategorySystem,
		Label:       "GPU Devices",
		Description: "List of detected GPU devices",
		Default:     []string{},
		AdminOnly:   true,
		ReadOnly:    true,
	},

	// ============================================================
	// SERVER (Mostly read-only or env-var controlled)
	// ============================================================
	{
		Key:         "server.version",
		Type:        TypeString,
		Category:    CategoryServer,
		Label:       "Server Version",
		Description: "ViewRA server version",
		Default:     "",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "server.environment",
		Type:        TypeString,
		Category:    CategoryServer,
		Label:       "Environment",
		Description: "Running mode (development or production)",
		Default:     "development",
		EnvVar:      "ENVIRONMENT",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "server.base_url",
		Type:        TypeString,
		Category:    CategoryServer,
		Label:       "External Base URL",
		Description: "External URL for the server (for webhooks, sharing links)",
		Default:     "",
		AdminOnly:   true,
		Restartable: false,
	},

	// ============================================================
	// TRANSCODING
	// ============================================================
	{
		Key:         "transcoding.hw_accel_detected",
		Type:        TypeString,
		Category:    CategoryTranscoding,
		Label:       "Detected Hardware Acceleration",
		Description: "Auto-detected hardware acceleration based on available GPU",
		Default:     "none",
		AdminOnly:   true,
		ReadOnly:    true,
	},
	{
		Key:         "transcoding.hw_accel",
		Type:        TypeString,
		Category:    CategoryTranscoding,
		Label:       "Hardware Acceleration Override",
		Description: "Override auto-detected hardware acceleration (leave as 'auto' to use detected)",
		Default:     "auto",
		EnvVar:      "HARDWARE_ACCEL",
		Options: []Option{
			{Value: "auto", Label: "Auto (use detected)"},
			{Value: "none", Label: "None (CPU only)"},
			{Value: "nvenc", Label: "NVIDIA NVENC"},
			{Value: "qsv", Label: "Intel QuickSync"},
			{Value: "vaapi", Label: "VAAPI (Linux)"},
		},
		AdminOnly:   true,
		Restartable: true,
	},
	{
		Key:         "transcoding.default_quality",
		Type:        TypeString,
		Category:    CategoryTranscoding,
		Label:       "Default Quality",
		Description: "Default video transcoding quality for new transcodes",
		Default:     "720p",
		Options: []Option{
			{Value: "480p", Label: "480p (SD)"},
			{Value: "720p", Label: "720p (HD)"},
			{Value: "1080p", Label: "1080p (Full HD)"},
			{Value: "1440p", Label: "1440p (2K)"},
			{Value: "2160p", Label: "2160p (4K)"},
		},
		AdminOnly:   true,
		Restartable: false,
	},
	{
		Key:         "transcoding.workers",
		Type:        TypeInt,
		Category:    CategoryTranscoding,
		Label:       "Transcode Workers",
		Description: "Number of concurrent transcode jobs",
		Default:     4,
		EnvVar:      "TRANSCODE_WORKERS",
		Validation:  &Validation{Min: intPtr(1), Max: intPtr(16)},
		AdminOnly:   true,
		Restartable: true,
	},
	{
		Key:         "transcoding.tone_mapping_enabled",
		Type:        TypeBool,
		Category:    CategoryTranscoding,
		Label:       "HDR Tone Mapping",
		Description: "Enable automatic HDR to SDR conversion for better compatibility",
		Default:     true,
		EnvVar:      "TONE_MAPPING_ENABLED",
		AdminOnly:   true,
		Restartable: false,
	},
	{
		Key:         "transcoding.tone_mapping_algorithm",
		Type:        TypeString,
		Category:    CategoryTranscoding,
		Label:       "Tone Mapping Algorithm",
		Description: "Algorithm for HDR to SDR conversion",
		Default:     "bt.2390",
		EnvVar:      "TONE_MAPPING_ALGORITHM",
		Options: []Option{
			{Value: "bt.2390", Label: "BT.2390 (Broadcast Standard)"},
			{Value: "hable", Label: "Hable (Filmic)"},
			{Value: "mobius", Label: "Mobius (Smooth)"},
			{Value: "reinhard", Label: "Reinhard (Classic)"},
		},
		AdminOnly:   true,
		Restartable: false,
	},
	{
		Key:         "transcoding.min_free_disk_gb",
		Type:        TypeInt,
		Category:    CategoryTranscoding,
		Label:       "Minimum Free Disk (GB)",
		Description: "Minimum free disk space required to start a transcode",
		Default:     10,
		Validation:  &Validation{Min: intPtr(1), Max: intPtr(100)},
		AdminOnly:   true,
		Restartable: false,
	},

	// ============================================================
	// SCANNING
	// ============================================================
	{
		Key:         "scanning.auto_enabled",
		Type:        TypeBool,
		Category:    CategoryScanning,
		Label:       "Automatic Scanning",
		Description: "Enable automatic periodic library scanning",
		Default:     true,
		EnvVar:      "AUTO_SCAN_ENABLED",
		AdminOnly:   true,
		Restartable: true,
	},
	{
		Key:         "scanning.auto_interval",
		Type:        TypeString,
		Category:    CategoryScanning,
		Label:       "Scan Interval",
		Description: "How often to scan libraries (cron format)",
		Default:     "*/15 * * * *",
		EnvVar:      "AUTO_SCAN_INTERVAL",
		Options: []Option{
			{Value: "*/5 * * * *", Label: "Every 5 minutes"},
			{Value: "*/15 * * * *", Label: "Every 15 minutes"},
			{Value: "*/30 * * * *", Label: "Every 30 minutes"},
			{Value: "0 * * * *", Label: "Every hour"},
			{Value: "0 */6 * * *", Label: "Every 6 hours"},
			{Value: "0 0 * * *", Label: "Daily at midnight"},
		},
		AdminOnly:   true,
		Restartable: true,
	},
	{
		Key:         "scanning.parallel_walkers",
		Type:        TypeInt,
		Category:    CategoryScanning,
		Label:       "Parallel Walkers",
		Description: "Concurrent directory walkers for scanning (higher for network storage)",
		Default:     10,
		EnvVar:      "SCAN_PARALLEL_WALKERS",
		Validation:  &Validation{Min: intPtr(1), Max: intPtr(50)},
		AdminOnly:   true,
		Restartable: true,
	},
	{
		Key:         "scanning.ignore_patterns",
		Type:        TypeJSON,
		Category:    CategoryScanning,
		Label:       "Ignore Patterns",
		Description: "File patterns to ignore during scanning",
		Default:     []string{".*", "*.nfo", "*.txt"},
		AdminOnly:   true,
		Restartable: false,
	},
	{
		Key:         "scanning.job_retention_minutes",
		Type:        TypeInt,
		Category:    CategoryScanning,
		Label:       "Job Retention (minutes)",
		Description: "How long to keep completed scan job records",
		Default:     30,
		EnvVar:      "SCAN_JOB_RETENTION_MINUTES",
		Validation:  &Validation{Min: intPtr(5), Max: intPtr(1440)},
		AdminOnly:   true,
		Restartable: false,
	},
}

// UserSettingDefinitions contains all known user settings.
var UserSettingDefinitions = []Definition{
	{
		Key:         "playback.default_quality",
		Type:        TypeString,
		Category:    CategoryPlayback,
		Label:       "Preferred Quality",
		Description: "Preferred video quality for playback",
		Default:     "auto",
		Options: []Option{
			{Value: "auto", Label: "Auto (adaptive)"},
			{Value: "480p", Label: "480p"},
			{Value: "720p", Label: "720p"},
			{Value: "1080p", Label: "1080p"},
			{Value: "max", Label: "Maximum available"},
		},
		AdminOnly: false,
	},
	{
		Key:         "playback.autoplay",
		Type:        TypeBool,
		Category:    CategoryPlayback,
		Label:       "Auto-play Next",
		Description: "Automatically play the next episode or track",
		Default:     true,
		AdminOnly:   false,
	},
	{
		Key:         "playback.preferred_audio_language",
		Type:        TypeString,
		Category:    CategoryPlayback,
		Label:       "Preferred Audio Language",
		Description: "Default audio track language for playback (ISO 639-2 code)",
		Default:     "eng",
		Options: []Option{
			{Value: "eng", Label: "English"},
			{Value: "spa", Label: "Spanish"},
			{Value: "fra", Label: "French"},
			{Value: "deu", Label: "German"},
			{Value: "ita", Label: "Italian"},
			{Value: "por", Label: "Portuguese"},
			{Value: "jpn", Label: "Japanese"},
			{Value: "kor", Label: "Korean"},
			{Value: "zho", Label: "Chinese"},
			{Value: "rus", Label: "Russian"},
			{Value: "ara", Label: "Arabic"},
			{Value: "hin", Label: "Hindi"},
			{Value: "tha", Label: "Thai"},
			{Value: "vie", Label: "Vietnamese"},
			{Value: "und", Label: "Original/Unknown"},
		},
		AdminOnly: false,
	},
	{
		Key:         "playback.preferred_subtitle_language",
		Type:        TypeString,
		Category:    CategoryPlayback,
		Label:       "Preferred Subtitle Language",
		Description: "Default subtitle track language for playback (ISO 639-2 code)",
		Default:     "off",
		Options: []Option{
			{Value: "off", Label: "Subtitles Off"},
			{Value: "eng", Label: "English"},
			{Value: "spa", Label: "Spanish"},
			{Value: "fra", Label: "French"},
			{Value: "deu", Label: "German"},
			{Value: "ita", Label: "Italian"},
			{Value: "por", Label: "Portuguese"},
			{Value: "jpn", Label: "Japanese"},
			{Value: "kor", Label: "Korean"},
			{Value: "zho", Label: "Chinese"},
			{Value: "rus", Label: "Russian"},
			{Value: "ara", Label: "Arabic"},
			{Value: "hin", Label: "Hindi"},
			{Value: "tha", Label: "Thai"},
			{Value: "vie", Label: "Vietnamese"},
		},
		AdminOnly: false,
	},
	{
		Key:         "playback.subtitle_mode",
		Type:        TypeString,
		Category:    CategoryPlayback,
		Label:       "Subtitle Mode",
		Description: "When to show subtitles automatically",
		Default:     "foreign_audio",
		Options: []Option{
			{Value: "off", Label: "Always Off"},
			{Value: "foreign_audio", Label: "Only for Foreign Audio"},
			{Value: "always", Label: "Always On"},
		},
		AdminOnly: false,
	},
	{
		Key:         "playback.prefer_sdh",
		Type:        TypeBool,
		Category:    CategoryPlayback,
		Label:       "Prefer SDH Subtitles",
		Description: "Prefer Subtitles for Deaf/Hard of Hearing when available",
		Default:     false,
		AdminOnly:   false,
	},
	{
		Key:         "playback.prefer_forced",
		Type:        TypeBool,
		Category:    CategoryPlayback,
		Label:       "Always Show Forced Subtitles",
		Description: "Always display forced subtitles (translations of foreign dialogue)",
		Default:     true,
		AdminOnly:   false,
	},
	{
		Key:         "ui.theme",
		Type:        TypeString,
		Category:    CategoryUI,
		Label:       "Theme",
		Description: "UI color theme",
		Default:     "system",
		Options: []Option{
			{Value: "system", Label: "System default"},
			{Value: "light", Label: "Light"},
			{Value: "dark", Label: "Dark"},
		},
		AdminOnly: false,
	},
	{
		Key:         "ui.language",
		Type:        TypeString,
		Category:    CategoryUI,
		Label:       "Language",
		Description: "UI language",
		Default:     "en",
		Options: []Option{
			{Value: "en", Label: "English"},
		},
		AdminOnly: false,
	},
}

// GetSystemDefinition returns the definition for a system setting key.
func GetSystemDefinition(key string) *Definition {
	for _, def := range SystemSettingDefinitions {
		if def.Key == key {
			return &def
		}
	}
	return nil
}

// GetUserDefinition returns the definition for a user setting key.
func GetUserDefinition(key string) *Definition {
	for _, def := range UserSettingDefinitions {
		if def.Key == key {
			return &def
		}
	}
	return nil
}

func intPtr(v int) *int {
	return &v
}
