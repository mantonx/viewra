// Schema builder for ViewRA plugin settings.
//
// This package provides a fluent API for building JSON Schema definitions
// with ViewRA-specific extensions. Plugin authors use this to define their
// settings UI without writing raw JSON.
//
// # Quick Start
//
// Create a schema.go file in your plugin's internal package:
//
//	package internal
//
//	import "github.com/mantonx/viewra/pkg/plugin/sdk"
//
//	func SettingsSchema() *sdk.Schema {
//	    return sdk.NewSchema("My Plugin Settings").
//	        Meta(sdk.PluginMeta{
//	            DisplayName: "My Plugin",
//	            Description: "Does something useful",
//	            Icon:        "star",
//	        }).
//	        Property("api_key", sdk.String().
//	            Title("API Key").
//	            Description("Your API key").
//	            Format("password").
//	            Required()).
//	        Action(sdk.TestAction("test-connection", "/health"))
//	}
//
// Then in your plugin.go, use it in GetSettingsSchema:
//
//	func (p *Plugin) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
//	    return SettingsSchema().BuildSettingsSchema()
//	}
//
// # ViewRA Schema Extensions
//
// ViewRA extends JSON Schema with custom fields:
//
//   - x-viewra-meta: Plugin metadata for the UI (display name, icon, etc.)
//   - x-viewra-actions: Interactive UI elements (test buttons, lists, forms)
//   - x-viewra-sections: Group properties/actions by capability for filtering
//
// # Examples
//
// Simple API provider (like OpenAI):
//
//	sdk.NewSchema("OpenAI Settings").
//	    Meta(sdk.PluginMeta{
//	        DisplayName: "OpenAI",
//	        Description: "OpenAI API for embeddings and chat",
//	        Tip:         "Requires API key. Usage is billed per token.",
//	        Icon:        "cloud",
//	    }).
//	    Property("api_key", sdk.String().
//	        Title("API Key").
//	        Format("password").
//	        Required()).
//	    Property("model", sdk.String().
//	        Title("Model").
//	        Default("gpt-4o-mini")).
//	    Action(sdk.TestAction("test-connection", "/health"))
//
// Provider with model management (like Ollama):
//
//	sdk.NewSchema("Ollama Settings").
//	    Meta(sdk.PluginMeta{
//	        DisplayName: "Ollama",
//	        Description: "Local AI inference",
//	        IsLocal:     true,
//	        Icon:        "hard-drive",
//	    }).
//	    Property("base_url", sdk.String().
//	        Title("Server URL").
//	        Default("http://localhost:11434")).
//	    Property("model", sdk.String().
//	        Title("Model").
//	        EnumStrings("llama3", "mistral")). // Dynamic from installed models
//	    Section(sdk.NewSection("connection").
//	        Properties("base_url").
//	        Actions("test-connection").
//	        Capabilities("embedding", "chat")).
//	    Section(sdk.NewSection("models").
//	        Properties("model").
//	        Actions("model-list").
//	        Capabilities("chat")).
//	    Action(sdk.TestAction("test-connection", "/health")).
//	    Action(sdk.ListAction("model-list", "/models").
//	        Title("Available Models").
//	        TabTitle("Models").
//	        Display(sdk.NewListDisplay("name").
//	            SecondaryField("description")).
//	        ItemAction(sdk.NewDeleteAction("delete", "/models/:id").
//	            Confirm("Delete Model", "Are you sure?")))
//
// # Available Icons
//
// Common icons: "cloud", "hard-drive", "brain", "compass", "star", "settings",
// "database", "film", "music", "tv", "search", "sparkles"
//
// # Property Formats
//
// Special formats for string properties:
//   - "password": Renders as a password field (masked input)
//   - "uri": URL validation
//   - "email": Email validation
package sdk

