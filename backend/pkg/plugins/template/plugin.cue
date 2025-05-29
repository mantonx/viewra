#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:            "template_plugin"
	name:          "Template Plugin"
	version:       "1.0.0"
	description:   "Template plugin demonstrating all service interfaces and best practices"
	author:        "Viewra Team"
	website:       "https://github.com/mantonx/viewra"
	repository:    "https://github.com/mantonx/viewra"
	license:       "MIT"
	type:          "metadata_scraper"
	tags: [
		"template",
		"example",
		"metadata",
		"reference"
	]

	// Plugin behavior
	enabled_by_default: false // Template should be disabled by default

	// Plugin capabilities
	capabilities: {
		metadata_extraction: true
		scanner_hooks:       true
		search_service:      true
		api_endpoints:       true
		database_access:     true
		background_tasks:    false
		external_services:   false
		asset_management:    false
	}

	// Entry points
	entry_points: {
		main: "template_plugin"
	}

	// Permissions using modern string array format
	permissions: [
		"database:read",
		"database:write",
		"filesystem:read"
	]

	// Plugin-specific settings using CueLang's type system
	settings: {
		// Core settings
		enabled: bool | *true

		// Plugin configuration
		config: {
			api_key:      string | *""
			user_agent:   string | *"Viewra-Template/1.0"
			max_results:  int & >=1 & <=100 | *10
			cache_hours:  int & >=1 & <=168 | *24
			debug:        bool | *false
		}

		// Example of different setting types for reference
		examples: {
			// String with validation
			text_field: string & =~"^[a-zA-Z0-9_-]+$" | *"default_value"
			
			// Number with range validation
			number_field: int & >=0 & <=1000 | *100
			
			// Boolean
			flag_field: bool | *false
			
			// Enum/choice field
			choice_field: "option1" | "option2" | "option3" | *"option1"
			
			// Array of strings
			list_field: [...string] | *["default1", "default2"]
			
			// Nested object
			object_field: {
				sub_field1: string | *"sub_default"
				sub_field2: int | *42
			}
		}
	}
} 