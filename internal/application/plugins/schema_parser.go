package plugins

import (
	"encoding/json"
)

// SchemaParser extracts metadata from JSON Schema definitions.
type SchemaParser struct{}

// NewSchemaParser creates a new SchemaParser.
func NewSchemaParser() *SchemaParser {
	return &SchemaParser{}
}

// GetSensitiveFields returns a set of field names that are marked as sensitive.
// Sensitive fields are identified by having format: "password" in their schema.
func (p *SchemaParser) GetSensitiveFields(schemaJSON json.RawMessage) map[string]bool {
	sensitiveFields := make(map[string]bool)

	if len(schemaJSON) == 0 {
		return sensitiveFields
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return sensitiveFields
	}

	// Look for properties in the schema
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return sensitiveFields
	}

	// Check each property for format: "password"
	for fieldName, fieldDef := range properties {
		fieldMap, ok := fieldDef.(map[string]any)
		if !ok {
			continue
		}

		// Check for format: "password" which indicates a sensitive field
		if format, ok := fieldMap["format"].(string); ok && format == "password" {
			sensitiveFields[fieldName] = true
		}

		// Also check for x-sensitive: true as an alternative marker
		if sensitive, ok := fieldMap["x-sensitive"].(bool); ok && sensitive {
			sensitiveFields[fieldName] = true
		}
	}

	return sensitiveFields
}

// IsSensitiveField checks if a specific field is sensitive in the given schema.
func (p *SchemaParser) IsSensitiveField(schemaJSON json.RawMessage, fieldName string) bool {
	sensitiveFields := p.GetSensitiveFields(schemaJSON)
	return sensitiveFields[fieldName]
}
