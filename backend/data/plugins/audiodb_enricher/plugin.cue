schema_version: "1.0"

// Plugin identification
id:          "audiodb_enricher"
name:        "AudioDB Metadata Enricher"
version:     "1.0.0"
description: "Enriches music metadata using The AudioDB database"
author:      "Viewra Team"
website:     "https://github.com/mantonx/viewra"
repository:  "https://github.com/mantonx/viewra"
license:     "MIT"
type:        "metadata_scraper"
tags:        ["music", "metadata", "enrichment", "audiodb"]
help:        "This plugin enriches music file metadata by searching The AudioDB database for additional information such as genre, mood, artwork, and more detailed track information."

// Plugin behavior
enabled_by_default: true

// Plugin capabilities
capabilities: {
	metadata_extraction: true
	api_endpoints:      true
	background_tasks:   true
	database_access:    true
	external_services:  true
}

// Entry points
entry_points: {
	main: "./audiodb_enricher"
}

// Permissions
permissions: [
	"network:external",
	"database:read_write",
	"filesystem:read",
]

// Plugin-specific configuration
settings: {
	// AudioDB API configuration
	enabled:              true
	api_key:              "" // Optional API key for enhanced access
	user_agent:           "Viewra AudioDB Enricher/1.0.0"
	
	// Enrichment settings
	enable_artwork:         true
	artwork_max_size:       1200
	artwork_quality:        "front" // front, back, all
	download_album_art:     true    // Download album artwork
	download_artist_images: false   // Download artist images (logos, fanart, etc.)
	prefer_high_quality:    true    // Prefer high quality images when available
	match_threshold:        0.75    // Minimum match score (0.0-1.0)
	auto_enrich:            true    // Auto-enrich during scan
	overwrite_existing:     false   // Overwrite existing metadata
	
	// Asset download controls
	max_asset_size:         10485760 // Max asset file size in bytes (10MB)
	asset_timeout_sec:      30       // Asset download timeout in seconds
	skip_existing_assets:   true     // Skip downloading if asset already exists
	retry_failed_downloads: true     // Retry failed downloads
	max_retries:            3        // Maximum number of retry attempts
	
	// Caching and rate limiting
	cache_duration_hours: 168  // 1 week
	request_delay_ms:     1000 // Delay between API requests
	
	// Search capabilities
	supported_fields: ["title", "artist", "album", "genre"]
	max_search_results: 50
	supports_pagination: false
} 