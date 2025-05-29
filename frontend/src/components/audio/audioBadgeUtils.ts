export interface AudioBadgeInfo {
  label: string;
  className: string;
  iconName: string;
  tooltip: string;
}

export const getAudioBadge = (format: string, bitrate: number): AudioBadgeInfo => {
  const upperFormat = format.toUpperCase();

  // FLAC - Lossless format
  if (upperFormat === 'FLAC') {
    return {
      label: 'FLAC',
      className:
        'bg-gradient-to-br from-emerald-500/30 to-emerald-600/50 text-emerald-100 border-emerald-300/30 backdrop-blur-sm',
      iconName: 'Crown',
      tooltip: 'Lossless FLAC format - highest quality',
    };
  }

  // WAV - Raw PCM
  if (upperFormat === 'WAV') {
    return {
      label: 'WAV',
      className:
        'bg-gradient-to-br from-slate-500/30 to-slate-600/50 text-slate-100 border-slate-300/30 backdrop-blur-sm',
      iconName: 'AudioWaveform',
      tooltip: 'Uncompressed WAV format - lossless',
    };
  }

  // OGG Vorbis - handle both OGG and VORBIS format strings
  if (upperFormat === 'OGG' || upperFormat === 'VORBIS' || upperFormat === 'OGG VORBIS') {
    return {
      label: 'OGG',
      className:
        'bg-gradient-to-br from-amber-500/30 to-amber-600/50 text-amber-100 border-amber-300/30 backdrop-blur-sm',
      iconName: 'Zap',
      tooltip: 'OGG Vorbis format - good quality lossy compression',
    };
  }

  // MP3 High Quality (320kbps) - only if bitrate is meaningful (> 0)
  if (upperFormat === 'MP3' && bitrate > 0 && bitrate >= 320000) {
    return {
      label: 'MP3 320',
      className:
        'bg-gradient-to-br from-blue-500/30 to-blue-600/50 text-blue-100 border-blue-300/30 backdrop-blur-sm',
      iconName: 'Volume2',
      tooltip: 'High quality MP3 at 320kbps',
    };
  }

  // MP3 Variable Bitrate or standard quality
  if (upperFormat === 'MP3') {
    const bitrateDisplay = bitrate > 0 ? `~${Math.round(bitrate / 1000)}kbps` : 'VBR';
    return {
      label: bitrate > 0 && bitrate >= 256000 ? 'MP3 HQ' : 'MP3 VBR',
      className:
        'bg-gradient-to-br from-sky-500/30 to-sky-600/50 text-sky-100 border-sky-300/30 backdrop-blur-sm',
      iconName: 'Signal',
      tooltip: `MP3 format (${bitrateDisplay})`,
    };
  }

  // AAC High Quality
  if (upperFormat === 'AAC' || upperFormat === 'M4A') {
    return {
      label: 'AAC 256',
      className:
        'bg-gradient-to-br from-purple-500/30 to-purple-600/50 text-purple-100 border-purple-300/30 backdrop-blur-sm',
      iconName: 'Radio',
      tooltip: 'Advanced Audio Codec at 256kbps',
    };
  }

  // Low bitrate warning - only for lossy formats with meaningful bitrate
  if (bitrate > 0 && bitrate <= 128000 && !['FLAC', 'WAV', 'OGG', 'VORBIS'].includes(upperFormat)) {
    return {
      label: 'Low',
      className:
        'bg-gradient-to-br from-red-500/30 to-red-600/50 text-red-100 border-red-300/30 backdrop-blur-sm',
      iconName: 'AlertTriangle',
      tooltip: `Low quality audio (${Math.round(bitrate / 1000)}kbps or lower)`,
    };
  }

  // Default fallback
  const bitrateInfo = bitrate > 0 ? ` at ${Math.round(bitrate / 1000)}kbps` : '';
  return {
    label: upperFormat,
    className:
      'bg-gradient-to-br from-slate-500/30 to-slate-600/50 text-slate-100 border-slate-300/30 backdrop-blur-sm',
    iconName: 'AudioWaveform',
    tooltip: `${upperFormat} format${bitrateInfo}`,
  };
};
