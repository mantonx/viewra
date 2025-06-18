// Core Plugin Types
export interface Plugin {
  id: string;
  name: string;
  version: string;
  description: string;
  author?: string;
  website?: string;
  repository?: string;
  license?: string;
  type: string;
  enabled: boolean;
  status: 'active' | 'disabled' | 'error' | 'loading' | 'installing';
  is_core: boolean;
  
  // Plugin capabilities and metadata
  capabilities?: PluginCapabilities;
  dependencies?: string[];
  dependents?: string[];
  permissions?: string[];
  
  // Runtime information
  health?: PluginHealth;
  configuration?: Record<string, unknown>;
  admin_pages?: AdminPage[];
  last_activity?: string;
  installation_date?: string;
  update_available?: boolean;
  latest_version?: string;
  configuration_hash?: string;
}

export interface PluginCapabilities {
  metadata_extraction?: boolean;
  admin_pages?: boolean;
  ui_components?: boolean;
  api_endpoints?: boolean;
  background_tasks?: boolean;
  file_transcoding?: boolean;
  notifications?: boolean;
  database_access?: boolean;
  external_services?: boolean;
}

export interface PluginHealth {
  status: string;
  running: boolean;
  healthy: boolean;
  error_rate: number;
  total_requests: number;
  successful_requests: number;
  failed_requests: number;
  consecutive_failures: number;
  average_response_time: string;
  uptime: string;
  last_error?: string;
  last_check_time: string;
  start_time: string;
}

export interface AdminPage {
  id: string;
  title: string;
  path: string;
  icon?: string;
  category?: string;
  url: string;
  type: 'configuration' | 'dashboard' | 'status' | 'external';
  permissions?: string[];
}

// Configuration System
export interface ConfigurationSchema {
  version: string;
  title: string;
  description: string;
  properties: Record<string, ConfigurationProperty>;
  required?: string[];
  categories?: ConfigurationCategory[];
}

export interface ConfigurationProperty {
  id?: string;
  type: 'string' | 'number' | 'integer' | 'boolean' | 'array' | 'object' | 'text' | 'select' | 'textarea';
  title: string;
  label?: string;
  description: string;
  default?: unknown;
  enum?: unknown[];
  minimum?: number;
  maximum?: number;
  min?: number;
  max?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
  placeholder?: string;
  rows?: number;
  items?: ConfigurationProperty;
  properties?: Record<string, ConfigurationProperty>;
  options?: Array<{ value: string; label: string }>;
  dependencies?: string[];
  sensitive?: boolean;
  advanced?: boolean;
  is_basic?: boolean;
  importance?: number;
  user_friendly?: boolean;
  readOnly?: boolean;
  required?: boolean;
  category?: string;
  order?: number;
  conditional?: ConditionalProperty;
  validation_message?: string;
}

export interface ConditionalProperty {
  property: string;
  value: unknown;
  operator: 'eq' | 'ne' | 'gt' | 'lt' | 'contains';
}

export interface ConfigurationCategory {
  id: string;
  title: string;
  description?: string;
  order?: number;
  collapsible?: boolean;
  collapsed?: boolean;
  fields: ConfigurationProperty[];
}

export interface PluginConfiguration {
  plugin_id: string;
  schema?: ConfigurationSchema;
  settings: Record<string, ConfigurationValue>;
  version: string;
  last_modified: string;
  modified_by: string;
  validation_rules?: ValidationRule[];
  dependencies?: string[];
  permissions?: string[];
}

export interface ConfigurationValue {
  value: unknown;
  type: string;
  source: string;
  overridden: boolean;
  last_changed: string;
  changed_by: string;
  validation?: ValidationResult;
}

export interface ValidationResult {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
}

export interface ValidationRule {
  id: string;
  type: string;
  properties: string[];
  message: string;
  condition: Record<string, unknown>;
  severity: 'error' | 'warning' | 'info';
}

// API Response Types
export interface APIResponse<T = unknown> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
  timestamp: string;
  request_id?: string;
}

export interface PaginatedResponse<T = unknown> extends APIResponse<T[]> {
  pagination?: PaginationMeta;
}

export interface PaginationMeta {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_previous: boolean;
}

// System Status Types
export interface SystemStatus {
  plugins_total: number;
  plugins_enabled: number;
  plugins_disabled: number;
  plugins_error: number;
  hot_reload_enabled: boolean;
  last_refresh: string;
  health_summary: {
    healthy: number;
    degraded: number;
    unhealthy: number;
  };
}

export interface HotReloadStatus {
  enabled: boolean;
  pending_reloads: number;
  pending_plugins: string[];
  debounce_delay: string;
}

// Filter and Search Types
export interface PluginFilters {
  category?: string;
  status?: string;
  type?: string;
  search?: string;
  enabled?: boolean;
  has_config?: boolean;
}

export interface PluginSearchResult {
  plugins: Plugin[];
  total: number;
  categories: string[];
  types: string[];
}

// Plugin Update Types
export interface PluginUpdateRequest {
  version?: string;
  configuration?: Record<string, unknown>;
  enabled?: boolean;
}

// Bulk Operations
export interface BulkOperationRequest {
  plugin_ids: string[];
  operation: 'enable' | 'disable' | 'update' | 'restart';
  parameters?: Record<string, unknown>;
}

export interface BulkOperationResult {
  success: string[];
  failed: Record<string, string>;
  total_processed: number;
}

// Response Types
export interface PluginResponse {
  plugins: Plugin[];
  total_count?: number;
}

export interface AdminNavigation {
  [category: string]: AdminPage[];
}

export interface PluginPermissionInfo {
  plugin_id: string;
  permission: string;
  granted: boolean;
  granted_at?: string;
  description?: string;
}

export interface GlobalPluginSettings {
  auto_enable_plugins: boolean;
  auto_update_plugins: boolean;
  plugin_timeout_seconds: number;
  max_concurrent_plugins: number;
  hot_reload_enabled: boolean;
  debug_mode: boolean;
}
