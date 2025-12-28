import { useState, useEffect } from 'react'
import {
  loadVideoPreferences,
  saveVideoPreferences,
  DEFAULT_VIDEO_PREFERENCES,
} from '@/lib/preferences'
import type { VideoQualityPreferences } from '@/lib/preferences'
import type { QualityPreferencesProps } from './QualityPreferences.types'

export const QualityPreferences = ({
  isOpen,
  onClose,
  onPreferencesChange,
}: QualityPreferencesProps) => {
  const [preferences, setPreferences] = useState<VideoQualityPreferences>(DEFAULT_VIDEO_PREFERENCES)

  // Load preferences on mount
  useEffect(() => {
    setPreferences(loadVideoPreferences())
  }, [])

  const updatePreference = <K extends keyof VideoQualityPreferences>(
    key: K,
    value: VideoQualityPreferences[K]
  ) => {
    const updated = { ...preferences, [key]: value }

    // Handle mutual exclusivity between data saving and quality preference
    if (key === 'preferDataSaving' && value === true) {
      updated.preferQuality = false
    } else if (key === 'preferQuality' && value === true) {
      updated.preferDataSaving = false
    }

    setPreferences(updated)
    saveVideoPreferences(updated)
    onPreferencesChange?.(updated)
  }

  const resetPreferences = () => {
    setPreferences(DEFAULT_VIDEO_PREFERENCES)
    saveVideoPreferences(DEFAULT_VIDEO_PREFERENCES)
    onPreferencesChange?.(DEFAULT_VIDEO_PREFERENCES)
  }

  if (!isOpen) {
    return null
  }

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative w-full max-w-md mx-4 bg-gray-900 rounded-xl shadow-2xl border border-white/10 overflow-hidden">
        {/* Header */}
        <div className="px-6 py-4 border-b border-white/10 flex items-center justify-between">
          <h2 className="text-white text-lg font-semibold">Video Quality Preferences</h2>
          <button
            onClick={onClose}
            className="text-white/60 hover:text-white transition-colors cursor-pointer"
            aria-label="Close"
          >
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                clipRule="evenodd"
              />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="px-6 py-4 space-y-5 max-h-[60vh] overflow-y-auto">
          {/* Auto Quality */}
          <div className="space-y-3">
            <h3 className="text-white text-sm font-medium">Automatic Quality</h3>

            <label className="flex items-center gap-3 cursor-pointer group">
              <input
                type="checkbox"
                checked={preferences.autoQualityEnabled}
                onChange={(e) => updatePreference('autoQualityEnabled', e.target.checked)}
                className="w-5 h-5 rounded border-white/30 bg-white/10 text-primary-500 focus:ring-primary-500 focus:ring-offset-0 cursor-pointer"
              />
              <div className="flex-1">
                <span className="text-white text-sm group-hover:text-white/90">Enable auto quality</span>
                <p className="text-white/50 text-xs mt-0.5">
                  Automatically adjust quality based on network conditions
                </p>
              </div>
            </label>
          </div>

          {/* Quality Preference */}
          <div className="space-y-3">
            <h3 className="text-white text-sm font-medium">Quality vs Data Usage</h3>

            <label className="flex items-center gap-3 cursor-pointer group">
              <input
                type="checkbox"
                checked={preferences.preferDataSaving}
                onChange={(e) => updatePreference('preferDataSaving', e.target.checked)}
                className="w-5 h-5 rounded border-white/30 bg-white/10 text-primary-500 focus:ring-primary-500 focus:ring-offset-0 cursor-pointer"
              />
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <svg className="w-4 h-4 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M3 12v3c0 1.657 3.134 3 7 3s7-1.343 7-3v-3c0 1.657-3.134 3-7 3s-7-1.343-7-3z" />
                    <path d="M3 7v3c0 1.657 3.134 3 7 3s7-1.343 7-3V7c0 1.657-3.134 3-7 3S3 8.657 3 7z" />
                    <path d="M17 5c0 1.657-3.134 3-7 3S3 6.657 3 5s3.134-3 7-3 7 1.343 7 3z" />
                  </svg>
                  <span className="text-white text-sm group-hover:text-white/90">Prefer data saving</span>
                </div>
                <p className="text-white/50 text-xs mt-0.5 ml-6">
                  Prioritize lower quality to reduce data usage
                </p>
              </div>
            </label>

            <label className="flex items-center gap-3 cursor-pointer group">
              <input
                type="checkbox"
                checked={preferences.preferQuality}
                onChange={(e) => updatePreference('preferQuality', e.target.checked)}
                className="w-5 h-5 rounded border-white/30 bg-white/10 text-primary-500 focus:ring-primary-500 focus:ring-offset-0 cursor-pointer"
              />
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <svg className="w-4 h-4 text-yellow-400" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                  </svg>
                  <span className="text-white text-sm group-hover:text-white/90">Prefer quality</span>
                </div>
                <p className="text-white/50 text-xs mt-0.5 ml-6">
                  Prioritize higher quality even if it uses more data
                </p>
              </div>
            </label>
          </div>

          {/* Cellular/Metered Settings */}
          <div className="space-y-3">
            <h3 className="text-white text-sm font-medium">Mobile Data / Metered Connections</h3>

            <label className="flex items-center gap-3 cursor-pointer group">
              <input
                type="checkbox"
                checked={preferences.allowCellularHD}
                onChange={(e) => updatePreference('allowCellularHD', e.target.checked)}
                className="w-5 h-5 rounded border-white/30 bg-white/10 text-primary-500 focus:ring-primary-500 focus:ring-offset-0 cursor-pointer"
              />
              <div className="flex-1">
                <span className="text-white text-sm group-hover:text-white/90">Allow HD on cellular</span>
                <p className="text-white/50 text-xs mt-0.5">
                  Stream up to 1080p on mobile data
                </p>
              </div>
            </label>

            <label className="flex items-center gap-3 cursor-pointer group">
              <input
                type="checkbox"
                checked={preferences.allowCellular4K}
                onChange={(e) => updatePreference('allowCellular4K', e.target.checked)}
                className="w-5 h-5 rounded border-white/30 bg-white/10 text-primary-500 focus:ring-primary-500 focus:ring-offset-0 cursor-pointer"
              />
              <div className="flex-1">
                <span className="text-white text-sm group-hover:text-white/90">Allow 4K on cellular</span>
                <p className="text-white/50 text-xs mt-0.5">
                  Stream 4K on mobile data (high data usage)
                </p>
              </div>
            </label>
          </div>

          {/* Quality Limits */}
          <div className="space-y-3">
            <h3 className="text-white text-sm font-medium">Quality Limits</h3>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-white/70 text-xs mb-1">Maximum auto quality</label>
                <select
                  value={preferences.maxAutoQuality || ''}
                  onChange={(e) => updatePreference('maxAutoQuality', e.target.value || null)}
                  className="w-full bg-white/10 border border-white/20 rounded-md px-3 py-2 text-white text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 cursor-pointer"
                >
                  <option value="" className="bg-neutral-800 text-neutral-50">No limit</option>
                  <option value="720p" className="bg-neutral-800 text-neutral-50">720p</option>
                  <option value="1080p" className="bg-neutral-800 text-neutral-50">1080p</option>
                  <option value="1440p" className="bg-neutral-800 text-neutral-50">1440p (2K)</option>
                  <option value="4k" className="bg-neutral-800 text-neutral-50">4K</option>
                </select>
              </div>

              <div>
                <label className="block text-white/70 text-xs mb-1">Minimum auto quality</label>
                <select
                  value={preferences.minAutoQuality || ''}
                  onChange={(e) => updatePreference('minAutoQuality', e.target.value || null)}
                  className="w-full bg-white/10 border border-white/20 rounded-md px-3 py-2 text-white text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 cursor-pointer"
                >
                  <option value="" className="bg-neutral-800 text-neutral-50">No limit</option>
                  <option value="360p" className="bg-neutral-800 text-neutral-50">360p</option>
                  <option value="480p" className="bg-neutral-800 text-neutral-50">480p</option>
                  <option value="720p" className="bg-neutral-800 text-neutral-50">720p</option>
                  <option value="1080p" className="bg-neutral-800 text-neutral-50">1080p</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-4 border-t border-white/10 flex items-center justify-between">
          <button
            onClick={resetPreferences}
            className="text-white/60 hover:text-white text-sm transition-colors cursor-pointer"
          >
            Reset to defaults
          </button>
          <button
            onClick={onClose}
            className="bg-primary-500 hover:bg-primary-600 text-white px-4 py-2 rounded-md text-sm font-medium transition-colors cursor-pointer"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  )
}

export default QualityPreferences
