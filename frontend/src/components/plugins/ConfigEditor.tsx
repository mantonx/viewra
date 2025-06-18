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
  Info,
  Search,
  Filter,
  Copy,
  Download,
  Upload,
  Trash2,
  Plus,
  GripVertical
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
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [fieldWarnings, setFieldWarnings] = useState<Record<string, string>>({});
  const [validatedFields, setValidatedFields] = useState<Record<string, boolean>>({});
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
        // Auto-expand first few categories
        const expanded: Record<string, boolean> = {};
        schemaResponse.data.categories?.slice(0, 2).forEach(cat => {
          expanded[cat.id] = !cat.collapsed;
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

  // Validation function
  const validateField = (field: ConfigurationProperty, value: unknown): { isValid: boolean; error?: string; warning?: string } => {
    if (value === null || value === undefined || value === '') {
      if (field.required) {
        return { isValid: false, error: `${field.title} is required` };
      }
      return { isValid: true };
    }

    // Type-specific validation
    switch (field.type) {
      case 'string':
      case 'text':
        const strValue = String(value);
        if (field.minLength && strValue.length < field.minLength) {
          return { isValid: false, error: `Minimum length is ${field.minLength}` };
        }
        if (field.maxLength && strValue.length > field.maxLength) {
          return { isValid: false, error: `Maximum length is ${field.maxLength}` };
        }
        if (field.pattern && !new RegExp(field.pattern).test(strValue)) {
          return { isValid: false, error: 'Invalid format' };
        }
        break;

      case 'number':
        const numValue = Number(value);
        if (isNaN(numValue)) {
          return { isValid: false, error: 'Must be a valid number' };
        }
        if (field.minimum !== undefined && numValue < field.minimum) {
          return { isValid: false, error: `Minimum value is ${field.minimum}` };
        }
        if (field.maximum !== undefined && numValue > field.maximum) {
          return { isValid: false, error: `Maximum value is ${field.maximum}` };
        }
        // Add warning for values near limits
        if (field.minimum !== undefined && field.maximum !== undefined) {
          const range = field.maximum - field.minimum;
          if (numValue < field.minimum + range * 0.1) {
            return { isValid: true, warning: 'Value is near minimum limit' };
          }
          if (numValue > field.maximum - range * 0.1) {
            return { isValid: true, warning: 'Value is near maximum limit' };
          }
        }
        break;

      case 'array':
        if (!Array.isArray(value)) {
          return { isValid: false, error: 'Must be an array' };
        }
        break;
    }

    return { isValid: true };
  };

  // Real-time validation effect
  const validateAndUpdateField = (fieldId: string, value: unknown, field: ConfigurationProperty) => {
    const validation = validateField(field, value);
    
    setFieldErrors(prev => {
      const newErrors = { ...prev };
      if (validation.error) {
        newErrors[fieldId] = validation.error;
      } else {
        delete newErrors[fieldId];
      }
      return newErrors;
    });

    setFieldWarnings(prev => {
      const newWarnings = { ...prev };
      if (validation.warning) {
        newWarnings[fieldId] = validation.warning;
      } else {
        delete newWarnings[fieldId];
      }
      return newWarnings;
    });

    setValidatedFields(prev => ({
      ...prev,
      [fieldId]: validation.isValid
    }));
  };

  // Enhanced field change handler with validation
  const handleFieldChangeWithValidation = (fieldId: string, value: unknown, field: ConfigurationProperty) => {
    handleFieldChange(fieldId, value);
    validateAndUpdateField(fieldId, value, field);
  };

  // Filter properties by search term
  const filteredProperties = useMemo(() => {
    if (!schema?.properties || !searchTerm) return schema?.properties || {};
    
    const filtered: Record<string, ConfigurationProperty> = {};
    Object.entries(schema.properties).forEach(([key, property]) => {
      if (
        key.toLowerCase().includes(searchTerm.toLowerCase()) ||
        property.title.toLowerCase().includes(searchTerm.toLowerCase()) ||
        property.description?.toLowerCase().includes(searchTerm.toLowerCase())
      ) {
        filtered[key] = property;
      }
    });
    return filtered;
  }, [schema?.properties, searchTerm]);

  // Enhanced field wrapper with validation feedback
  const renderFieldWrapper = (field: ConfigurationProperty, fieldId: string, children: React.ReactNode) => {
    const hasError = fieldErrors[fieldId];
    const hasWarning = fieldWarnings[fieldId];
    const isValid = validatedFields[fieldId] && !hasError;

    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="flex items-center text-sm font-medium text-white">
            {field.title}
            {field.required && (
              <span className="text-red-400 ml-1">*</span>
            )}
            {field.description && (
              <div className="relative ml-2">
                <Info 
                  size={14} 
                  className="text-slate-400 hover:text-white cursor-help"
                  title={field.description}
                />
              </div>
            )}
          </label>
          <div className="flex items-center space-x-2">
            {field.default !== undefined && (
              <span className="text-xs text-slate-500">
                Default: {String(field.default)}
              </span>
            )}
            {isValid && (
              <CheckCircle size={14} className="text-green-400" />
            )}
            {hasWarning && (
              <AlertTriangle size={14} className="text-yellow-400" />
            )}
            {hasError && (
              <AlertTriangle size={14} className="text-red-400" />
            )}
          </div>
        </div>
        
        {/* Input container with validation styling */}
        <div className={`relative ${
          hasError ? 'ring-2 ring-red-500/50 rounded-md' : 
          hasWarning ? 'ring-2 ring-yellow-500/50 rounded-md' : 
          isValid ? 'ring-2 ring-green-500/50 rounded-md' : ''
        }`}>
          {children}
        </div>
        
        {/* Error/Warning messages */}
        {hasError && (
          <div className="flex items-center text-xs text-red-400">
            <AlertTriangle size={12} className="mr-1" />
            {hasError}
          </div>
        )}
        {hasWarning && !hasError && (
          <div className="flex items-center text-xs text-yellow-400">
            <AlertTriangle size={12} className="mr-1" />
            {hasWarning}
          </div>
        )}
      </div>
    );
  };

  const renderField = (field: ConfigurationProperty, fieldId: string) => {
    // Handle nested field paths (e.g., "adaptive.enabled")
    let value;
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
                onChange={(e) => handleFieldChangeWithValidation(fieldId, e.target.value, field)}
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
                  onChange={(e) => handleFieldChangeWithValidation(fieldId, Number(e.target.value), field)}
                  min={field.minimum}
                  max={field.maximum}
                  step={0.1}
                  className="flex-1 px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
                {field.minimum !== undefined && field.maximum !== undefined && (
                  <input
                    type="range"
                    value={Number(value) || field.minimum}
                    onChange={(e) => handleFieldChangeWithValidation(fieldId, Number(e.target.value), field)}
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

  const getPropertiesByCategory = (categoryId: string) => {
    if (!schema?.properties) return [];
    
    return Object.entries(schema.properties).filter(([, property]) => 
      property.category === categoryId
    );
  };

  const getUncategorizedProperties = () => {
    if (!schema?.properties) return [];
    
    const categorizedProps = new Set();
    schema.categories?.forEach(category => {
      getPropertiesByCategory(category.id).forEach(([key]) => {
        categorizedProps.add(key);
      });
    });

    return Object.entries(schema.properties).filter(([key]) => 
      !categorizedProps.has(key)
    );
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
    <div className="space-y-6 max-h-[80vh] overflow-y-auto">
      {/* Header */}
      <div className="flex items-center justify-between sticky top-0 bg-slate-800 p-4 border-b border-slate-700">
        <div>
          <h3 className="text-lg font-medium text-white">{schema.title || plugin.name}</h3>
          {schema.description && (
            <p className="text-sm text-slate-400 mt-1">{schema.description}</p>
          )}
          <div className="text-xs text-slate-500 mt-1">
            {Object.keys(schema.properties).length} configuration options
          </div>
        </div>
        <div className="flex items-center space-x-2">
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
            className="flex items-center px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md text-sm transition-colors"
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
      <div className="space-y-4 px-4 pb-4">
        {schema.categories && schema.categories.length > 0 ? (
          // Render by categories
          <>
            {schema.categories
              .sort((a, b) => (a.order || 0) - (b.order || 0))
              .map((category) => {
                const categoryProperties = getPropertiesByCategory(category.id);
                if (categoryProperties.length === 0) return null;

                const isExpanded = expandedCategories[category.id];

                return (
                  <div key={category.id} className="bg-slate-700 rounded-lg overflow-hidden">
                    <button
                      onClick={() => toggleCategory(category.id)}
                      className="w-full flex items-center justify-between p-4 hover:bg-slate-600 transition-colors"
                    >
                      <div className="text-left">
                        <h4 className="text-white font-medium">{category.title}</h4>
                        {category.description && (
                          <p className="text-slate-400 text-sm mt-1">{category.description}</p>
                        )}
                        <p className="text-slate-500 text-xs mt-1">
                          {categoryProperties.length} setting{categoryProperties.length !== 1 ? 's' : ''}
                        </p>
                      </div>
                      <div className="text-slate-400">
                        {isExpanded ? <ChevronDown size={20} /> : <ChevronRight size={20} />}
                      </div>
                    </button>
                    
                    {isExpanded && (
                      <div className="p-4 border-t border-slate-600 space-y-4">
                        {categoryProperties.map(([fieldId, field]) => (
                          <div key={fieldId} className="space-y-2">
                            <div className="flex items-center justify-between">
                              <label className="block text-sm font-medium text-white">
                                {field.title}
                                {schema.required?.includes(fieldId) && (
                                  <span className="text-red-400 ml-1">*</span>
                                )}
                              </label>
                              {field.default !== undefined && (
                                <span className="text-xs text-slate-500">
                                  Default: {String(field.default)}
                                </span>
                              )}
                            </div>
                            {field.description && (
                              <p className="text-slate-400 text-xs">{field.description}</p>
                            )}
                            {renderField(field, fieldId)}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}

            {/* Uncategorized properties */}
            {getUncategorizedProperties().length > 0 && (
              <div className="bg-slate-700 rounded-lg p-4">
                <h4 className="text-white font-medium mb-4">Other Settings</h4>
                <div className="space-y-4">
                  {getUncategorizedProperties().map(([fieldId, field]) => (
                    <div key={fieldId} className="space-y-2">
                      <div className="flex items-center justify-between">
                        <label className="block text-sm font-medium text-white">
                          {field.title}
                          {schema.required?.includes(fieldId) && (
                            <span className="text-red-400 ml-1">*</span>
                          )}
                        </label>
                        {field.default !== undefined && (
                          <span className="text-xs text-slate-500">
                            Default: {String(field.default)}
                          </span>
                        )}
                      </div>
                      {field.description && (
                        <p className="text-slate-400 text-xs">{field.description}</p>
                      )}
                      {renderField(field, fieldId)}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        ) : (
          // Render properties directly if no categories
          <div className="bg-slate-700 rounded-lg p-4">
            <div className="space-y-4">
              {Object.entries(schema.properties).map(([fieldId, field]) => (
                <div key={fieldId} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <label className="block text-sm font-medium text-white">
                      {field.title}
                      {schema.required?.includes(fieldId) && (
                        <span className="text-red-400 ml-1">*</span>
                      )}
                    </label>
                    {field.default !== undefined && (
                      <span className="text-xs text-slate-500">
                        Default: {String(field.default)}
                      </span>
                    )}
                  </div>
                  {field.description && (
                    <p className="text-slate-400 text-xs">{field.description}</p>
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