package pluginmodule

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// CUEParser handles parsing CUE configuration files and converting them to JSON schemas
type CUEParser struct {
	ctx *cue.Context
}

// NewCUEParser creates a new CUE parser instance
func NewCUEParser() *CUEParser {
	return &CUEParser{
		ctx: cuecontext.New(),
	}
}

// ParsePluginConfiguration extracts the configuration schema from a plugin's CUE file
func (p *CUEParser) ParsePluginConfiguration(pluginDir string) (map[string]interface{}, error) {
	cueFile := filepath.Join(pluginDir, "plugin.cue")

	// Load the CUE file
	buildInstances := load.Instances([]string{cueFile}, nil)
	if len(buildInstances) == 0 {
		return nil, fmt.Errorf("no CUE instances found in %s", cueFile)
	}

	buildInstance := buildInstances[0]
	if buildInstance.Err != nil {
		return nil, fmt.Errorf("error loading CUE file: %v", buildInstance.Err)
	}

	value := p.ctx.BuildInstance(buildInstance)
	if value.Err() != nil {
		return nil, fmt.Errorf("error building CUE instance: %v", value.Err())
	}

	// Look for #Plugin definition
	pluginDef := value.LookupPath(cue.ParsePath("#Plugin"))
	if !pluginDef.Exists() {
		return nil, fmt.Errorf("#Plugin definition not found in CUE file")
	}

	// Extract settings from #Plugin.settings
	settingsValue := pluginDef.LookupPath(cue.ParsePath("settings"))
	if !settingsValue.Exists() {
		return nil, fmt.Errorf("settings not found in #Plugin definition")
	}

	return p.extractConfigurationSchema(settingsValue)
}

// extractConfigurationSchema converts a CUE value to a JSON schema structure
func (p *CUEParser) extractConfigurationSchema(value cue.Value) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	iter, err := value.Fields()
	if err != nil {
		return nil, fmt.Errorf("error iterating fields: %v", err)
	}

	for iter.Next() {
		fieldName := iter.Label()
		fieldValue := iter.Value()

		property, err := p.convertCueValueToProperty(fieldValue, fieldName)
		if err != nil {
			// Log the error but continue processing other fields
			continue
		}

		result[fieldName] = property
	}

	return result, nil
}

// convertCueValueToProperty converts a CUE value to a JSON schema property
func (p *CUEParser) convertCueValueToProperty(value cue.Value, fieldName string) (map[string]interface{}, error) {
	property := map[string]interface{}{
		"title":       p.generateHumanReadableName(fieldName),
		"description": p.extractDescription(value),
		"category":    p.categorizeField(fieldName),
	}

	// Handle different CUE value types
	kind := value.IncompleteKind()

	switch {
	case kind&cue.BoolKind != 0:
		property["type"] = "boolean"
		if defaultVal := p.extractDefaultValue(value); defaultVal != nil {
			property["default"] = defaultVal
		}

	case kind&cue.IntKind != 0:
		property["type"] = "integer"
		if defaultVal := p.extractDefaultValue(value); defaultVal != nil {
			property["default"] = defaultVal
		}

	case kind&cue.FloatKind != 0 || kind&cue.NumberKind != 0:
		property["type"] = "number"
		if defaultVal := p.extractDefaultValue(value); defaultVal != nil {
			property["default"] = defaultVal
		}

	case kind&cue.StringKind != 0:
		property["type"] = "string"
		if defaultVal := p.extractDefaultValue(value); defaultVal != nil {
			property["default"] = defaultVal
		}

	case kind&cue.ListKind != 0:
		property["type"] = "array"
		if defaultVal := p.extractDefaultValue(value); defaultVal != nil {
			property["default"] = defaultVal
		}

	case kind&cue.StructKind != 0:
		// For nested structures, recursively process
		if nestedSchema, err := p.extractConfigurationSchema(value); err == nil && len(nestedSchema) > 0 {
			property["type"] = "object"
			property["properties"] = nestedSchema
		} else {
			property["type"] = "object"
			property["description"] = property["description"].(string) + " (Object configuration)"
		}

	default:
		// Default to string for unknown types
		property["type"] = "string"
		if defaultVal := p.extractDefaultValue(value); defaultVal != nil {
			property["default"] = defaultVal
		}
	}

	return property, nil
}

// extractDefaultValue extracts the default value from a CUE disjunction or concrete value
func (p *CUEParser) extractDefaultValue(value cue.Value) interface{} {
	// Try to get a concrete value first
	if value.IsConcrete() {
		return p.cueValueToInterface(value)
	}

	// Handle disjunctions with defaults (e.g., bool | *true)
	// Look for the marked default in disjunctions
	if op, args := value.Expr(); op == cue.OrOp {
		for _, arg := range args {
			// Check if this argument has a default marker
			if p.isDefaultInDisjunction(arg) {
				return p.cueValueToInterface(arg)
			}
		}
		// If no explicit default found, try the first concrete value
		for _, arg := range args {
			if arg.IsConcrete() {
				return p.cueValueToInterface(arg)
			}
		}
	}

	return nil
}

