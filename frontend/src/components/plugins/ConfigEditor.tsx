import React, { useState, useEffect, useMemo } from 'react';
import { 
  Save, 
  RotateCcw, 
  AlertTriangle, 
  CheckCircle, 
  RefreshCw, 
  ChevronDown, 
  ChevronRight, 
  Eye, 
  EyeOff,
  Settings,
  Zap,
  Shield,
  Search
} from 'lucide-react';
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
  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({});
  const [showSensitive, setShowSensitive] = useState<Record<string, boolean>>({});
  const [searchTerm, setSearchTerm] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);

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
        // Auto-expand General category and other important categories
        const expanded: Record<string, boolean> = {
          'General': true,  // Always expand General category
        };
        // Also expand first few other categories if they exist
        schemaResponse.data.categories?.slice(0, 2).forEach(cat => {
          if (cat.id !== 'General') {
            expanded[cat.id] = !cat.collapsed;
          }
        });
        setExpandedCategories(expanded);
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
        
        // Fill in default values for any missing settings from schema
        if (schemaResponse.success && schemaResponse.data?.properties) {
          Object.entries(schemaResponse.data.properties).forEach(([key, property]) => {
            if (settingsData[key] === undefined && property.default !== undefined) {
              settingsData[key] = property.default;
            }
          });
        }
        
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
    setFormData(prev => {
      // Handle nested field paths (e.g., "adaptive.enabled")
      if (fieldId.includes('.')) {
        const [parentKey, ...subKeys] = fieldId.split('.');
        const subPath = subKeys.join('.');
        
        const parentValue = prev[parentKey];
        const parentObject = (parentValue && typeof parentValue === 'object' && !Array.isArray(parentValue)) 
          ? parentValue as Record<string, unknown>
          : {};
        
        return {
          ...prev,
          [parentKey]: {
            ...parentObject,
            [subPath]: value,
          },
        };
      }
      
      // Handle top-level fields
      return {
        ...prev,
        [fieldId]: value,
      };
    });
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

  const toggleCategory = (categoryId: string) => {
    setExpandedCategories(prev => ({
      ...prev,
      [categoryId]: !prev[categoryId]
    }));
  };

  const toggleSensitiveVisibility = (fieldId: string) => {
    setShowSensitive(prev => ({
      ...prev,
      [fieldId]: !prev[fieldId]
    }));
  };

  // Legacy renderField function - keeping for potential compatibility
  const renderField = (field: ConfigurationProperty, fieldId: string) => {
    // Handle nested field paths (e.g., "adaptive.enabled")
    let value: any;
    if (fieldId.includes('.')) {
      const [parentKey, ...subKeys] = fieldId.split('.');
      const subPath = subKeys.join('.');
      const parentValue = formData[parentKey];
      if (parentValue && typeof parentValue === 'object' && !Array.isArray(parentValue)) {
        value = (parentValue as Record<string, unknown>)[subPath];
      }
    } else {
      value = formData[fieldId];
    }
    
    // Use default if value is undefined
    if (value === undefined) {
      value = field.default;
    }

    const renderControl = () => {
      switch (field.type) {
        case 'text':
        case 'string':
          return (
            <div className="relative">
              <input
                type={field.sensitive && !showSensitive[fieldId] ? 'password' : 'text'}
                value={String(value || '')}
                onChange={(e) => handleFieldChange(fieldId, e.target.value)}
                placeholder={`Enter ${field.title.toLowerCase()}`}
                className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500 pr-10"
                {...(field.pattern && { pattern: field.pattern })}
                {...(field.minLength && { minLength: field.minLength })}
                {...(field.maxLength && { maxLength: field.maxLength })}
              />
              {field.sensitive && (
                <button
                  type="button"
                  onClick={() => toggleSensitiveVisibility(fieldId)}
                  className="absolute right-2 top-1/2 transform -translate-y-1/2 text-slate-400 hover:text-white"
                >
                  {showSensitive[fieldId] ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              )}
            </div>
          );

        case 'number':
          return (
            <div className="space-y-2">
              <div className="flex items-center space-x-2">
                <input
                  type="number"
                  value={String(value || '')}
                  onChange={(e) => handleFieldChange(fieldId, Number(e.target.value))}
                  min={field.minimum}
                  max={field.maximum}
                  step={0.1}
                  className="flex-1 px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                {field.minimum !== undefined && field.maximum !== undefined && (
                  <input
                    type="range"
                    value={Number(value) || field.minimum}
                    onChange={(e) => handleFieldChange(fieldId, Number(e.target.value))}
                    min={field.minimum}
                    max={field.maximum}
                    step={0.1}
                    className="w-24 accent-purple-500"
                  />
                )}
              </div>
              {field.minimum !== undefined && field.maximum !== undefined && (
                <div className="flex justify-between text-xs text-slate-400">
                  <span>{field.minimum}</span>
                  <span>{field.maximum}</span>
                </div>
              )}
            </div>
          );

      case 'boolean':
        return (
          <label className="flex items-center space-x-3 cursor-pointer">
            <input
              type="checkbox"
              checked={Boolean(value)}
              onChange={(e) => handleFieldChange(fieldId, e.target.checked)}
              className="w-4 h-4 text-purple-600 bg-slate-700 border-slate-600 rounded focus:ring-purple-500"
            />
            <span className="text-slate-300 text-sm">
              {field.description || `Enable ${field.title}`}
            </span>
          </label>
        );

      case 'array': {
        const arrayValue = Array.isArray(value) ? value : [];
        return (
          <div className="space-y-2">
            {arrayValue.map((item, index) => (
              <div key={index} className="flex items-center space-x-2">
                <input
                  type="text"
                  value={String(item)}
                  onChange={(e) => {
                    const newArray = [...arrayValue];
                    newArray[index] = e.target.value;
                    handleFieldChange(fieldId, newArray);
                  }}
                  className="flex-1 px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                <button
                  onClick={() => {
                    const newArray = arrayValue.filter((_, i) => i !== index);
                    handleFieldChange(fieldId, newArray);
                  }}
                  className="px-2 py-2 bg-red-600 hover:bg-red-700 text-white rounded-md"
                >
                  ×
                </button>
              </div>
            ))}
            <button
              onClick={() => {
                const newArray = [...arrayValue, ''];
                handleFieldChange(fieldId, newArray);
              }}
              className="px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md text-sm"
            >
              Add Item
            </button>
          </div>
        );
      }

      case 'object': {
        const objectValue = (value && typeof value === 'object' && !Array.isArray(value)) ? value as Record<string, unknown> : {};
        
        // If the field has properties defined, render them as sub-fields
        if (field.properties && Object.keys(field.properties).length > 0) {
          return (
            <div className="bg-slate-800 border border-slate-600 rounded-lg p-4 space-y-4">
              <div className="text-sm font-medium text-slate-300 border-b border-slate-600 pb-2">
                {field.title} Configuration
              </div>
              {Object.entries(field.properties).map(([subFieldId, subField]) => {
                const subFieldPath = `${fieldId}.${subFieldId}`;
                
                return (
                  <div key={subFieldId} className="space-y-2">
                    <div className="flex items-center justify-between">
                      <label className="block text-sm font-medium text-white">
                        {subField.title}
                        {subField.required && (
                          <span className="text-red-400 ml-1">*</span>
                        )}
                      </label>
                      {subField.default !== undefined && (
                        <span className="text-xs text-slate-500">
                          Default: {String(subField.default)}
                        </span>
                      )}
                    </div>
                    {subField.description && (
                      <p className="text-slate-400 text-xs">{subField.description}</p>
                    )}
                    <div className="pl-4">
                      {renderField(subField, subFieldPath)}
                    </div>
                  </div>
                );
              })}
            </div>
          );
        }
        
        // Fallback: render as JSON editor for objects without defined properties
        return (
          <div className="space-y-2">
            <textarea
              value={JSON.stringify(objectValue, null, 2)}
              onChange={(e) => {
                try {
                  const parsed = JSON.parse(e.target.value);
                  handleFieldChange(fieldId, parsed);
                } catch {
                  // Invalid JSON, don't update
                }
              }}
              placeholder="{}"
              rows={6}
              className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white font-mono text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
            />
            <div className="text-xs text-slate-400">
              Enter valid JSON configuration. Changes are applied when JSON is valid.
            </div>
          </div>
        );
      }

      default:
        // Handle enum/select cases
        if (field.enum && field.enum.length > 0) {
          return (
            <select
              value={String(value || '')}
              onChange={(e) => handleFieldChange(fieldId, e.target.value)}
              className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
            >
              <option value="">Select an option</option>
              {field.enum.map((option, index) => (
                <option key={index} value={String(option)}>
                  {String(option)}
                </option>
              ))}
            </select>
          );
        }

        return (
          <input
            type="text"
            value={String(value || '')}
            onChange={(e) => handleFieldChange(fieldId, e.target.value)}
            placeholder={`Enter ${field.title.toLowerCase()}`}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );
    }
    };
    
    return renderControl();
  };

  // Simplified field renderer for the new clean UI
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const renderSettingField = (fieldId: string, field: any) => {
    // Use current value from formData first, then fallback to field default
    const value: any = formData[fieldId] !== undefined ? formData[fieldId] : field.default;
    const isRequired = schema.required?.includes(fieldId);
    
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="block text-sm font-medium text-white">
            {field.title}
            {isRequired && <span className="text-red-400 ml-1">*</span>}
            {field.importance && field.importance >= 8 && (
              <span className="ml-2 inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-yellow-600/20 text-yellow-400">
                Essential
              </span>
            )}
          </label>
          {field.default !== undefined && (
            <span className="text-xs text-slate-500">
              Default: {String(field.default)}
            </span>
          )}
        </div>
        
        {field.description && (
          <p className="text-slate-400 text-xs leading-relaxed">{field.description}</p>
        )}
        
        <div className="mt-2">
          {renderFieldControl(field, fieldId, value)}
        </div>
      </div>
    );
  };

  // Field control renderer
  const renderFieldControl = (field: any, fieldId: string, value: any) => {
    switch (field.type) {
      case 'boolean': {
        // Use value if defined, otherwise use field default, finally fallback to false
        const booleanValue = value !== undefined ? value : (field.default !== undefined ? field.default : false);
        return (
          <div className="flex items-center">
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                className="sr-only peer"
                checked={booleanValue}
                onChange={(e) => handleFieldChange(fieldId, e.target.checked)}
              />
              <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-purple-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-purple-600"></div>
            </label>
            <span className="ml-3 text-sm text-slate-300">
              {booleanValue ? 'Enabled' : 'Disabled'}
            </span>
          </div>
        );
      }

      case 'number':
      case 'integer':
        return (
          <input
            type="number"
            value={value || field.default || ''}
            onChange={(e) => handleFieldChange(fieldId, field.type === 'integer' ? parseInt(e.target.value) || 0 : parseFloat(e.target.value) || 0)}
            min={field.minimum}
            max={field.maximum}
            step={field.type === 'integer' ? 1 : 0.1}
            className="w-full px-3 py-2 bg-slate-600 border border-slate-500 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );

      case 'string': {
        const isPassword = fieldId.toLowerCase().includes('password') || fieldId.toLowerCase().includes('key') || fieldId.toLowerCase().includes('secret') || fieldId.toLowerCase().includes('token');
        
        if (isPassword) {
          return (
            <div className="relative">
              <input
                type={showSensitive[fieldId] ? 'text' : 'password'}
                value={value || field.default || ''}
                onChange={(e) => handleFieldChange(fieldId, e.target.value)}
                placeholder={field.default ? `Default: ${field.default}` : ''}
                className="w-full px-3 py-2 pr-10 bg-slate-600 border border-slate-500 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
              <button
                type="button"
                onClick={() => setShowSensitive(prev => ({ ...prev, [fieldId]: !prev[fieldId] }))}
                className="absolute inset-y-0 right-0 pr-3 flex items-center text-slate-400 hover:text-white"
              >
                {showSensitive[fieldId] ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          );
        }
        
        if (field.enum) {
          return (
            <select
              value={value || field.default || ''}
              onChange={(e) => handleFieldChange(fieldId, e.target.value)}
              className="w-full px-3 py-2 bg-slate-600 border border-slate-500 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
            >
              <option value="">Select an option</option>
              {field.enum.map((option: string) => (
                <option key={option} value={option}>{option}</option>
              ))}
            </select>
          );
        }

        return (
          <input
            type="text"
            value={value || field.default || ''}
            onChange={(e) => handleFieldChange(fieldId, e.target.value)}
            placeholder={field.default ? `Default: ${field.default}` : ''}
            className="w-full px-3 py-2 bg-slate-600 border border-slate-500 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );
      }

      case 'array': {
        const arrayValue = Array.isArray(value) ? value : (field.default || []);
        
        return (
          <div className="space-y-2">
            {arrayValue.map((item: any, index: number) => (
              <div key={index} className="flex items-center space-x-2">
                <input
                  type="text"
                  value={item}
                  onChange={(e) => {
                    const newArray = [...arrayValue];
                    newArray[index] = e.target.value;
                    handleFieldChange(fieldId, newArray);
                  }}
                  className="flex-1 px-3 py-2 bg-slate-600 border border-slate-500 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                <button
                  onClick={() => {
                    const newArray = arrayValue.filter((_: any, i: number) => i !== index);
                    handleFieldChange(fieldId, newArray);
                  }}
                  className="px-2 py-2 bg-red-600 hover:bg-red-700 text-white rounded-md text-sm"
                >
                  ✕
                </button>
              </div>
            ))}
            
            <button
              onClick={() => {
                const newArray = [...arrayValue, ''];
                handleFieldChange(fieldId, newArray);
              }}
              className="px-3 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-md text-sm"
            >
              + Add Item
            </button>
          </div>
        );
      }

      case 'object':
        return (
          <textarea
            value={typeof value === 'object' ? JSON.stringify(value, null, 2) : value || '{}'}
            onChange={(e) => {
              try {
                const parsed = JSON.parse(e.target.value);
                handleFieldChange(fieldId, parsed);
              } catch {
                handleFieldChange(fieldId, e.target.value);
              }
            }}
            rows={4}
            className="w-full px-3 py-2 bg-slate-600 border border-slate-500 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500 font-mono text-sm"
          />
        );

      default:
        return (
          <input
            type="text"
            value={value || field.default || ''}
            onChange={(e) => handleFieldChange(fieldId, e.target.value)}
            placeholder={field.default ? `Default: ${field.default}` : ''}
            className="w-full px-3 py-2 bg-slate-600 border border-slate-500 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500"
          />
        );
    }
  };

  // Filter properties based on search term and show advanced toggle
  const filteredProperties = useMemo(() => {
    if (!schema?.properties) return {};
    
    return Object.fromEntries(
      Object.entries(schema.properties).filter(([key, property]) => {
        // Search filter
        if (searchTerm) {
          const searchLower = searchTerm.toLowerCase();
          const matchesSearch = 
            key.toLowerCase().includes(searchLower) ||
            property.title.toLowerCase().includes(searchLower) ||
            property.description?.toLowerCase().includes(searchLower);
          if (!matchesSearch) return false;
        }
        
        // Show basic settings always, advanced settings only if toggle is on
        if (property.is_basic === true) {
          return true;
        } else if (property.is_basic === false) {
          return showAdvanced;
        }
        
        // Default: show if toggle is on (for properties without is_basic defined)
        return showAdvanced;
      })
    );
  }, [schema?.properties, searchTerm, showAdvanced]);

  // Get priority settings (most important for users)
  const prioritySettings = useMemo(() => {
    return Object.entries(filteredProperties)
      .filter(([, property]) => (property.importance || 0) >= 8)
      .sort(([, a], [, b]) => (b.importance || 0) - (a.importance || 0));
  }, [filteredProperties]);

  // Get quick settings (user-friendly, high importance)
  const quickSettings = useMemo(() => {
    return Object.entries(filteredProperties)
      .filter(([, property]) => 
        property.user_friendly === true && 
        (property.importance || 0) >= 6 && 
        (property.importance || 0) < 8
      )
      .sort(([, a], [, b]) => (b.importance || 0) - (a.importance || 0));
  }, [filteredProperties]);

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
    <div className="h-full flex flex-col bg-slate-900">
      {/* Header */}
      <div className="flex-shrink-0 bg-slate-800 border-b border-slate-700 p-6">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <div className="flex items-center space-x-3">
              <h3 className="text-xl font-bold text-white">{plugin.name} Configuration</h3>
            </div>
            {schema.description && (
              <p className="text-slate-400 mt-2 max-w-2xl">{schema.description}</p>
            )}
          </div>
          <div className="flex items-center space-x-3">
            <button
              onClick={handleReset}
              disabled={saving}
              className="flex items-center px-3 py-2 bg-gray-600 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md text-sm transition-colors"
            >
              <RotateCcw size={16} className="mr-2" />
              Reset
            </button>
            <button
              onClick={handleSave}
              disabled={saving || !hasChanges}
              className="flex items-center px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md text-sm font-medium transition-colors"
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

        {/* Controls */}
        <div className="mt-6 flex items-center justify-between">
          <div className="flex items-center space-x-4">
            {/* Search */}
            <div className="relative">
              <Search size={16} className="absolute left-3 top-1/2 transform -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                placeholder="Search settings..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-9 pr-4 py-2 bg-slate-700 border border-slate-600 rounded-md text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-purple-500 text-sm w-64"
              />
            </div>

            {/* Show Advanced Toggle */}
            <div className="flex items-center bg-slate-700 rounded-lg p-1">
              <button
                onClick={() => setShowAdvanced(!showAdvanced)}
                className={`px-3 py-1.5 text-sm rounded-md transition-colors capitalize ${
                  showAdvanced
                    ? 'bg-purple-600 text-white'
                    : 'text-slate-300 hover:text-white hover:bg-slate-600'
                }`}
              >
                <Settings size={14} className="mr-1 inline" />
                {showAdvanced ? 'Hide Advanced' : 'Show Advanced'}
              </button>
            </div>
          </div>


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

      {/* Main Content */}
      <div className="flex-1 overflow-y-auto">
        <div className="p-6 space-y-6">
          {/* Priority Settings */}
          {prioritySettings.length > 0 && (
            <div className="bg-gradient-to-r from-purple-600/10 to-blue-600/10 border border-purple-600/30 rounded-lg p-6">
              <div className="flex items-center space-x-2 mb-4">
                <Zap className="text-purple-400" size={20} />
                <h4 className="text-lg font-semibold text-white">Essential Settings</h4>
                <span className="text-xs bg-purple-600/20 text-purple-400 px-2 py-1 rounded">
                  Most Important
                </span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {prioritySettings.map(([fieldId, field]) => (
                  <div key={fieldId} className="bg-slate-800/50 rounded-lg p-4">
                    {renderSettingField(fieldId, field)}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Quick Settings */}
          {quickSettings.length > 0 && (
            <div className="bg-slate-800 rounded-lg border border-slate-700">
              <div className="flex items-center space-x-2 p-4 border-b border-slate-700">
                <Settings className="text-blue-400" size={18} />
                <h4 className="text-lg font-medium text-white">Quick Settings</h4>
                <span className="text-xs bg-blue-600/20 text-blue-400 px-2 py-1 rounded">
                  Commonly Used
                </span>
              </div>
              <div className="p-4 grid grid-cols-1 md:grid-cols-2 gap-4">
                {quickSettings.map(([fieldId, field]) => (
                  <div key={fieldId}>
                    {renderSettingField(fieldId, field)}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Categorized Settings */}
          {Object.entries(
            Object.entries(filteredProperties).reduce((acc, [key, field]) => {
              const category = field.category || 'Other';
              if (!acc[category]) acc[category] = [];
              acc[category].push([key, field]);
              return acc;
            }, {} as Record<string, Array<[string, any]>>)
          )
          .sort(([a], [b]) => {
            // Sort categories: General first, then alphabetically
            if (a === 'General') return -1;
            if (b === 'General') return 1;
            return a.localeCompare(b);
          })
          .map(([categoryName, categoryFields]) => (
            <div key={categoryName} className="bg-slate-800 rounded-lg border border-slate-700">
              <button
                onClick={() => toggleCategory(categoryName)}
                className="w-full flex items-center justify-between p-4 hover:bg-slate-700/50 transition-colors"
              >
                <div className="flex items-center space-x-3">
                  <div className="text-left">
                    <h4 className="text-white font-medium">{categoryName}</h4>
                    <p className="text-slate-400 text-sm">
                      {categoryFields.length} setting{categoryFields.length !== 1 ? 's' : ''}
                    </p>
                  </div>
                </div>
                <div className="text-slate-400">
                  {expandedCategories[categoryName] ? <ChevronDown size={20} /> : <ChevronRight size={20} />}
                </div>
              </button>
              
              {expandedCategories[categoryName] && (
                <div className="border-t border-slate-700 p-4 space-y-4">
                  {categoryFields.map(([fieldId, field]) => (
                    <div key={fieldId}>
                      {renderSettingField(fieldId, field)}
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}

          {/* No Results */}
          {Object.keys(filteredProperties).length === 0 && (
            <div className="text-center py-12">
              <Search size={48} className="text-slate-600 mx-auto mb-4" />
              <div className="text-slate-400 mb-2">No settings found</div>
              <p className="text-sm text-slate-500">
                {searchTerm ? 'Try adjusting your search term' : 'No settings match the current filter'}
              </p>
            </div>
          )}
        </div>
      </div>

      {/* Configuration Info */}
      {config && (
        <div className="bg-slate-700 rounded-lg p-4 mx-4">
          <h4 className="text-white font-medium mb-3">Configuration Info</h4>
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
              <span className="text-slate-400">Total Fields:</span>
              <span className="text-white ml-2">{Object.keys(schema.properties).length}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ConfigEditor; 