import (
	"encoding/json"
	"fmt"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// ============================================================================
// Schema Builder
// ============================================================================

// Schema builds a JSON Schema for plugin settings.
// Use NewSchema to create an instance and chain methods to configure it.
type Schema struct {
	title      string
	meta       *PluginMeta
	properties map[string]*Property
	propOrder  []string // Maintain insertion order
	required   []string
	actions    []Action
	sections   []Section
}

// PluginMeta contains metadata for the plugin UI display.
// This appears in the settings page header when the plugin is selected.
type PluginMeta struct {
	// DisplayName is the human-readable name shown in the UI.
	// Example: "OpenAI", "Ollama", "TMDB"
	DisplayName string `json:"displayName"`

	// ProviderName is the name shown in provider selection dropdowns.
	// If not set, DisplayName is used. Use this when the plugin name
	// differs from how it should appear as a provider option.
	// Example: DisplayName="AI Features", ProviderName="Ollama"
	ProviderName string `json:"providerName,omitempty"`

	// Description is a short description of what the plugin does.
	// Example: "OpenAI API for embeddings and chat"
	Description string `json:"description"`

	// Tip is optional help text shown as a tooltip.
	// Example: "Requires API key. Usage is billed per token."
	Tip string `json:"tip,omitempty"`

	// IsLocal indicates if this runs locally (no external API calls).
	// Local plugins show a "Local" badge in the UI.
	IsLocal bool `json:"isLocal,omitempty"`

	// Icon is the icon name to display. Common options:
	// "cloud", "hard-drive", "brain", "compass", "star", "settings"
	Icon string `json:"icon,omitempty"`
}

// NewSchema creates a new schema builder with the given title.
// The title appears in the schema but is typically stripped by the UI
// since the plugin name is shown in the card header.
//
// Example:
//
//	schema := sdk.NewSchema("My Plugin Settings")
func NewSchema(title string) *Schema {
	return &Schema{
		title:      title,
		properties: make(map[string]*Property),
	}
}

// Meta sets the x-viewra-meta extension for the schema.
// This metadata controls how the plugin appears in the UI.
//
// Example:
//
//	schema.Meta(sdk.PluginMeta{
//	    DisplayName: "My Plugin",
//	    Description: "Does useful things",
//	    Icon:        "star",
//	})
func (s *Schema) Meta(meta PluginMeta) *Schema {
	s.meta = &meta
	return s
}

// Property adds a property (setting field) to the schema.
// Properties are rendered as form fields in the settings UI.
// Use sdk.String(), sdk.Number(), sdk.Boolean(), or sdk.Integer()
// to create the property builder.
//
// Example:
//
//	schema.Property("api_key", sdk.String().
//	    Title("API Key").
//	    Format("password").
//	    Required())
func (s *Schema) Property(name string, prop *Property) *Schema {
	s.properties[name] = prop
	s.propOrder = append(s.propOrder, name)
	if prop.required {
		s.required = append(s.required, name)
	}
	return s
}

// Action adds an action to the x-viewra-actions extension.
// Actions create interactive UI elements like buttons and lists.
//
// Action types:
//   - TestAction: A "Test Connection" button
//   - ListAction: A list of items with actions (e.g., model list)
//
// Example:
//
//	schema.Action(sdk.TestAction("test-connection", "/health"))
func (s *Schema) Action(action Action) *Schema {
	s.actions = append(s.actions, action)
	return s
}

// Section adds a section to the x-viewra-sections extension.
// Sections group properties and actions by capability, allowing
// the UI to show only relevant settings based on context.
//
// This is useful for plugins that support multiple capabilities
// (e.g., both embedding and chat) but have different settings for each.
//
// Example:
//
//	schema.Section(sdk.NewSection("embedding").
//	    Properties("embedding_model").
//	    Actions("embedding-models").
//	    Capabilities("embedding"))
func (s *Schema) Section(section *Section) *Schema {
	s.sections = append(s.sections, *section)
	return s
}

// Build serializes the schema to JSON bytes.
// Most plugins should use BuildSettingsSchema instead.
func (s *Schema) Build() ([]byte, error) {
	schema := map[string]any{
		"type":  "object",
		"title": s.title,
	}

	if s.meta != nil {
		schema["x-viewra-meta"] = s.meta
	}

	if len(s.required) > 0 {
		schema["required"] = s.required
	}

	// Build properties in insertion order
	props := make(map[string]any)
	for _, name := range s.propOrder {
		props[name] = s.properties[name].build()
	}
	schema["properties"] = props

	// Output property order for frontend to use
	// JSON Schema doesn't guarantee object key order, so we need this extension
	if len(s.propOrder) > 0 {
		schema["x-viewra-order"] = s.propOrder
	}

	if len(s.sections) > 0 {
		sections := make([]any, len(s.sections))
		for i, sec := range s.sections {
			sections[i] = sec.build()
		}
		schema["x-viewra-sections"] = sections
	}

	if len(s.actions) > 0 {
		actions := make([]any, len(s.actions))
		for i, action := range s.actions {
			actions[i] = action.build()
		}
		schema["x-viewra-actions"] = actions
	}

	return json.Marshal(schema)
}

// BuildSettingsSchema builds and returns a SettingsSchema proto message.
// Deprecated: Use Build() instead for SDK-only plugins.
//
// Example:
//
//	func (p *Plugin) GetSettingsSchema() ([]byte, error) {
//	    return SettingsSchema().Build()
//	}
func (s *Schema) BuildSettingsSchema() (*pluginv1.SettingsSchema, error) {
	data, err := s.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build schema: %w", err)
	}
	return &pluginv1.SettingsSchema{JsonSchema: data}, nil
}

