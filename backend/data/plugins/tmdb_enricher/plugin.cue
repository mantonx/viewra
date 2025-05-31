#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:            "tmdb_enricher"
	name:          "TMDb Metadata Enricher"
	version:       "1.0.0"
	description:   "Enriches TV shows and movie metadata using The Movie Database (TMDb)"
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
		"tmdb"
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
		main: "tmdb_enricher"
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
			api_key:     string | *"eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI1YTU2ODc0YjRmMzU4YjIzZDhkM2YzZmI5ZDc4NDNiOSIsIm5iZiI6MTc0ODYzOTc1Ny40MDEsInN1YiI6IjY4M2EyMDBkNzA5OGI4MzMzNThmZThmOSIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.OXT68T0EtU-WXhcP7nwyWjMePuEuCpfWtDlvdntWKw8"
			rate_limit:  float & >=0.1 & <=2.0 | *1.0
			user_agent:  string | *"Viewra/2.0"
			language:    string | *"en-US"
			region:      string | *"US"
		}

		// Content type settings
		content: {
			enable_movies:   bool | *true
			enable_tv_shows: bool | *true
			enable_episodes: bool | *true
		}

		// Artwork settings
		artwork: {
			enabled:              bool | *true
			download_posters:     bool | *true
			download_backdrops:   bool | *true
			download_logos:       bool | *true
			download_stills:      bool | *false
			poster_size:          "w92" | "w154" | "w185" | "w342" | "w500" | "w780" | "original" | *"w500"
			backdrop_size:        "w300" | "w780" | "w1280" | "original" | *"w1280"
			logo_size:            "w45" | "w92" | "w154" | "w185" | "w300" | "w500" | "original" | *"w500"
			still_size:           "w92" | "w185" | "w300" | "original" | *"w300"
			max_asset_size:       int & >=1048576 & <=52428800 | *10485760 // 1MB to 50MB
			timeout_sec:          int & >=10 & <=120 | *30
			skip_existing:        bool | *true
			retry_failed:         bool | *true
			max_retries:          int & >=1 & <=5 | *3
		}

		// Matching configuration
		matching: {
			threshold:           float & >=0.5 & <=1.0 | *0.85
			auto_enrich:         bool | *true
			overwrite_existing:  bool | *false
			match_year:          bool | *true
			year_tolerance:      int & >=0 & <=5 | *2
		}

		// Cache settings
		cache: {
			duration_hours: int & >=1 & <=8760 | *168 // 1 week default
		}

		// Episode settings
		episodes: {
			enrich_episodes:      bool | *true
			match_episode_names:  bool | *true
			match_season_episode: bool | *true
		}
	}
} 