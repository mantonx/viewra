// Re-export all components from organized subdirectories

// Layout Components
export { Header } from './layout';

// Media Components
export {
  MediaLibraryManager,
  MusicLibrary,
  MediaCard,
  ViewControls,
  ScanProgressCard,
  ScanActivityFeed,
  EnhancedScannerDashboard,
} from './media';

// Audio Components
export { AudioPlayer, AudioBadge, AlbumArtwork } from './audio';

// System Components
export { SystemInfo, SystemEvents, ApiTester } from './system';

// Plugin Components
export {
  PluginManager,
  PluginAdminPageCards,
  PluginAdminPages,
  PluginConfigEditor,
  PluginDependencies,
  PluginEvents,
  PluginInstaller,
  PluginPermissions,
  PluginUIComponents,
} from './plugins';

// Admin Components
export { AdminDashboard } from './admin';

// UI Components
export { default as IconButton } from './ui/IconButton';
export { default as ImageModal } from './ui/ImageModal';
export { default as Modal } from './ui/Modal';
export { default as AnimatedPlayPause } from './ui/AnimatedPlayPause';

// Icons (re-export from ui)
export * from './ui/icons';
