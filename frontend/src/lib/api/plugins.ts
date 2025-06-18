import type {
  APIResponse,
  Plugin,
  PluginConfiguration,
  AdminPage,
  ConfigurationSchema,
} from '@/types/plugin.types';

class PluginAPIService {
  private baseUrl = '/api/v1/plugins';

  async listAllPlugins(page = 1, limit = 20, filters?: {
    category?: string;
    status?: string;
    type?: string;
  }): Promise<APIResponse<Plugin[]>> {
    const params = new URLSearchParams({
      page: page.toString(),
      limit: limit.toString(),
      ...(filters?.category && { category: filters.category }),
      ...(filters?.status && { status: filters.status }),
      ...(filters?.type && { type: filters.type }),
    });
    
    const response = await fetch(`${this.baseUrl}/?${params}`);
    return response.json();
  }

  async searchPlugins(query: string, page = 1, limit = 20): Promise<APIResponse<Plugin[]>> {
    const params = new URLSearchParams({
      q: query,
      page: page.toString(),
      limit: limit.toString(),
    });
    
    const response = await fetch(`${this.baseUrl}/search?${params}`);
    return response.json();
  }

  async getPlugin(id: string): Promise<APIResponse<Plugin>> {
    const response = await fetch(`${this.baseUrl}/${id}`);
    return response.json();
  }

