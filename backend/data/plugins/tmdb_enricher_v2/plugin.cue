#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:            "tmdb_enricher_v2"
	name:          "TMDb Metadata Enricher v2"
	version:       "2.0.0"
	description:   "Enriches TV shows and movie metadata using The Movie Database (TMDb) API with modern architecture and reliability features"
	author:        "Viewra Team"
	website:       "https://github.com/mantonx/viewra"
	repository:    "https://github.com/mantonx/viewra"
	license:       "MIT"
	type:          "metadata_scraper"
	tags: [
		"tv",
		"movies",
		"metadata",
		"enrichment",
		"tmdb",
		"external-api"
	]

	// Plugin behavior
	enabled_by_default: true

	// Plugin capabilities
	capabilities: {
		metadata_extraction: true
		scanner_hooks:       true
		search_service:      true
		api_endpoints:       false
		database_access:     true
		background_tasks:    true
		external_services:   true
		asset_management:    true
	}

	// Entry points
	entry_points: {
		main: "tmdb_enricher_v2"
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
		// Core API settings
		api: {
			key:          string | *"" @tag(sensitive)     // TMDb API key
			rate_limit:   float64 | *0.6                  // Requests per second (conservative)
			user_agent:   string | *"Viewra/2.0"          // User agent for API requests
			language:     string | *"en-US"               // Preferred language
			region:       string | *"US"                  // Preferred region
			timeout_sec:  int | *60                       // Request timeout in seconds
			delay_ms:     int | *1200                     // Delay between requests in milliseconds
		}

		// Feature toggles
		features: {
			enable_movies:      bool | *true   // Enable movie enrichment
			enable_tv_shows:    bool | *true   // Enable TV show enrichment
			enable_episodes:    bool | *true   // Enable episode-level enrichment
			enable_artwork:     bool | *true   // Enable artwork downloads
			auto_enrich:        bool | *true   // Automatically enrich during scanning
			overwrite_existing: bool | *false  // Overwrite existing metadata
		}

		// Artwork download settings
		artwork: {
			download_posters:        bool | *true   // Download movie/show posters
			download_backdrops:      bool | *false  // Download backdrop images
			download_logos:          bool | *false  // Download logo images
			download_stills:         bool | *false  // Download episode stills
			download_season_posters: bool | *true   // Download season posters
			download_episode_stills: bool | *true   // Download episode stills

			// Image sizes (TMDb size options)
			poster_size:   string | *"w500"  // w92, w154, w185, w342, w500, w780, original
			backdrop_size: string | *"w780"  // w300, w780, w1280, original
			logo_size:     string | *"w300"  // w45, w92, w154, w185, w300, w500, original
			still_size:    string | *"w300"  // w92, w185, w300, original

			// Download limits
			max_asset_size_mb:    int | *10    // Maximum asset size in MB
			asset_timeout_sec:    int | *60    // Asset download timeout
			skip_existing_assets: bool | *true // Skip downloading existing assets
		}

		// Matching and quality settings
		matching: {
			match_threshold: float64 | *0.85 // Minimum similarity score for matches
			match_year:      bool | *true    // Use release year for matching
			year_tolerance:  int | *2        // Allow +/- years difference
		}

		// Cache settings
		cache: {
			duration_hours:   int | *168 // Cache duration (1 week)
			cleanup_interval: int | *24  // Cleanup interval in hours
		}

		// Retry and reliability settings
		reliability: {
			max_retries:            int | *5        // Maximum retry attempts
			initial_delay_sec:      int | *2        // Initial retry delay
			max_delay_sec:          int | *30       // Maximum retry delay
			backoff_multiplier:     float64 | *2.0  // Exponential backoff multiplier
			retry_failed_downloads: bool | *true    // Retry failed artwork downloads
		}

		// Debug and monitoring
		debug: {
			enabled:          bool | *false // Enable debug logging
			log_api_requests: bool | *false // Log all API requests
			log_cache_hits:   bool | *false // Log cache hit/miss
		}
	}
}