// isDefaultInDisjunction checks if a value is marked as default in a disjunction
func (p *CUEParser) isDefaultInDisjunction(value cue.Value) bool {
	// In CUE, the default is often marked with *
	// This is a simplified check - CUE's actual default detection is more complex
	if value.IsConcrete() {
		return true // Assume concrete values in disjunctions are defaults
	}
	return false
}

// cueValueToInterface converts a CUE value to a Go interface{}
func (p *CUEParser) cueValueToInterface(value cue.Value) interface{} {
	if !value.IsConcrete() {
		return nil
	}

	var result interface{}
	if err := value.Decode(&result); err == nil {
		return result
	}

	// Fallback: try to extract as different types
	if b, err := value.Bool(); err == nil {
		return b
	}
	if i, err := value.Int64(); err == nil {
		return i
	}
	if f, err := value.Float64(); err == nil {
		return f
	}
	if s, err := value.String(); err == nil {
		return s
	}

	// For complex types, convert to JSON and back
	jsonBytes, _ := json.Marshal(value)
	var jsonResult interface{}
	json.Unmarshal(jsonBytes, &jsonResult)
	return jsonResult
}

// extractDescription extracts documentation/description from CUE comments
func (p *CUEParser) extractDescription(value cue.Value) string {
	// Try to get documentation from the value
	docs := value.Doc()
	if len(docs) > 0 {
		var description strings.Builder
		for i, comment := range docs {
			if i > 0 {
				description.WriteString(" ")
			}
			// Remove comment markers and clean up
			text := strings.TrimPrefix(comment.Text(), "//")
			text = strings.TrimSpace(text)
			description.WriteString(text)
		}
		return description.String()
	}

	return ""
}

// generateHumanReadableName converts field names to human-readable titles
func (p *CUEParser) generateHumanReadableName(fieldName string) string {
	// Convert snake_case or camelCase to Title Case
	words := strings.FieldsFunc(fieldName, func(r rune) bool {
		return r == '_' || r == '-'
	})

	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	result := strings.Join(words, " ")

	// Handle camelCase
	if len(words) == 1 {
		var titleCase strings.Builder
		for i, r := range fieldName {
			if i == 0 {
				upperStr := strings.ToUpper(string(r))
				titleCase.WriteRune(rune(upperStr[0]))
			} else if strings.ToUpper(string(r)) == string(r) && i > 0 {
				titleCase.WriteString(" ")
				titleCase.WriteRune(r)
			} else {
				titleCase.WriteRune(r)
			}
		}
		result = titleCase.String()
	}

	return result
}

// categorizeField assigns a category to a field based on its name
func (p *CUEParser) categorizeField(fieldName string) string {
	lowerField := strings.ToLower(fieldName)

	// Prioritized categories for better organization
	switch {
	case strings.Contains(lowerField, "general") || strings.Contains(lowerField, "enabled") || strings.Contains(lowerField, "priority"):
		return "General"
	case strings.Contains(lowerField, "api") || strings.Contains(lowerField, "key") || strings.Contains(lowerField, "token") || strings.Contains(lowerField, "auth"):
		return "API"
	case strings.Contains(lowerField, "performance") || strings.Contains(lowerField, "timeout") || strings.Contains(lowerField, "concurrent") || strings.Contains(lowerField, "cache"):
		return "Performance"
	case strings.Contains(lowerField, "quality") || strings.Contains(lowerField, "crf") || strings.Contains(lowerField, "bitrate") || strings.Contains(lowerField, "codec"):
		return "Quality"
	case strings.Contains(lowerField, "feature") || strings.Contains(lowerField, "enable") || strings.Contains(lowerField, "support"):
		return "Features"
	case strings.Contains(lowerField, "hardware") || strings.Contains(lowerField, "device") || strings.Contains(lowerField, "ffmpeg"):
		return "Hardware"
	case strings.Contains(lowerField, "filter") || strings.Contains(lowerField, "process") || strings.Contains(lowerField, "effect"):
		return "Filters"
	case strings.Contains(lowerField, "cleanup") || strings.Contains(lowerField, "retention") || strings.Contains(lowerField, "storage"):
		return "Storage"
	case strings.Contains(lowerField, "health") || strings.Contains(lowerField, "monitor") || strings.Contains(lowerField, "check"):
		return "Monitoring"
	case strings.Contains(lowerField, "log") || strings.Contains(lowerField, "debug") || strings.Contains(lowerField, "verbose"):
		return "Logging"
	default:
		return "Advanced"
	}
}
