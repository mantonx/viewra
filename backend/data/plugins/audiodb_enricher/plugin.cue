#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:            "audiodb_enricher"
	name:          "AudioDB Metadata Enricher"
	version:       "2.0.0"
	description:   "Enriches music metadata using The AudioDB API with artwork, artist bios, and genre classification"
	author:        "Viewra Team"
	website:       "https://github.com/mantonx/viewra"
	repository:    "https://github.com/mantonx/viewra"
	license:       "MIT"
	type:          "metadata_scraper"
	tags: [
		"music",
		"metadata",
		"enrichment",
		"audiodb",
		"artwork",
		"api"
	]

	// Plugin behavior
	enabled_by_default: true

	// Plugin capabilities
	capabilities: {
		metadata_extraction: true
		api_endpoints:       true
		background_tasks:    true
		database_access:     true
		external_services:   true
		asset_management:    true
	}

	// Entry points
	entry_points: {
		main: "audiodb_enricher"
	}

	// Permissions
	permissions: [
		"database:read",
		"database:write",
		"network:external",
		"filesystem:read",
		"filesystem:write"
	]

	// Plugin-specific settings using CueLang's powerful type system
	settings: {
		// Core settings
		enabled: bool | *true

		// API configuration with validation
		api: {
			api_key:      string | *""
			user_agent:   string | *"Viewra/2.0"
			timeout_sec:  int & >=5 & <=60 | *30
			delay_ms:     int & >=100 & <=5000 | *100
		}

		// Artwork settings
		artwork: {
			enabled:        bool | *true
			max_size:       int & >=250 & <=2000 | *1200
			quality:        "front" | "back" | "all" | *"front"
			download_album: bool | *true
			download_artist: bool | *true
			prefer_hq:      bool | *true
			max_file_size:  int & >=1048576 & <=52428800 | *10485760 // 1MB to 50MB
		}

		// Matching configuration
		matching: {
			threshold:           float & >=0.5 & <=1.0 | *0.85
			auto_enrich:         bool | *true
			overwrite_existing:  bool | *false
		}

		// Cache settings
		cache: {
			duration_hours: int & >=1 & <=8760 | *168 // 1 week default
		}

		// Asset management
		assets: {
			skip_existing:       bool | *true
			retry_failed:        bool | *true
			max_retries:         int & >=1 & <=5 | *3
		}
	}
} 