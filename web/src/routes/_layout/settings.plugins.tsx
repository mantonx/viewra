import { createFileRoute } from '@tanstack/react-router'
import { AdminRoute } from '@/components/common'
import { PluginsSettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/plugins')({
  component: () => (
    <AdminRoute>
      <PluginsSettings />
    </AdminRoute>
  ),
})
