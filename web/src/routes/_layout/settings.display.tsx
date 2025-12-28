import { createFileRoute } from '@tanstack/react-router'
import { DisplaySettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/display')({
  component: DisplaySettings,
})