  async enablePlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/${id}/enable`, {
      method: 'POST',
    });
    return response.json();
  }

  async disablePlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/${id}/disable`, {
      method: 'POST',
    });
    return response.json();
  }

  async restartPlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/${id}/restart`, {
      method: 'POST',
    });
    return response.json();
  }

  async reloadPlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/${id}/reload`, {
      method: 'POST',
    });
    return response.json();
  }

  // Configuration Management
  async getPluginConfig(id: string): Promise<APIResponse<PluginConfiguration>> {
    const response = await fetch(`${this.baseUrl}/${id}/config`);
    return response.json();
  }

  async updatePluginConfig(id: string, config: Record<string, unknown>): Promise<APIResponse<PluginConfiguration>> {
    const response = await fetch(`${this.baseUrl}/${id}/config`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    });
    return response.json();
  }

  async getPluginConfigSchema(id: string): Promise<APIResponse<ConfigurationSchema>> {
    const response = await fetch(`${this.baseUrl}/${id}/config/schema`);
    return response.json();
  }

  async validatePluginConfig(id: string, config: Record<string, unknown>): Promise<APIResponse<ValidationResult>> {
    const response = await fetch(`${this.baseUrl}/${id}/config/validate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    });
    return response.json();
  }

  async resetPluginConfig(id: string): Promise<APIResponse<PluginConfiguration>> {
    const response = await fetch(`${this.baseUrl}/${id}/config/reset`, {
      method: 'POST',
    });
    return response.json();
  }

  // Health & Monitoring
  async getPluginHealth(id: string): Promise<APIResponse<PluginHealthInfo>> {
    const response = await fetch(`${this.baseUrl}/${id}/health`);
    return response.json();
  }

  async getPluginMetrics(id: string): Promise<APIResponse<unknown>> {
    const response = await fetch(`${this.baseUrl}/${id}/metrics`);
    return response.json();
  }

  async resetPluginHealth(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/${id}/health/reset`, {
      method: 'POST',
    });
    return response.json();
  }

  // Admin Pages & UI
  async getPluginAdminPages(id: string): Promise<APIResponse<AdminPage[]>> {
    const response = await fetch(`${this.baseUrl}/${id}/admin-pages`);
    return response.json();
  }

  async getAllAdminPages(): Promise<APIResponse<AdminPage[]>> {
    const response = await fetch(`${this.baseUrl}/admin/pages`);
    return response.json();
  }

  async getAdminNavigation(): Promise<APIResponse<Record<string, AdminPage[]>>> {
    const response = await fetch(`${this.baseUrl}/admin/navigation`);
    return response.json();
  }

  // System Management
  async getSystemStatus(): Promise<APIResponse<unknown>> {
    const response = await fetch(`${this.baseUrl}/system/status`);
    return response.json();
  }

  async getSystemStats(): Promise<APIResponse<unknown>> {
    const response = await fetch(`${this.baseUrl}/system/stats`);
    return response.json();
  }

  async refreshAllPlugins(): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/refresh`, {
      method: 'POST',
    });
    return response.json();
  }

  async cleanupSystem(): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/cleanup`, {
      method: 'POST',
    });
    return response.json();
  }

  // Hot Reload Management
  async getHotReloadStatus(): Promise<APIResponse<HotReloadStatus>> {
    const response = await fetch(`${this.baseUrl}/system/hot-reload`);
    return response.json();
  }

  async enableHotReload(): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/hot-reload/enable`, {
      method: 'POST',
    });
    return response.json();
  }

  async disableHotReload(): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/hot-reload/disable`, {
      method: 'POST',
    });
    return response.json();
  }

  async triggerHotReload(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/hot-reload/trigger/${id}`, {
      method: 'POST',
    });
    return response.json();
  }

  // Core Plugins
  async listCorePlugins(): Promise<APIResponse<Plugin[]>> {
    const response = await fetch(`${this.baseUrl}/core/`);
    return response.json();
  }

  async getCorePlugin(name: string): Promise<APIResponse<Plugin>> {
    const response = await fetch(`${this.baseUrl}/core/${name}`);
    return response.json();
  }

  async enableCorePlugin(name: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/core/${name}/enable`, {
      method: 'POST',
    });
    return response.json();
  }

  async disableCorePlugin(name: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/core/${name}/disable`, {
      method: 'POST',
    });
    return response.json();
  }

  // External Plugins
  async listExternalPlugins(): Promise<APIResponse<Plugin[]>> {
    const response = await fetch(`${this.baseUrl}/external/`);
    return response.json();
  }

  async getExternalPlugin(id: string): Promise<APIResponse<Plugin>> {
    const response = await fetch(`${this.baseUrl}/external/${id}`);
    return response.json();
  }

  async loadExternalPlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/external/${id}/load`, {
      method: 'POST',
    });
    return response.json();
  }

  async unloadExternalPlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/external/${id}/unload`, {
      method: 'POST',
    });
    return response.json();
  }

  async refreshExternalPlugins(): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/external/refresh`, {
      method: 'POST',
    });
    return response.json();
  }

  // Bulk Operations
  async bulkEnable(pluginIds: string[]): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/bulk/enable`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ plugin_ids: pluginIds }),
    });
    return response.json();
  }

  async bulkDisable(pluginIds: string[]): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/system/bulk/disable`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ plugin_ids: pluginIds }),
    });
    return response.json();
  }

  // Utility methods
  async getPluginCategories(): Promise<APIResponse<string[]>> {
    const response = await fetch(`${this.baseUrl}/categories`);
    return response.json();
  }

  async getSystemCapabilities(): Promise<APIResponse<Record<string, unknown>>> {
    const response = await fetch(`${this.baseUrl}/capabilities`);
    return response.json();
  }

  // Real-time updates helpers
  createEventSource(endpoint: string): EventSource {
    return new EventSource(`${this.baseUrl}/${endpoint}`);
  }

  // Plugin installation/management (for future)
  async installPlugin(source: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/external/`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ source }),
    });
    return response.json();
  }

  async uninstallPlugin(id: string): Promise<APIResponse<void>> {
    const response = await fetch(`${this.baseUrl}/${id}`, {
      method: 'DELETE',
    });
    return response.json();
  }
}

// Additional types for new functionality
interface HotReloadStatus {
  enabled: boolean;
  pending_reloads?: number;
  pending_plugins?: string[];
  debounce_delay?: string;
  error?: string;
}

interface ValidationResult {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
}

interface PluginHealthInfo {
  status: 'healthy' | 'unhealthy' | 'unknown';
  last_check: string;
  response_time?: number;
  error?: string;
  consecutive_failures?: number;
  uptime?: number;
  memory_usage?: number;
  cpu_usage?: number;
  open_files?: number;
  active_connections?: number;
}

export default new PluginAPIService(); 