export interface AudioBadgeInfo {
  label: string;
  className: string;
  iconName: string;
  tooltip: string;
}

export const getAudioBadge = (format: string, bitrate: number): AudioBadgeInfo => {
  const upperFormat = format.toUpperCase();

  // First check for lossless formats (highest priority)
  const losslessFormats = ['FLAC', 'WAV', 'ALAC', 'APE', 'AIFF', 'DSD', 'WV'];
  const isLossless = losslessFormats.some((losslessFormat) => upperFormat.includes(losslessFormat));

  if (isLossless) {
    return {
      label: 'LOSSLESS',
      className:
        'bg-gradient-to-r from-purple-600/30 to-pink-600/30 text-purple-100 border border-purple-400/40 backdrop-blur-md shadow-lg shadow-purple-500/20',
      iconName: 'Crown',
      tooltip: `Lossless ${upperFormat} format - maximum quality`,
    };
  }

  // Bitrate-based quality tiers for lossy formats
  if (bitrate > 0) {
    // Hi-Fi: 320 kbps and above
    if (bitrate >= 320000) {
      return {
        label: 'HI-FI',
        className:
          'bg-gradient-to-r from-emerald-600/30 to-cyan-600/30 text-emerald-100 border border-emerald-400/40 backdrop-blur-md shadow-lg shadow-emerald-500/20',
        iconName: 'Zap',
        tooltip: `Hi-Fi quality - ${Math.round(bitrate / 1000)}kbps`,
      };
    }

    // Enhanced: 192-319 kbps
    if (bitrate >= 192000) {
      return {
        label: 'ENHANCED',
        className:
          'bg-gradient-to-r from-blue-600/30 to-indigo-600/30 text-blue-100 border border-blue-400/40 backdrop-blur-md shadow-lg shadow-blue-500/20',
        iconName: 'Volume2',
        tooltip: `Enhanced quality - ${Math.round(bitrate / 1000)}kbps`,
      };
    }

    // Standard: 128 kbps and below
    if (bitrate <= 128000) {
      return {
        label: 'STANDARD',
        className:
          'bg-gradient-to-r from-slate-600/30 to-gray-600/30 text-slate-100 border border-slate-400/40 backdrop-blur-md shadow-lg shadow-slate-500/20',
        iconName: 'Signal',
        tooltip: `Standard quality - ${Math.round(bitrate / 1000)}kbps`,
      };
    }

    // Mid-range: 129-191 kbps (falls between Standard and Enhanced)
    return {
      label: 'GOOD',
      className:
        'bg-gradient-to-r from-amber-600/30 to-orange-600/30 text-amber-100 border border-amber-400/40 backdrop-blur-md shadow-lg shadow-amber-500/20',
      iconName: 'TrendingUp',
      tooltip: `Good quality - ${Math.round(bitrate / 1000)}kbps`,
    };
  }

  // Fallback for variable bitrate or unknown bitrate
  if (upperFormat === 'MP3') {
    return {
      label: 'VBR',
      className:
        'bg-gradient-to-r from-sky-600/30 to-blue-600/30 text-sky-100 border border-sky-400/40 backdrop-blur-md shadow-lg shadow-sky-500/20',
      iconName: 'Activity',
      tooltip: 'Variable bitrate MP3',
    };
  }

  // Generic fallback for other formats
  return {
    label: upperFormat || 'AUDIO',
    className:
      'bg-gradient-to-r from-gray-600/30 to-slate-600/30 text-gray-100 border border-gray-400/40 backdrop-blur-md shadow-lg shadow-gray-500/20',
    iconName: 'FileAudio',
    tooltip: `${upperFormat || 'Audio'} format`,
  };
};