// ============================================================================
// Property Builder
// ============================================================================

// PropertyType represents JSON Schema property types.
type PropertyType string

const (
	TypeString  PropertyType = "string"
	TypeNumber  PropertyType = "number"
	TypeInteger PropertyType = "integer"
	TypeBoolean PropertyType = "boolean"
	TypeArray   PropertyType = "array"
	TypeObject  PropertyType = "object"
)

// Property represents a JSON Schema property (a settings field).
// Use String(), Number(), Boolean(), or Integer() to create one.
type Property struct {
	propType    PropertyType
	title       string
	description string
	format      string
	defaultVal  any
	enum        []any
	required    bool
	minimum     *float64
	maximum     *float64

	// Conditional visibility
	dependsOn *DependsOnConfig

	// Plugin reference fields (for PluginRef type)
	pluginRef *PluginRefConfig
}

// DependsOnConfig configures conditional visibility for a property or section.
type DependsOnConfig struct {
	// Field is the property name this depends on
	Field string `json:"field"`
	// Value is the value the field must have for this to be visible
	Value any `json:"value"`
}

// PluginRefConfig configures a plugin reference property.
// Used to embed another plugin's settings inline within this plugin's form.
type PluginRefConfig struct {
	// Capability is the capability to filter plugins by (e.g., "embedding", "chat").
	// Only plugins providing this capability appear in the dropdown.
	Capability string

	// SettingsKey is the key where the referenced plugin's settings are stored.
	// If empty, defaults to "{capability}_provider_settings".
	SettingsKey string
}

// String creates a string property builder.
// String properties render as text inputs.
//
// Example:
//
//	sdk.String().Title("API Key").Format("password").Required()
func String() *Property {
	return &Property{propType: TypeString}
}

// Number creates a number property builder.
// Number properties render as numeric inputs (allows decimals).
//
// Example:
//
//	sdk.Number().Title("Temperature").Default(0.7)
func Number() *Property {
	return &Property{propType: TypeNumber}
}

// Integer creates an integer property builder.
// Integer properties render as numeric inputs (whole numbers only).
//
// Example:
//
//	sdk.Integer().Title("Max Tokens").Default(1000)
func Integer() *Property {
	return &Property{propType: TypeInteger}
}

// Boolean creates a boolean property builder.
// Boolean properties render as toggle switches.
//
// Example:
//
//	sdk.Boolean().Title("Enable Feature").Default(true)
func Boolean() *Property {
	return &Property{propType: TypeBoolean}
}

// PluginRef creates a plugin reference property builder.
// This renders as a dropdown to select a provider plugin, with the selected
// plugin's settings shown inline below the dropdown.
//
// Use this in configuration plugins (like ai-local) to let users select
// and configure provider plugins for specific capabilities.
//
// The property value is the selected plugin ID (string).
// The referenced plugin's settings are stored separately under the settingsKey.
//
// Example:
//
//	// In ai-local schema:
//	sdk.PluginRef("embedding").
//	    Title("Embedding Provider").
//	    Description("Select which plugin provides embeddings")
//
// This generates a UI with:
//   - A dropdown listing all plugins providing the "embedding" capability
//   - When a plugin is selected, its settings form appears inline
//   - The selected plugin's settings are saved under "embedding_provider_settings"
func PluginRef(capability string) *Property {
	return &Property{
		propType: TypeString, // The value is the selected plugin ID
		pluginRef: &PluginRefConfig{
			Capability:  capability,
			SettingsKey: capability + "_provider_settings",
		},
	}
}

