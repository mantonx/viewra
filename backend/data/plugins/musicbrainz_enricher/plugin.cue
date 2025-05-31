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
			rate_limit:  float & >=0.1 & <=2.0 | *0.8
			user_agent:  string | *"Viewra/2.0"
		}

		// Artwork settings
		artwork: {
			enabled:    bool | *true
			max_size:   int & >=250 & <=2000 | *1200
			quality:    "front" | "back" | "all" | *"front"
			
			// Cover Art Archive settings
			download_front_cover:  bool | *true
			download_back_cover:   bool | *true
			download_booklet:      bool | *true
			download_medium:       bool | *true
			download_tray:         bool | *true
			download_obi:          bool | *false
			download_spine:        bool | *true
			download_liner:        bool | *true
			download_sticker:      bool | *false
			download_poster:       bool | *false
			
			max_asset_size:        int & >=1048576 & <=52428800 | *10485760 // 1MB to 50MB
			timeout_sec:           int & >=10 & <=120 | *30
			skip_existing:         bool | *true
			retry_failed:          bool | *true
			max_retries:           int & >=1 & <=5 | *3
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