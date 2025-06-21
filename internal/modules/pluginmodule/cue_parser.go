package pluginmodule

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// UIMetadata holds UI-specific metadata extracted from CUE @ui attributes
type UIMetadata struct {
	importance  int
	level       string
	category    string
	description string
	min         interface{}
	max         interface{}
	enum        []string
}

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
	// Extract UI metadata from @ui attribute
	uiMeta := p.extractUIMetadata(value)

	// Use extracted metadata or fall back to inferred values
	category := uiMeta.category
	if category == "" {
		category = p.categorizeField(fieldName)
	}

	importance := uiMeta.importance
	if importance == 0 {
		importance = p.calculateImportance(fieldName, category)
	}

	// Determine if setting is basic based on field name and section
	isBasic := p.isEssentialSetting(fieldName, category)
	if uiMeta.level == "basic" {
		isBasic = true
	} else if uiMeta.level == "advanced" {
		isBasic = false
	}

	property := map[string]interface{}{
		"title":         p.generateHumanReadableName(fieldName),
		"description":   uiMeta.description,
		"category":      category,
		"importance":    importance,
		"is_basic":      isBasic,
		"user_friendly": uiMeta.level != "advanced",
	}

	// Add UI constraints if present
	if uiMeta.min != nil {
		property["minimum"] = uiMeta.min
	}
	if uiMeta.max != nil {
		property["maximum"] = uiMeta.max
	}
	if len(uiMeta.enum) > 0 {
		property["enum"] = uiMeta.enum
	}

	// Check if this value has explicit type/default structure
	if typeVal := value.LookupPath(cue.ParsePath("type")); typeVal.Exists() {
		// Handle explicit type definition
		if typeStr, err := typeVal.String(); err == nil {
			property["type"] = typeStr
		}

		// Handle explicit default
		if defaultVal := value.LookupPath(cue.ParsePath("default")); defaultVal.Exists() {
			if defaultInterface := p.cueValueToInterface(defaultVal); defaultInterface != nil {
				property["default"] = defaultInterface
			}
		}
		return property, nil
	} else {
		// Handle different CUE value types (traditional approach)
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
		// First pass: look for explicit defaults marked with *
		for _, arg := range args {
			if p.isDefaultInDisjunction(arg) {
				if concrete, ok := arg.Default(); ok && concrete.IsConcrete() {
					return p.cueValueToInterface(concrete)
				}
			}
		}

		// Second pass: try to get default from the value itself
		if def, ok := value.Default(); ok && def.IsConcrete() {
			return p.cueValueToInterface(def)
		}

		// Third pass: if no explicit default found, try the first concrete value
		for _, arg := range args {
			if arg.IsConcrete() {
				return p.cueValueToInterface(arg)
			}
		}
	}

	// Try to get default from value directly (for non-disjunctions)
	if def, ok := value.Default(); ok && def.IsConcrete() {
		return p.cueValueToInterface(def)
	}

	return nil
}