// SettingsKey sets the key where the referenced plugin's settings are stored.
// Default is "{capability}_provider_settings".
//
// Example:
//
//	sdk.PluginRef("embedding").SettingsKey("custom_embedding_config")
func (p *Property) SettingsKey(key string) *Property {
	if p.pluginRef != nil {
		p.pluginRef.SettingsKey = key
	}
	return p
}

// Title sets the property title shown as the field label.
//
// Example:
//
//	sdk.String().Title("API Key")
func (p *Property) Title(title string) *Property {
	p.title = title
	return p
}

// Description sets the property description shown as help text.
//
// Example:
//
//	sdk.String().Title("API Key").Description("Get your key from the dashboard")
func (p *Property) Description(desc string) *Property {
	p.description = desc
	return p
}

// Format sets the property format for special rendering.
// Common formats:
//   - "password": Renders as masked input
//   - "uri": Validates as URL
//   - "email": Validates as email
//
// Example:
//
//	sdk.String().Title("API Key").Format("password")
func (p *Property) Format(format string) *Property {
	p.format = format
	return p
}

// Default sets the default value for the property.
// This value is used when no value has been saved.
//
// Example:
//
//	sdk.String().Title("Model").Default("gpt-4o-mini")
func (p *Property) Default(val any) *Property {
	p.defaultVal = val
	return p
}

// Enum sets the allowed values for the property.
// When set, the field renders as a dropdown select.
//
// Example:
//
//	sdk.String().Title("Model").Enum("gpt-4", "gpt-3.5-turbo")
func (p *Property) Enum(values ...any) *Property {
	p.enum = values
	return p
}

// EnumStrings sets the allowed string values for the property.
// Convenience method for string enums.
//
// Example:
//
//	sdk.String().Title("Model").EnumStrings("gpt-4", "gpt-3.5-turbo")
func (p *Property) EnumStrings(values ...string) *Property {
	p.enum = make([]any, len(values))
	for i, v := range values {
		p.enum[i] = v
	}
	return p
}

// Required marks this property as required.
// Required fields must have a value before saving.
//
// Example:
//
//	sdk.String().Title("API Key").Required()
func (p *Property) Required() *Property {
	p.required = true
	return p
}

// Min sets the minimum value for number/integer properties.
//
// Example:
//
//	sdk.Integer().Title("Batch Size").Min(10).Max(200)
func (p *Property) Min(val float64) *Property {
	p.minimum = &val
	return p
}

// Max sets the maximum value for number/integer properties.
//
// Example:
//
//	sdk.Number().Title("Similarity").Min(0.0).Max(1.0)
func (p *Property) Max(val float64) *Property {
	p.maximum = &val
	return p
}

// DependsOn sets conditional visibility - this property only shows when
// the specified field has the specified value.
//
// Example:
//
//	sdk.String().Title("API Key").DependsOn("enabled", true)
func (p *Property) DependsOn(field string, value any) *Property {
	p.dependsOn = &DependsOnConfig{Field: field, Value: value}
	return p
}

func (p *Property) build() map[string]any {
	prop := map[string]any{
		"type": string(p.propType),
	}
	if p.title != "" {
		prop["title"] = p.title
	}
	if p.description != "" {
		prop["description"] = p.description
	}
	if p.format != "" {
		prop["format"] = p.format
	}
	if p.defaultVal != nil {
		prop["default"] = p.defaultVal
	}
	if len(p.enum) > 0 {
		prop["enum"] = p.enum
	}
	if p.minimum != nil {
		prop["minimum"] = *p.minimum
	}
	if p.maximum != nil {
		prop["maximum"] = *p.maximum
	}
	if p.pluginRef != nil {
		prop["x-viewra-plugin-ref"] = map[string]any{
			"capability":  p.pluginRef.Capability,
			"settingsKey": p.pluginRef.SettingsKey,
		}
	}
	if p.dependsOn != nil {
		prop["x-viewra-depends-on"] = map[string]any{
			"field": p.dependsOn.Field,
			"value": p.dependsOn.Value,
		}
	}
	return prop
}

// ============================================================================
// Action Builders
// ============================================================================

