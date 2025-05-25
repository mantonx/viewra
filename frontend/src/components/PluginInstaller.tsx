import React, { useState } from 'react';

interface InstallResponse {
  success: boolean;
  message: string;
  plugin_id?: string;
}

const PluginInstaller: React.FC = () => {
  const [url, setUrl] = useState('');
  const [isUploading, setIsUploading] = useState(false);
  const [installResponse, setInstallResponse] = useState<InstallResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [showForm, setShowForm] = useState(false);

  // Handle URL installation
  const handleUrlInstall = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!url.trim()) {
      setError('Please enter a valid URL');
      return;
    }

    try {
      setIsUploading(true);
      setError(null);
      setInstallResponse(null);

      const response = await fetch('/api/admin/plugins/install', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ url }),
      });

      const data = await response.json();

      if (response.ok) {
        setInstallResponse({
          success: true,
          message: data.message || 'Plugin installed successfully',
          plugin_id: data.plugin_id,
        });
        setUrl('');
      } else {
        setInstallResponse({
          success: false,
          message: data.error || 'Failed to install plugin',
        });
      }
    } catch (err) {
      setError('An error occurred while installing the plugin');
      console.error('Plugin installation error:', err);
    } finally {
      setIsUploading(false);
      setUploadProgress(0);
    }
  };

  // Handle file upload
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Check if it's a zip file
    if (file.type !== 'application/zip' && !file.name.endsWith('.zip')) {
      setError('Please upload a ZIP file containing the plugin');
      return;
    }

    try {
      setIsUploading(true);
      setError(null);
      setInstallResponse(null);

      const formData = new FormData();
      formData.append('plugin_file', file);

      const xhr = new XMLHttpRequest();

      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          const progress = Math.round((event.loaded / event.total) * 100);
          setUploadProgress(progress);
        }
      });

      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          const data = JSON.parse(xhr.responseText);
          setInstallResponse({
            success: true,
            message: data.message || 'Plugin installed successfully',
            plugin_id: data.plugin_id,
          });
          // Reset file input
          e.target.value = '';
        } else {
          try {
            const errorData = JSON.parse(xhr.responseText);
            setInstallResponse({
              success: false,
              message: errorData.error || 'Failed to install plugin',
            });
          } catch {
            setError('Failed to install plugin');
          }
        }
        setIsUploading(false);
        setUploadProgress(0);
      };

      xhr.onerror = () => {
        setError('Network error occurred while uploading');
        setIsUploading(false);
        setUploadProgress(0);
      };

      xhr.open('POST', '/api/admin/plugins/install', true);
      xhr.send(formData);
    } catch (err) {
      setError('An error occurred while uploading the plugin');
      console.error('Plugin upload error:', err);
      setIsUploading(false);
      setUploadProgress(0);
    }
  };

  return (
    <div className="bg-slate-800 rounded-lg p-6 shadow-xl">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-medium text-white flex items-center">
          <span className="mr-2">📦</span> Install Plugin
        </h3>
        <button
          onClick={() => setShowForm(!showForm)}
          className="text-slate-400 hover:text-white text-sm bg-slate-700 hover:bg-slate-600 px-3 py-1 rounded transition-colors"
        >
          {showForm ? 'Hide Form' : 'Show Form'}
        </button>
      </div>

      {showForm && (
        <div>
          {error && (
            <div className="bg-red-900/50 border border-red-700 text-red-100 px-4 py-3 rounded mb-4">
              {error}
            </div>
          )}

          {installResponse && (
            <div
              className={`${
                installResponse.success
                  ? 'bg-green-900/50 border border-green-700 text-green-100'
                  : 'bg-red-900/50 border border-red-700 text-red-100'
              } px-4 py-3 rounded mb-4`}
            >
              {installResponse.message}
            </div>
          )}

          <div className="mb-4">
            <h4 className="text-white font-medium mb-2">Option 1: Install from URL</h4>
            <form onSubmit={handleUrlInstall} className="flex gap-2">
              <input
                type="url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="Enter plugin URL (ZIP file)"
                className="flex-1 bg-slate-700 text-white px-4 py-2 rounded border border-slate-600 focus:border-blue-500 focus:outline-none"
                disabled={isUploading}
              />
              <button
                type="submit"
                disabled={isUploading || !url.trim()}
                className="bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white px-4 py-2 rounded transition-colors"
              >
                {isUploading ? 'Installing...' : 'Install'}
              </button>
            </form>
          </div>

          <div>
            <h4 className="text-white font-medium mb-2">Option 2: Upload Plugin File</h4>
            <div className="bg-slate-700 rounded border border-slate-600 p-4">
              <input
                type="file"
                accept=".zip"
                onChange={handleFileUpload}
                className="text-slate-300"
                disabled={isUploading}
              />
              <div className="text-xs text-slate-400 mt-2">
                Upload a ZIP file containing the plugin files
              </div>

              {isUploading && (
                <div className="mt-4">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs text-slate-400">Uploading...</span>
                    <span className="text-xs text-slate-400">{uploadProgress}%</span>
                  </div>
                  <div className="w-full bg-slate-800 rounded-full h-2">
                    <div
                      className="bg-blue-600 h-2 rounded-full"
                      style={{ width: `${uploadProgress}%` }}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default PluginInstaller;
