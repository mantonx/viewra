import { createFileRoute } from '@tanstack/react-router'
import { PreferencesSettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/preferences')({
  component: PreferencesSettings,
})