// Action represents an x-viewra-actions entry.
// Actions are interactive UI elements that call plugin endpoints.
type Action interface {
	build() map[string]any
}

// TestActionDef defines a test/health-check action.
// This renders as a "Test Connection" button that calls your health endpoint.
type TestActionDef struct {
	id       string
	title    string
	endpoint string
}

// TestAction creates a test connection action.
// The endpoint should return a JSON response with:
//
//	{"success": true, "message": "Connected successfully"}
//
// or on failure:
//
//	{"success": false, "error": "Connection failed"}
//
// Example:
//
//	sdk.TestAction("test-connection", "/health")
func TestAction(id, endpoint string) *TestActionDef {
	return &TestActionDef{
		id:       id,
		title:    "Test Connection",
		endpoint: endpoint,
	}
}

// Title sets the action button label.
// Default is "Test Connection".
//
// Example:
//
//	sdk.TestAction("test-connection", "/health").Title("Verify API Key")
func (a *TestActionDef) Title(title string) *TestActionDef {
	a.title = title
	return a
}

func (a *TestActionDef) build() map[string]any {
	return map[string]any{
		"id":       a.id,
		"type":     "test",
		"title":    a.title,
		"endpoint": a.endpoint,
	}
}

// ============================================================================
// List Action Builder
// ============================================================================

// ListActionDef defines a list action that displays items from an endpoint.
// Use this for managing collections like models, sources, or configurations.
type ListActionDef struct {
	id             string
	title          string
	tabTitle       string
	endpoint       string
	params         map[string]string
	showSystemInfo bool
	display        *ListDisplay
	itemActions    []ItemAction
	emptyState     *EmptyState
	dependsOn      *DependsOnConfig
}

// ListAction creates a list action builder.
// The endpoint should return a JSON response with:
//
//	{"items": [{"id": "1", "name": "Item 1", ...}, ...]}
//
// Example:
//
//	sdk.ListAction("model-list", "/models")
func ListAction(id, endpoint string) *ListActionDef {
	return &ListActionDef{
		id:       id,
		endpoint: endpoint,
	}
}

// Title sets the list header title.
//
// Example:
//
//	sdk.ListAction("models", "/models").Title("Available Models")
func (a *ListActionDef) Title(title string) *ListActionDef {
	a.title = title
	return a
}

// TabTitle sets the tab title. When set, this action appears as a tab
// in the settings form instead of inline.
//
// Example:
//
//	sdk.ListAction("models", "/models").TabTitle("Models")
func (a *ListActionDef) TabTitle(title string) *ListActionDef {
	a.tabTitle = title
	return a
}

// Params sets query parameters to include when calling the endpoint.
// Use this to filter list results.
//
// Example:
//
//	sdk.ListAction("embedding-models", "/models").
//	    Params(map[string]string{"type": "embedding"})
func (a *ListActionDef) Params(params map[string]string) *ListActionDef {
	a.params = params
	return a
}

// ShowSystemInfo enables display of system resource information.
// When enabled, shows available RAM/VRAM above the list.
// Useful for model lists where resource requirements matter.
//
// The endpoint response should include:
//
//	{"items": [...], "systemInfo": {"ramBytes": 16000000000, "vramBytes": 8000000000, "hasGpu": true}}
func (a *ListActionDef) ShowSystemInfo() *ListActionDef {
	a.showSystemInfo = true
	return a
}

// DependsOn sets conditional visibility - this action/tab only shows when
// the specified field has the specified value.
//
// Example:
//
//	sdk.ListAction("embedding-models", "/models").
//	    TabTitle("Ollama Embedding Models").
//	    DependsOn("embedding_provider", "ai-local")
func (a *ListActionDef) DependsOn(field string, value any) *ListActionDef {
	a.dependsOn = &DependsOnConfig{Field: field, Value: value}
	return a
}

// Display sets how list items are rendered.
// Use NewListDisplay to configure field mapping.
//
// Example:
//
//	sdk.ListAction("models", "/models").
//	    Display(sdk.NewListDisplay("name").SecondaryField("description"))
func (a *ListActionDef) Display(display *ListDisplay) *ListActionDef {
	a.display = display
	return a
}

