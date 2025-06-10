#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:            "musicbrainz_enricher"
	name:          "MusicBrainz Metadata Enricher"
	version:       "1.0.0"
	description:   "Enriches music metadata using the MusicBrainz database"
	author:        "Viewra Team"
	website:       "https://github.com/mantonx/viewra"
	repository:    "https://github.com/mantonx/viewra"
	license:       "MIT"
	type:          "metadata_scraper"
	tags: [
		"music",
		"metadata",
		"enrichment",
		"musicbrainz"
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
	}

	// Entry points
	entry_points: {
		main: "musicbrainz_enricher"
	}

	// Permissions
	permissions: [
		"database:read",
		"database:write",
		"network:external",
		"filesystem:read"
	]

	// Plugin-specific settings using CueLang's powerful type system
	settings: {
		// Core settings
		enabled: bool | *true

		// API configuration
		api: {
			rate_limit:  float & >=0.1 & <=2.0 | *0.5
			user_agent:  string | *"Viewra/2.0"
			request_delay_ms: int & >=500 & <=5000 | *1250
			burst_limit: int & >=1 & <=5 | *1
		}

		// Artwork settings
		artwork: {
			enabled:    bool | *true
			max_size:   int & >=250 & <=2000 | *1200
			quality:    "front" | "back" | "all" | *"front"
			
			// Cover Art Archive settings
			download_front_cover:  bool | *true
			download_back_cover:   bool | *false
			download_booklet:      bool | *false
			download_medium:       bool | *false
			download_tray:         bool | *false
			download_obi:          bool | *false
			download_spine:        bool | *false
			download_liner:        bool | *false
			download_sticker:      bool | *false
			download_poster:       bool | *false
			
			max_asset_size:        int & >=102400 & <=52428800 | *10485760 // 100KB to 50MB, default 10MB
			timeout_sec:           int & >=10 & <=300 | *60              // Adjusted max from 120 to 300 seconds
			skip_existing:         bool | *true
			retry_failed:          bool | *true
			max_retries:           int & >=1 & <=10 | *5
			
			// Enhanced retry configuration with size-aware backoff
			initial_retry_delay_sec: int & >=1 & <=10 | *2
			max_retry_delay_sec:     int & >=5 & <=60 | *30
			backoff_multiplier:      float & >=1.0 & <=5.0 | *2.0
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
	}
} 