package musicbrainz_enricher

import "strings"

// Plugin metadata
#Plugin: {
	id:          "musicbrainz_enricher"
	name:        "MusicBrainz Metadata Enricher"
	version:     "1.0.0"
	type:        "metadata_scraper"
	description: "Enriches music metadata using MusicBrainz database"
	author:      "Viewra Team"
	
	// Supported media types
	supported_types: ["audio", "album", "artist", "track"]
	
	// Plugin capabilities
	capabilities: [
		"metadata_extraction",
		"artist_enrichment", 
		"album_enrichment",
		"track_enrichment",
		"cover_art_download",
		"release_matching"
	]
}

// Configuration schema with defaults and validation
#Config: {
	// Core Settings
	enabled: bool | *true
	
	// API Configuration
	api: {
		user_agent:        string | *"Viewra/2.0 (https://github.com/mantonx/viewra)"
		request_timeout:   int | *30  // seconds
		max_connections:   int | *5   // concurrent connections
		request_delay:     int | *1000 // milliseconds between requests (MusicBrainz rate limiting)
		enable_cache:      bool | *true
		cache_duration_hours: int | *168 // 1 week
	}
	
	// Feature toggles
	features: {
		enable_artists:       bool | *true
		enable_albums:        bool | *true  
		enable_tracks:        bool | *true
		enable_cover_art:     bool | *true
		enable_relationships: bool | *true  // Artist relationships, collaborations
		enable_genres:        bool | *true
		enable_tags:          bool | *true
	}
	
	// Cover Art Settings
	cover_art: {
		download_covers:      bool | *true
		download_thumbnails:  bool | *false
		preferred_size:       string | *"500"      // 250, 500, 1200, original
		max_size_mb:         int | *5             // Maximum cover art file size
		skip_existing:       bool | *true
		cover_sources: [      // Preferred sources in order
			"Cover Art Archive",
			"Amazon",
			"Discogs", 
			"Last.fm"
		]
	}
	
	// Matching Configuration
	matching: {
		match_threshold:      number | *0.80      // Minimum match confidence
		auto_enrich:         bool | *true
		overwrite_existing:  bool | *false
		fuzzy_matching:      bool | *true
		match_by_isrc:       bool | *true         // International Standard Recording Code
		match_by_barcode:    bool | *true         // Album barcode matching
		match_duration:      bool | *true         // Track duration matching tolerance
		duration_tolerance:  int | *10            // seconds tolerance for duration matching
	}
	
	// Data Enrichment Options
	enrichment: {
		include_aliases:         bool | *true     // Alternative artist/album names
		include_annotations:     bool | *false    // Detailed annotations (can be verbose)
		include_recording_level: bool | *true     // Recording-level metadata vs. track-level
		include_work_info:       bool | *true     // Musical work information (compositions)
		include_label_info:      bool | *true     // Record label information
		include_country_info:    bool | *true     // Release country information
		max_tags_per_item:      int | *10        // Limit tags to avoid data bloat
		max_genres_per_item:    int | *5         // Limit genres per item
	}
	
	// Reliability & Error Handling  
	reliability: {
		max_retries:         int | *3
		initial_retry_delay: int | *2             // seconds
		max_retry_delay:     int | *30            // seconds  
		backoff_multiplier:  number | *2.0       // exponential backoff
		timeout_multiplier:  number | *1.5       // increase timeout on retries
		
		// Circuit breaker settings
		circuit_breaker: {
			failure_threshold:    int | *5        // failures before opening circuit
			success_threshold:    int | *3        // successes to close circuit  
			timeout:             int | *60        // seconds before retry after failure
		}
	}
	
	// Debug & Logging
	debug: {
		enable_debug_logs:   bool | *false
		log_api_requests:    bool | *false
		log_match_details:   bool | *false  
		save_api_responses:  bool | *false       // For debugging API issues
	}
}

// Validation rules
#Config: {
	// Validate timeouts are reasonable
	api: request_timeout: >=5 & <=300
	api: request_delay: >=500 & <=5000
	
	// Validate thresholds  
	matching: match_threshold: >=0.1 & <=1.0
	matching: duration_tolerance: >=1 & <=60
	
	// Validate retry settings
	reliability: max_retries: >=1 & <=10
	reliability: initial_retry_delay: >=1 & <=30
	reliability: max_retry_delay: >=5 & <=300
	
	// Validate limits
	enrichment: max_tags_per_item: >=1 & <=50
	enrichment: max_genres_per_item: >=1 & <=20
	cover_art: max_size_mb: >=1 & <=20
}

// Export the configuration
config: #Config & {
	// Default instance with all defaults applied
	enabled: true
	
	api: {
		user_agent: "Viewra/2.0 (https://github.com/mantonx/viewra)"
		request_timeout: 30
		max_connections: 5
		request_delay: 1000
		enable_cache: true
		cache_duration_hours: 168
	}
	
	features: {
		enable_artists: true
		enable_albums: true
		enable_tracks: true
		enable_cover_art: true
		enable_relationships: true
		enable_genres: true
		enable_tags: true
	}
	
	cover_art: {
		download_covers: true
		download_thumbnails: false
		preferred_size: "500"
		max_size_mb: 5
		skip_existing: true
		cover_sources: [
			"Cover Art Archive",
			"Amazon", 
			"Discogs",
			"Last.fm"
		]
	}
	
	matching: {
		match_threshold: 0.80
		auto_enrich: true
		overwrite_existing: false
		fuzzy_matching: true
		match_by_isrc: true
		match_by_barcode: true
		match_duration: true
		duration_tolerance: 10
	}
	
	enrichment: {
		include_aliases: true
		include_annotations: false
		include_recording_level: true
		include_work_info: true
		include_label_info: true
		include_country_info: true
		max_tags_per_item: 10
		max_genres_per_item: 5
	}
	
	reliability: {
		max_retries: 3
		initial_retry_delay: 2
		max_retry_delay: 30
		backoff_multiplier: 2.0
		timeout_multiplier: 1.5
		
		circuit_breaker: {
			failure_threshold: 5
			success_threshold: 3
			timeout: 60
		}
	}
	
	debug: {
		enable_debug_logs: false
		log_api_requests: false
		log_match_details: false
		save_api_responses: false
	}
} 