// ItemAction adds an action that appears on each list item.
// Common item actions: delete, download, configure.
//
// Example:
//
//	sdk.ListAction("models", "/models").
//	    ItemAction(sdk.NewDeleteAction("delete", "/models/:id"))
func (a *ListActionDef) ItemAction(action ItemAction) *ListActionDef {
	a.itemActions = append(a.itemActions, action)
	return a
}

// EmptyState sets what to show when the list is empty.
//
// Example:
//
//	sdk.ListAction("models", "/models").
//	    EmptyState(sdk.NewEmptyState("No models installed").
//	        Description("Pull a model to get started"))
func (a *ListActionDef) EmptyState(state *EmptyState) *ListActionDef {
	a.emptyState = state
	return a
}

func (a *ListActionDef) build() map[string]any {
	action := map[string]any{
		"id":   a.id,
		"type": "list",
	}
	if a.title != "" {
		action["title"] = a.title
	}
	if a.tabTitle != "" {
		action["tabTitle"] = a.tabTitle
	}

	source := map[string]any{"endpoint": a.endpoint}
	if len(a.params) > 0 {
		source["params"] = a.params
	}
	action["source"] = source

	if a.showSystemInfo {
		action["showSystemInfo"] = true
	}
	if a.display != nil {
		action["display"] = a.display.build()
	}
	if len(a.itemActions) > 0 {
		items := make([]any, len(a.itemActions))
		for i, ia := range a.itemActions {
			items[i] = ia.build()
		}
		action["itemActions"] = items
	}
	if a.emptyState != nil {
		action["emptyState"] = a.emptyState.build()
	}
	if a.dependsOn != nil {
		action["x-viewra-depends-on"] = map[string]any{
			"field": a.dependsOn.Field,
			"value": a.dependsOn.Value,
		}
	}
	return action
}

// ============================================================================
// List Display Configuration
// ============================================================================

// ListDisplay configures how list items are rendered.
type ListDisplay struct {
	primaryField   string
	secondaryField string
	badges         []Badge
	metadata       []string
}

// NewListDisplay creates a list display configuration.
// The primary field is required and shown as the main text for each item.
//
// Example:
//
//	sdk.NewListDisplay("name").SecondaryField("description")
func NewListDisplay(primaryField string) *ListDisplay {
	return &ListDisplay{primaryField: primaryField}
}

// SecondaryField sets the secondary display field.
// Shown below the primary field in smaller text.
//
// Example:
//
//	sdk.NewListDisplay("name").SecondaryField("description")
func (d *ListDisplay) SecondaryField(field string) *ListDisplay {
	d.secondaryField = field
	return d
}

// Badge adds a conditional badge to list items.
// Badges are colored labels that appear when a condition is met.
//
// Example:
//
//	sdk.NewListDisplay("name").
//	    Badge(sdk.NewBadge("installed", true, "Installed", "emerald")).
//	    Badge(sdk.NewBadge("canRun", false, "Too Large", "red"))
func (d *ListDisplay) Badge(badge Badge) *ListDisplay {
	d.badges = append(d.badges, badge)
	return d
}

// Metadata sets additional fields to show as small metadata items.
//
// Example:
//
//	sdk.NewListDisplay("name").Metadata("size", "created")
func (d *ListDisplay) Metadata(fields ...string) *ListDisplay {
	d.metadata = fields
	return d
}

func (d *ListDisplay) build() map[string]any {
	display := map[string]any{
		"primaryField": d.primaryField,
	}
	if d.secondaryField != "" {
		display["secondaryField"] = d.secondaryField
	}
	if len(d.badges) > 0 {
		badges := make([]any, len(d.badges))
		for i, b := range d.badges {
			badges[i] = b.build()
		}
		display["badges"] = badges
	}
	if len(d.metadata) > 0 {
		display["metadata"] = d.metadata
	}
	return display
}

// Badge represents a conditional badge configuration.
// Badges appear on list items when a field matches a value.
type Badge struct {
	field string
	value any
	label string
	color string
}

// NewBadge creates a badge configuration.
// The badge appears when item[field] == value.
//
// Colors: "blue", "green", "emerald", "yellow", "red", "purple", "gray"
//
// Example:
//
//	// Show "Installed" badge when installed field is true
//	sdk.NewBadge("installed", true, "Installed", "emerald")
//
//	// Show "Error" badge when status is "failed"
//	sdk.NewBadge("status", "failed", "Error", "red")
func NewBadge(field string, value any, label, color string) Badge {
	return Badge{field: field, value: value, label: label, color: color}
}

