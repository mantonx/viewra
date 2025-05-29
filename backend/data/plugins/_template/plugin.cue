#Plugin: {
	id:                   "template_plugin"     // TODO: Change this to your plugin ID  
	name:                 "Template Plugin"     // TODO: Change this to your plugin name
	version:              "1.0.0"
	description:          "A template plugin for Viewra"  // TODO: Change this
	author:               "Viewra Team"
	type:                 "metadata_scraper"    // TODO: Change to your plugin type
	enabled_by_default:   false                 // TODO: Set to true if should be enabled by default
	
	entry_points: {
		main: "template_plugin"                 // TODO: Change this to match your binary name
	}
	
	dependencies: [
		"gorm.io/gorm",
		"gorm.io/driver/sqlite",
		"github.com/hashicorp/go-hclog",
		"github.com/hashicorp/go-plugin"
		// TODO: Add your plugin-specific dependencies here
	]
	
	permissions: [
		"database",
		"network"
		// TODO: Add required permissions here
	]
	
	configuration: {
		enabled: {
			type:        "boolean"
			default:     true
			description: "Enable/disable the plugin"
		}
		api_key: {
			type:        "string"
			required:    false
			description: "API key for external service (if needed)"
			sensitive:   true
		}
		// TODO: Add your plugin-specific configuration schema here
	}
} 