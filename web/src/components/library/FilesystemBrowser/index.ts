// Main component
export { FilesystemBrowser } from './FilesystemBrowser'
export type { FilesystemBrowserProps } from './FilesystemBrowser.types'

// Sub-components (exported for potential reuse)
export { QuickAccessBar } from './QuickAccessBar'
export { BreadcrumbNav } from './BreadcrumbNav'
export { PathInput } from './PathInput'
export { DirectorySearch } from './DirectorySearch'
export { DirectoryTable } from './DirectoryTable'
export { ErrorDisplay } from './ErrorDisplay'

// Types (exported for testing and extensions)
export type { QuickAccessBarProps } from './QuickAccessBar.types'
export type { BreadcrumbNavProps } from './BreadcrumbNav.types'
export type { PathInputProps } from './PathInput.types'
export type { DirectorySearchProps } from './DirectorySearch.types'
export type { DirectoryTableProps, Directory } from './DirectoryTable.types'
export type { ErrorDisplayProps } from './ErrorDisplay.types'