func (b Badge) build() map[string]any {
	return map[string]any{
		"field": b.field,
		"value": b.value,
		"label": b.label,
		"color": b.color,
	}
}

// ============================================================================
// Item Actions
// ============================================================================

// ItemAction represents an action button on a list item.
type ItemAction interface {
	build() map[string]any
}

// StreamingItemAction represents a streaming action (e.g., model pull).
// The endpoint should return Server-Sent Events with progress updates.
type StreamingItemAction struct {
	id       string
	label    string
	endpoint string
	showWhen *ShowWhen
}

// NewStreamingAction creates a streaming item action.
// Use this for long-running operations like downloads.
//
// The endpoint receives POST with item data and should return SSE:
//
//	data: {"status": "Downloading...", "percent": 50}
//	data: {"done": true}
//
// Example:
//
//	sdk.NewStreamingAction("pull", "Pull", "/models/pull")
func NewStreamingAction(id, label, endpoint string) *StreamingItemAction {
	return &StreamingItemAction{id: id, label: label, endpoint: endpoint}
}

// ShowWhen sets a condition for showing this action.
// The action only appears on items where item[field] == value.
//
// Example:
//
//	// Only show "Pull" on items that aren't installed
//	sdk.NewStreamingAction("pull", "Pull", "/models/pull").
//	    ShowWhen("installed", false)
func (a *StreamingItemAction) ShowWhen(field string, value any) *StreamingItemAction {
	a.showWhen = &ShowWhen{Field: field, Value: value}
	return a
}

func (a *StreamingItemAction) build() map[string]any {
	action := map[string]any{
		"id":        a.id,
		"type":      "action",
		"label":     a.label,
		"endpoint":  a.endpoint,
		"streaming": true,
	}
	if a.showWhen != nil {
		action["showWhen"] = map[string]any{
			"field": a.showWhen.Field,
			"value": a.showWhen.Value,
		}
	}
	return action
}

// DeleteItemAction represents a delete action with confirmation.
type DeleteItemAction struct {
	id             string
	label          string
	endpoint       string
	showWhen       *ShowWhen
	confirmTitle   string
	confirmMessage string
}

// NewDeleteAction creates a delete item action.
// The endpoint should support DELETE method with the item ID.
// Use :id in the endpoint path for the item ID placeholder.
//
// Example:
//
//	sdk.NewDeleteAction("delete", "/models/:id")
func NewDeleteAction(id, endpoint string) *DeleteItemAction {
	return &DeleteItemAction{id: id, label: "Delete", endpoint: endpoint}
}

// Label sets the action button label.
// Default is "Delete".
func (a *DeleteItemAction) Label(label string) *DeleteItemAction {
	a.label = label
	return a
}

// ShowWhen sets a condition for showing this action.
//
// Example:
//
//	// Only show delete on installed items
//	sdk.NewDeleteAction("delete", "/models/:id").ShowWhen("installed", true)
func (a *DeleteItemAction) ShowWhen(field string, value any) *DeleteItemAction {
	a.showWhen = &ShowWhen{Field: field, Value: value}
	return a
}

// Confirm sets the confirmation dialog text.
// When set, a confirmation modal appears before deleting.
//
// Example:
//
//	sdk.NewDeleteAction("delete", "/models/:id").
//	    Confirm("Delete Model", "Are you sure you want to delete this model?")
func (a *DeleteItemAction) Confirm(title, message string) *DeleteItemAction {
	a.confirmTitle = title
	a.confirmMessage = message
	return a
}

func (a *DeleteItemAction) build() map[string]any {
	action := map[string]any{
		"id":       a.id,
		"type":     "delete",
		"label":    a.label,
		"endpoint": a.endpoint,
	}
	if a.showWhen != nil {
		action["showWhen"] = map[string]any{
			"field": a.showWhen.Field,
			"value": a.showWhen.Value,
		}
	}
	if a.confirmTitle != "" || a.confirmMessage != "" {
		action["confirm"] = map[string]any{
			"title":   a.confirmTitle,
			"message": a.confirmMessage,
		}
	}
	return action
}

// ShowWhen represents a condition for showing an action.
type ShowWhen struct {
	Field string
	Value any
}

