// Re-export all components from organized subdirectories

// Layout Components
export { default as Header } from './layout/Header';

// Media Components
export { default as MediaCard } from './media/MediaCard';
export { default as MediaLibraryManager } from './media/MediaLibraryManager';
export { default as MusicLibrary } from './media/MusicLibrary';
export { default as TVShowLibrary } from './tv/TVShowLibrary';
export { default as TVShowCard } from './tv/TVShowCard';
export { default as TVShowDetail } from './tv/TVShowDetail';

export { default as EnhancedScannerDashboard } from './media/EnhancedScannerDashboard';
export { default as ScanActivityFeed } from './media/ScanActivityFeed';
export { default as ScanProgressCard } from './media/ScanProgressCard';
export { default as EnrichmentProgressCard } from './media/EnrichmentProgressCard';

// Audio Components
export { default as AudioPlayer } from './audio/AudioPlayer';
export { default as AlbumArtwork } from './audio/AlbumArtwork';

// System Components
export { default as ApiTester } from './system/ApiTester';
export { default as SystemInfo } from './system/SystemInfo';
export { default as SystemEvents } from './system/SystemEvents';

// Plugin Components
export { ConfigEditor, PluginAdminPageRenderer } from './plugins';

// Admin Components
export { AdminDashboard } from './admin';

// UI Components
export { default as IconButton } from './ui/IconButton';
export { default as Modal } from './ui/Modal';
export { default as ImageModal } from './ui/ImageModal';
export { default as AnimatedPlayPause } from './ui/AnimatedPlayPause';

// Icons (re-export from ui)
export * from './ui/icons';