// isDefaultInDisjunction checks if a value is marked as default in a disjunction
func (p *CUEParser) isDefaultInDisjunction(value cue.Value) bool {
	// Check if this value has a default marker
	if def, ok := value.Default(); ok {
		return def.IsConcrete()
	}

	// Alternative approach: check the kind and see if it's a default
	// In CUE disjunctions, defaults are often the concrete values
	return value.IsConcrete()
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

// calculateImportance assigns an importance score (1-10) to fields for UI prioritization
func (p *CUEParser) calculateImportance(fieldName, category string) int {
	lowerField := strings.ToLower(fieldName)

	// Critical settings that users need most
	switch {
	case strings.Contains(lowerField, "enabled"):
		return 10
	case strings.Contains(lowerField, "quality") && !strings.Contains(lowerField, "profile"):
		return 9
	case strings.Contains(lowerField, "preset") || strings.Contains(lowerField, "crf"):
		return 8
	case category == "General":
		return 8
	case strings.Contains(lowerField, "bitrate") || strings.Contains(lowerField, "resolution"):
		return 7
	case category == "Performance" && (strings.Contains(lowerField, "concurrent") || strings.Contains(lowerField, "timeout")):
		return 7
	case category == "API" && (strings.Contains(lowerField, "key") || strings.Contains(lowerField, "token")):
		return 6
	case category == "Quality" || category == "Performance":
		return 5
	case category == "Features":
		return 4
	case category == "Hardware":
		return 3
	case category == "Logging" || category == "Monitoring":
		return 2
	default:
		return 1
	}
}

// isEssentialSetting determines if a setting is essential for basic users
func (p *CUEParser) isEssentialSetting(fieldName, category string) bool {
	lowerField := strings.ToLower(fieldName)

	// Essential individual settings that should always be in basic mode
	essentialFields := map[string]bool{
		"enabled":               true,
		"preset":                true,
		"crf_h264":              true,
		"quality_speed_balance": true,
		"max_concurrent_jobs":   true,
		"timeout_seconds":       true,
		"type":                  true, // hardware type
		"fallback_to_software":  true,
		"priority":              true,
		"path":                  true, // ffmpeg path
	}

	// Check exact field name matches
	if essentialFields[lowerField] {
		return true
	}

	// Essential top-level sections that contain basic settings
	essentialSections := map[string]bool{
		"general":     true, // contains enabled, priority
		"ffmpeg":      true, // contains path, preset
		"quality":     true, // contains crf settings
		"performance": true, // contains max_concurrent_jobs, timeout
		"hardware":    true, // contains hardware acceleration settings
	}

	// If this is a top-level section, check if it's essential
	if essentialSections[lowerField] {
		return true
	}

	// For properties within essential categories, check specific ones
	switch category {
	case "General":
		// All general settings are basic (enabled, priority)
		return true
	case "Hardware":
		// Hardware acceleration settings are important for users
		if strings.Contains(lowerField, "enabled") ||
			strings.Contains(lowerField, "type") ||
			strings.Contains(lowerField, "fallback") {
			return true
		}
	case "Performance":
		// Key performance settings
		if strings.Contains(lowerField, "concurrent") ||
			strings.Contains(lowerField, "timeout") ||
			strings.Contains(lowerField, "max_") {
			return true
		}
	case "Quality":
		// Essential quality settings
		if strings.Contains(lowerField, "crf") ||
			strings.Contains(lowerField, "preset") ||
			strings.Contains(lowerField, "quality_speed") ||
			strings.Contains(lowerField, "balance") {
			return true
		}
	}

	// Also check for commonly needed settings by keyword
	basicKeywords := []string{"enabled", "preset", "quality", "bitrate", "priority", "path"}
	for _, keyword := range basicKeywords {
		if strings.Contains(lowerField, keyword) {
			return true
		}
	}

	return false
}

// isUserFriendly determines if a setting is user-friendly (vs technical)
func (p *CUEParser) isUserFriendly(fieldName string) bool {
	lowerField := strings.ToLower(fieldName)

	// Technical/expert settings that are less user-friendly
	technicalKeywords := []string{
		"codec", "profile", "level", "tune", "refs", "bframes", "ctu_size",
		"tile_", "cpu_used", "row_mt", "threads", "buffer_size", "ladder",
		"multiplier", "keywords", "detection", "passthrough", "fallback",
	}

	for _, keyword := range technicalKeywords {
		if strings.Contains(lowerField, keyword) {
			return false
		}
	}

	return true
}

// extractUIMetadata extracts UI metadata from CUE @ui attributes
func (p *CUEParser) extractUIMetadata(value cue.Value) UIMetadata {
	meta := UIMetadata{}

	// Look for @ui attribute in the value
	attrs := value.Attributes(cue.ValueAttr)
	for _, attr := range attrs {
		if attr.Name() == "ui" {
			// Parse the attribute content which is in format: importance=8,level="basic",category="Quality"
			content := attr.Contents()
			p.parseUIAttributeContent(content, &meta)
			break
		}
	}

	// Extract description from comments
	meta.description = p.extractDescription(value)

	return meta
}

// parseUIAttributeContent parses the content of a @ui attribute
func (p *CUEParser) parseUIAttributeContent(content string, meta *UIMetadata) {
	// Remove parentheses if present
	content = strings.Trim(content, "()")

	// Split by comma and parse each key=value pair
	pairs := strings.Split(content, ",")
	for _, pair := range pairs {
		// Split by = and handle quoted values
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		switch key {
		case "importance":
			if importance, err := strconv.Atoi(value); err == nil {
				meta.importance = importance
			}
		case "level":
			meta.level = value
		case "category":
			meta.category = value
		case "description":
			meta.description = value
		case "min":
			if min, err := strconv.ParseFloat(value, 64); err == nil {
				meta.min = min
			}
		case "max":
			if max, err := strconv.ParseFloat(value, 64); err == nil {
				meta.max = max
			}
		case "enum":
			// Handle enum as comma-separated values in brackets
			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				enumContent := strings.Trim(value, "[]")
				enumValues := strings.Split(enumContent, ",")
				for _, enumValue := range enumValues {
					meta.enum = append(meta.enum, strings.Trim(strings.TrimSpace(enumValue), `"'`))
				}
			}
		}
	}
}
