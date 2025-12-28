import type { GithubComMantonxViewraInternalApplicationLibraryLibraryResponse } from '@/lib/api'

interface LibrarySettingsModalProps {
  library: GithubComMantonxViewraInternalApplicationLibraryLibraryResponse
  isOpen: boolean
  onClose: () => void
  onSave: () => void
}

export type { LibrarySettingsModalProps }