// EmptyState represents the empty state configuration for a list.
type EmptyState struct {
	title       string
	description string
	showCreate  string
}

// NewEmptyState creates an empty state configuration.
// Shown when the list has no items.
//
// Example:
//
//	sdk.NewEmptyState("No models installed")
func NewEmptyState(title string) *EmptyState {
	return &EmptyState{title: title}
}

// Description sets the empty state description.
//
// Example:
//
//	sdk.NewEmptyState("No models").Description("Pull a model to get started")
func (e *EmptyState) Description(desc string) *EmptyState {
	e.description = desc
	return e
}

// ShowCreate sets an action ID to show a "create" button.
// When clicked, switches to the referenced action (e.g., a create form).
func (e *EmptyState) ShowCreate(actionID string) *EmptyState {
	e.showCreate = actionID
	return e
}

func (e *EmptyState) build() map[string]any {
	state := map[string]any{"title": e.title}
	if e.description != "" {
		state["description"] = e.description
	}
	if e.showCreate != "" {
		state["showCreate"] = e.showCreate
	}
	return state
}

// ============================================================================
// Section Builder
// ============================================================================

// Section represents an x-viewra-sections entry.
// Sections group properties and actions by capability, allowing
// the host UI to filter what's shown based on context.
//
// For example, an AI provider plugin might have sections for:
//   - "connection": base_url, test-connection (shown for both embedding and chat)
//   - "embedding": embedding_model (shown only when used as embedding provider)
//   - "chat": chat_model (shown only when used as chat provider)
type Section struct {
	id           string
	title        string
	properties   []string
	actions      []string
	capabilities []string
	dependsOn    *DependsOnConfig
}

// NewSection creates a section builder.
// The ID should be unique within the schema.
//
// Example:
//
//	sdk.NewSection("connection")
func NewSection(id string) *Section {
	return &Section{id: id}
}

// Title sets the section title (optional, for display purposes).
func (s *Section) Title(title string) *Section {
	s.title = title
	return s
}

// Properties sets which property names belong to this section.
// These should match property names defined with schema.Property().
//
// Example:
//
//	sdk.NewSection("connection").Properties("base_url", "timeout")
func (s *Section) Properties(props ...string) *Section {
	s.properties = props
	return s
}

// Actions sets which action IDs belong to this section.
// These should match action IDs defined with schema.Action().
//
// Example:
//
//	sdk.NewSection("connection").Actions("test-connection")
func (s *Section) Actions(actions ...string) *Section {
	s.actions = actions
	return s
}

// Capabilities sets which capabilities this section applies to.
// The host UI uses this to filter sections based on context.
//
// Standard capabilities:
//   - "embedding": Show when plugin is used as embedding provider
//   - "chat": Show when plugin is used as chat provider
//
// A section can belong to multiple capabilities.
//
// Example:
//
//	// Connection settings shown for both embedding and chat
//	sdk.NewSection("connection").
//	    Properties("base_url").
//	    Actions("test-connection").
//	    Capabilities("embedding", "chat")
//
//	// Embedding model only shown for embedding capability
//	sdk.NewSection("embedding").
//	    Properties("embedding_model").
//	    Capabilities("embedding")
func (s *Section) Capabilities(caps ...string) *Section {
	s.capabilities = caps
	return s
}

// DependsOn sets conditional visibility - this section only shows when
// the specified field has the specified value.
//
// Example:
//
//	sdk.NewSection("advanced").
//	    Properties("timeout", "retries").
//	    DependsOn("enabled", true)
func (s *Section) DependsOn(field string, value any) *Section {
	s.dependsOn = &DependsOnConfig{Field: field, Value: value}
	return s
}

func (s *Section) build() map[string]any {
	section := map[string]any{"id": s.id}
	if s.title != "" {
		section["title"] = s.title
	}
	if len(s.properties) > 0 {
		section["properties"] = s.properties
	}
	if len(s.actions) > 0 {
		section["actions"] = s.actions
	}
	if len(s.capabilities) > 0 {
		section["capabilities"] = s.capabilities
	}
	if s.dependsOn != nil {
		section["x-viewra-depends-on"] = map[string]any{
			"field": s.dependsOn.Field,
			"value": s.dependsOn.Value,
		}
	}
	return section
}
