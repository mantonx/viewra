import { useState } from 'react';

const MediaUpload = () => {
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState<string | null>(null);

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFile = event.target.files?.[0];
    if (selectedFile) {
      setFile(selectedFile);
      setUploadResult(null);
    }
  };

  const handleUpload = async () => {
    if (!file) return;

    setUploading(true);
    setUploadResult(null);

    try {
      const formData = new FormData();
      formData.append('file', file);

      const response = await fetch('/api/media/', {
        method: 'POST',
        body: formData,
      });

      const result = await response.json();

      if (response.ok) {
        setUploadResult(`✅ Upload successful: ${JSON.stringify(result)}`);
        setFile(null);
        // Reset the input
        const input = document.getElementById('fileInput') as HTMLInputElement;
        if (input) input.value = '';
      } else {
        setUploadResult(`❌ Upload failed: ${result.message || result.error || 'Unknown error'}`);
      }
    } catch (error) {
      setUploadResult(
        `❌ Upload error: ${error instanceof Error ? error.message : 'Unknown error'}`
      );
    } finally {
      setUploading(false);
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <h2 className="text-xl font-semibold text-white mb-4">📁 Media Upload</h2>

      <div className="space-y-4">
        {/* File Input */}
        <div>
          <label htmlFor="fileInput" className="block text-sm font-medium text-slate-300 mb-2">
            Select Media File
          </label>
          <input
            id="fileInput"
            type="file"
            onChange={handleFileSelect}
            accept="video/*,audio/*,image/*"
            className="block w-full text-sm text-slate-300
                     file:mr-4 file:py-2 file:px-4
                     file:rounded-full file:border-0
                     file:text-sm file:font-semibold
                     file:bg-blue-600 file:text-white
                     hover:file:bg-blue-700
                     file:cursor-pointer cursor-pointer"
          />
        </div>

        {/* File Info */}
        {file && (
          <div className="bg-slate-800 rounded-lg p-4">
            <h3 className="text-white font-medium mb-2">File Information</h3>
            <div className="text-sm text-slate-300 space-y-1">
              <div>
                <span className="text-slate-400">Name:</span> {file.name}
              </div>
              <div>
                <span className="text-slate-400">Size:</span> {formatFileSize(file.size)}
              </div>
              <div>
                <span className="text-slate-400">Type:</span> {file.type || 'Unknown'}
              </div>
              <div>
                <span className="text-slate-400">Last Modified:</span>{' '}
                {new Date(file.lastModified).toLocaleString()}
              </div>
            </div>
          </div>
        )}

        {/* Upload Button */}
        <button
          onClick={handleUpload}
          disabled={!file || uploading}
          className={`w-full py-3 px-4 rounded-lg font-medium transition-colors ${
            !file || uploading
              ? 'bg-slate-600 text-slate-400 cursor-not-allowed'
              : 'bg-blue-600 hover:bg-blue-700 text-white'
          }`}
        >
          {uploading ? '🔄 Uploading...' : '📤 Upload Media'}
        </button>

        {/* Result */}
        {uploadResult && (
          <div
            className={`p-4 rounded-lg text-sm ${
              uploadResult.startsWith('✅')
                ? 'bg-green-900 text-green-300'
                : 'bg-red-900 text-red-300'
            }`}
          >
            {uploadResult}
          </div>
        )}

        {/* Info */}
        <div className="text-xs text-slate-400 bg-slate-800 rounded p-3">
          <p className="mb-1">
            <strong>📝 Note:</strong> File upload is currently in development.
          </p>
          <p>
            <strong>✅ Supported:</strong> Video, Audio, and Image files
          </p>
        </div>
      </div>
    </div>
  );
};

export default MediaUpload;
