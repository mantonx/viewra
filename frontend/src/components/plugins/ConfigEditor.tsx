import React, { useState, useEffect } from 'react';
import { Save, RotateCcw, AlertTriangle, CheckCircle, RefreshCw } from 'lucide-react';
import type {
  Plugin,
  ConfigurationSchema,
  PluginConfiguration,
  ConfigurationProperty,
} from '@/types/plugin.types';
import pluginApi from '@/lib/api/plugins';

interface ConfigEditorProps {
  plugin: Plugin;
  onConfigChange?: () => void;
}

const ConfigEditor: React.FC<ConfigEditorProps> = ({
  plugin,
  onConfigChange,
}) => {
  const [schema, setSchema] = useState<ConfigurationSchema | null>(null);
  const [config, setConfig] = useState<PluginConfiguration | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [hasChanges, setHasChanges] = useState(false);
  const [formData, setFormData] = useState<Record<string, unknown>>({});

  useEffect(() => {
    loadConfigurationData();
  }, [plugin.id]);

  const loadConfigurationData = async () => {
    try {
      setLoading(true);
      const [schemaResponse, configResponse] = await Promise.all([
        pluginApi.getPluginConfigSchema(plugin.id),
        pluginApi.getPluginConfig(plugin.id),
      ]);

      if (schemaResponse.success && schemaResponse.data) {
        setSchema(schemaResponse.data);
      }

      if (configResponse.success && configResponse.data) {
        setConfig(configResponse.data);
        // Extract values from settings object
        const settingsData: Record<string, unknown> = {};
        Object.entries(configResponse.data.settings).forEach(([key, configValue]) => {
          // Handle different value formats - the backend may return direct values or wrapped objects
          settingsData[key] = typeof configValue === 'object' && configValue !== null && 'value' in configValue 
            ? (configValue as { value: unknown }).value 
            : configValue;
        });
        setFormData(settingsData);
      }

      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load configuration');
    } finally {
      setLoading(false);
    }
  };

  const handleFieldChange = (fieldId: string, value: unknown) => {
    setFormData(prev => ({
      ...prev,
      [fieldId]: value,
    }));
    setHasChanges(true);
    setSuccess(null);
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      const response = await pluginApi.updatePluginConfig(plugin.id, formData);

      if (response.success) {
        setHasChanges(false);
        setSuccess('Configuration saved successfully');
        if (onConfigChange) {
          onConfigChange();
        }
        await loadConfigurationData();
      } else {
        setError(response.error || 'Failed to save configuration');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save configuration');
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    try {
      setSaving(true);
      const response = await pluginApi.resetPluginConfig(plugin.id);

      if (response.success) {
        setSuccess('Configuration reset to defaults');
        setHasChanges(false);
        await loadConfigurationData();
      } else {
        setError(response.error || 'Failed to reset configuration');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reset configuration');
    } finally {
      setSaving(false);
    }
  };

  const renderField = (field: ConfigurationProperty, fieldId: string) => {
    const value = formData[fieldId] ?? field.default;

    switch (field.type) {
      case 'text':
      case 'string':
        return (
          <input
            type={field.sensitive ? 'password' : 'text'}
            value={String(value || '')}
            onChange={(e) => handleFieldChange(fieldId, e.target.value)}
            placeholder={field.placeholder}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );

      case 'number':
      case 'integer':
        return (
          <input
            type="number"
            value={String(value || '')}
            onChange={(e) => handleFieldChange(fieldId, Number(e.target.value))}
            min={field.min}
            max={field.max}
            step={field.type === 'integer' ? 1 : 0.1}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );

      case 'boolean':
        return (
          <label className="flex items-center space-x-3">
            <input
              type="checkbox"
              checked={Boolean(value)}
              onChange={(e) => handleFieldChange(fieldId, e.target.checked)}
              className="w-4 h-4 text-purple-600 bg-slate-700 border-slate-600 rounded focus:ring-purple-500"
            />
            <span className="text-slate-300">
              {field.label || field.title}
            </span>
          </label>
        );

      case 'select':
        return (
          <select
            value={String(value || '')}
            onChange={(e) => handleFieldChange(fieldId, e.target.value)}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
          >
            <option value="">Select an option</option>
            {field.options?.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        );

      case 'textarea':
        return (
          <textarea
            value={String(value || '')}
            onChange={(e) => handleFieldChange(fieldId, e.target.value)}
            placeholder={field.placeholder}
            rows={field.rows || 4}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );

      default:
        return (
          <div className="text-slate-400 italic">
            Unsupported field type: {field.type}
          </div>
        );
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <RefreshCw size={24} className="animate-spin text-purple-400 mr-3" />
        <span className="text-slate-300">Loading configuration...</span>
      </div>
    );
  }

  if (error && !schema) {
    return (
      <div className="bg-red-600/20 border border-red-600 rounded-lg p-4">
        <div className="flex items-center">
          <AlertTriangle size={20} className="text-red-400 mr-2" />
          <span className="text-red-400">{error}</span>
        </div>
        <button
          onClick={loadConfigurationData}
          className="mt-3 bg-red-600 hover:bg-red-700 px-3 py-1 rounded text-white text-sm"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!schema) {
    return (
      <div className="text-center py-8">
        <div className="text-slate-400 mb-4">No configuration schema available</div>
        <p className="text-sm text-slate-500">This plugin does not provide configurable settings</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-medium text-white">{schema.title}</h3>
          {schema.description && (
            <p className="text-sm text-slate-400 mt-1">{schema.description}</p>
          )}
        </div>
        <div className="flex items-center space-x-2">
          <button
            onClick={handleReset}
            disabled={saving}
            className="flex items-center px-3 py-2 bg-gray-600 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md text-sm"
          >
            <RotateCcw size={16} className="mr-2" />
            Reset
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !hasChanges}
            className="flex items-center px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md text-sm"
          >
            {saving ? (
              <RefreshCw size={16} className="mr-2 animate-spin" />
            ) : (
              <Save size={16} className="mr-2" />
            )}
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>

      {/* Status Messages */}
      {error && (
        <div className="bg-red-600/20 border border-red-600 rounded-lg p-4">
          <div className="flex items-center">
            <AlertTriangle size={16} className="text-red-400 mr-2" />
            <span className="text-red-400">{error}</span>
          </div>
        </div>
      )}

      {success && (
        <div className="bg-green-600/20 border border-green-600 rounded-lg p-4">
          <div className="flex items-center">
            <CheckCircle size={16} className="text-green-400 mr-2" />
            <span className="text-green-400">{success}</span>
          </div>
        </div>
      )}

      {/* Configuration Form */}
      <div className="space-y-6">
        {schema.categories && schema.categories.length > 0 ? (
          // Render by categories
          schema.categories.map((category) => (
            <div key={category.id} className="bg-slate-700 rounded-lg p-6">
              <h4 className="text-white font-medium mb-2">{category.title}</h4>
              {category.description && (
                <p className="text-slate-400 text-sm mb-4">{category.description}</p>
              )}
              <div className="space-y-4">
                {category.fields.map((field) => {
                  const fieldId = field.id || field.title.toLowerCase().replace(/\s+/g, '_');
                  return (
                    <div key={fieldId}>
                      <label className="block text-sm font-medium text-white mb-2">
                        {field.label || field.title}
                        {field.required && <span className="text-red-400 ml-1">*</span>}
                      </label>
                      {field.description && (
                        <p className="text-slate-400 text-xs mb-2">{field.description}</p>
                      )}
                      {renderField(field, fieldId)}
                    </div>
                  );
                })}
              </div>
            </div>
          ))
        ) : (
          // Render properties directly
          <div className="bg-slate-700 rounded-lg p-6">
            <div className="space-y-4">
              {Object.entries(schema.properties).map(([fieldId, field]) => (
                <div key={fieldId}>
                  <label className="block text-sm font-medium text-white mb-2">
                    {field.label || field.title}
                    {field.required && <span className="text-red-400 ml-1">*</span>}
                  </label>
                  {field.description && (
                    <p className="text-slate-400 text-xs mb-2">{field.description}</p>
                  )}
                  {renderField(field, fieldId)}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Configuration Info */}
      {config && (
        <div className="bg-slate-700 rounded-lg p-4">
          <h4 className="text-white font-medium mb-2">Configuration Info</h4>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-slate-400">Version:</span>
              <span className="text-white ml-2">{config.version}</span>
            </div>
            <div>
              <span className="text-slate-400">Last Modified:</span>
              <span className="text-white ml-2">
                {config.last_modified ? new Date(config.last_modified).toLocaleString() : 'Never'}
              </span>
            </div>
            <div>
              <span className="text-slate-400">Modified By:</span>
              <span className="text-white ml-2">{config.modified_by || 'System'}</span>
            </div>
            <div>
              <span className="text-slate-400">Fields:</span>
              <span className="text-white ml-2">{Object.keys(schema.properties).length}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ConfigEditor; 