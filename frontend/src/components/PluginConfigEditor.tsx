import React, { useState } from 'react';
import type { PropertySchema, ConfigValue, PluginConfigEditorProps } from '../types/plugin.types';

const PluginConfigEditor: React.FC<PluginConfigEditorProps> = ({
  schema,
  initialValues = {},
  onSave,
}) => {
  const [values, setValues] = useState<ConfigValue>(initialValues);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // If no schema is provided, return empty component
  if (!schema || !schema.properties || Object.keys(schema.properties).length === 0) {
    return (
      <div className="text-slate-400 text-sm italic">
        No configuration options available for this plugin
      </div>
    );
  }

  const handleInputChange = (key: string, value: unknown) => {
    setValues({
      ...values,
      [key]: value,
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!onSave) return;

    try {
      setSaving(true);
      setError(null);
      setSuccess(null);

      await onSave(values);
      setSuccess('Configuration saved successfully');
    } catch (err) {
      setError('Failed to save configuration');
      console.error('Error saving plugin configuration:', err);
    } finally {
      setSaving(false);
    }
  };

  // Render different input types based on property type
  const renderInput = (key: string, property: PropertySchema) => {
    const value = values[key] ?? property.default;

    switch (property.type) {
      case 'string':
        return (
          <input
            type="text"
            value={String(value || '')}
            onChange={(e) => handleInputChange(key, e.target.value)}
            className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            disabled={saving}
          />
        );

      case 'number':
        return (
          <input
            type="number"
            value={Number(value) || 0}
            onChange={(e) => handleInputChange(key, parseFloat(e.target.value) || 0)}
            className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            disabled={saving}
          />
        );

      case 'boolean':
        return (
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={Boolean(value)}
              onChange={(e) => handleInputChange(key, e.target.checked)}
              className="sr-only peer"
              disabled={saving}
            />
            <div className="w-11 h-6 bg-slate-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        );

      case 'array':
        // Simple comma-separated list for now
        return (
          <input
            type="text"
            value={Array.isArray(value) ? value.join(', ') : ''}
            onChange={(e) =>
              handleInputChange(
                key,
                e.target.value.split(',').map((v) => v.trim())
              )
            }
            className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            placeholder="Comma separated values"
            disabled={saving}
          />
        );

      default:
        return (
          <input
            type="text"
            value={typeof value === 'string' ? value : JSON.stringify(value) || ''}
            onChange={(e) => {
              try {
                handleInputChange(key, JSON.parse(e.target.value));
              } catch {
                handleInputChange(key, e.target.value);
              }
            }}
            className="w-full bg-slate-700 text-white px-3 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
            disabled={saving}
          />
        );
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
          {error}
        </div>
      )}

      {success && (
        <div className="bg-green-900/50 border border-green-700 text-green-100 px-4 py-3 rounded mb-4">
          {success}
        </div>
      )}

      <div className="space-y-4">
        {Object.entries(schema.properties).map(([key, property]) => (
          <div key={key} className="space-y-1">
            <label className="block text-sm font-medium text-white">{property.title || key}</label>
            {property.description && (
              <p className="text-slate-400 text-xs mb-1">{property.description}</p>
            )}
            {renderInput(key, property)}
          </div>
        ))}
      </div>

      <div className="mt-6">
        <button
          type="submit"
          className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-4 py-2 rounded transition-colors"
          disabled={saving}
        >
          {saving ? 'Saving...' : 'Save Configuration'}
        </button>
      </div>
    </form>
  );
};

export default PluginConfigEditor;
