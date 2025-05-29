#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:            "audiodb_enricher"
	name:          "AudioDB Metadata Enricher"
	version:       "1.0.0"
	description:   "Enriches music metadata using The AudioDB database"
	author:        "Viewra Team"
	website:       "https://github.com/mantonx/viewra"
	repository:    "https://github.com/mantonx/viewra"
	license:       "MIT"
	type:          "metadata_scraper"
	tags: [
		"music",
		"metadata",
		"enrichment",
		"audiodb"
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
		main: "audiodb_enricher"
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

		// API configuration with validation
		api: {
			rate_limit:   float & >=0.1 & <=1.0 | *0.8
			user_agent:   string | *"Viewra AudioDB Enricher/1.0.0"
			timeout_sec:  int & >=5 & <=30 | *10
		}

		// Artwork settings
		artwork: {
			enabled:     bool | *true
			max_size:    int & >=250 & <=2000 | *1200
			quality:     "front" | "back" | "all" | *"front"
			cache_days:  int & >=1 & <=365 | *30
		}

		// Matching configuration
		matching: {
			threshold:           float & >=0.5 & <=1.0 | *0.75
			auto_enrich:         bool | *true
			overwrite_existing:  bool | *false
		}

		// Cache settings
		cache: {
			duration_hours: int & >=1 & <=8760 | *168 // 1 week default
			max_entries:    int & >=100 & <=10000 | *1000
		}
	}
} 