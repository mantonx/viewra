export interface Plugin {
  id: string;
  name: string;
  version?: string;
  description?: string;
  author?: string;
  enabled?: boolean;
  type?: string;
  status?: string;
  manifest?: PluginManifest;
  config?: Record<string, unknown>;
}

export interface PluginManifest {
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  website?: string;
  repository?: string;
  license?: string;
  type?: string;
  tags?: string[];
  capabilities?: {
    metadata_extraction?: boolean;
    admin_pages?: boolean;
    ui_components?: boolean;
    api_endpoints?: boolean;
    background_tasks?: boolean;
    file_transcoding?: boolean;
    notifications?: boolean;
    database_access?: boolean;
    external_services?: boolean;
  };
  dependencies?: {
    viewra_version?: string;
    plugins?: Record<string, string>;
  };
  config_schema?: ConfigSchema;
  permissions?: string[];
  ui?: {
    admin_pages?: AdminPageConfig[];
    components?: UIComponentConfig[];
  };
}

export interface ConfigSchema {
  type: string;
  properties: Record<string, PropertySchema>;
}

export interface PropertySchema {
  type: string;
  title?: string;
  description?: string;
  default?: unknown;
}

export interface ConfigValue {
  [key: string]: unknown;
}

export interface AdminPageConfig {
  id: string;
  title: string;
  path: string;
  icon?: string;
  category?: string;
  url: string;
  type: string;
}

export interface UIComponentConfig {
  id: string;
  name: string;
  type: string;
  url: string;
}

export interface UIComponent {
  id: number;
  plugin_id: number;
  plugin: Plugin;
  component_id: string;
  name: string;
  type: string;
  url: string;
  enabled: boolean;
}

export interface PluginEvent {
  id: number;
  plugin_id: number;
  plugin: Plugin;
  event_type: string;
  status: string;
  message: string;
  timestamp: string;
  details: Record<string, unknown>;
}

export interface PluginResponse {
  plugins: Plugin[];
  count: number;
}

export interface UIComponentsResponse {
  ui_components: UIComponent[];
  count: number;
}

export interface PluginEventsResponse {
  events: PluginEvent[];
  count: number;
}

export interface PluginConfigEditorProps {
  pluginId?: string;
  schema?: ConfigSchema;
  initialValues?: ConfigValue;
  onSave?: (values: ConfigValue) => Promise<void>;
}

export interface PluginDependenciesProps {
  viewraDependency?: string;
  pluginDependencies?: Record<string, string>;
  satisfied?: boolean;
}

export interface PluginPermission {
  id: number;
  plugin_id: number;
  plugin: Plugin;
  permission_name: string;
  description: string;
  granted: boolean;
  required: boolean;
}

export interface PermissionsResponse {
  permissions: PluginPermission[];
  count: number;
}
