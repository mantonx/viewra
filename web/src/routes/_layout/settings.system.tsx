import { createFileRoute } from '@tanstack/react-router'
import { AdminRoute } from '@/components/common'
import { SystemSettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/system')({
  component: () => (
    <AdminRoute>
      <SystemSettings />
    </AdminRoute>
  ),
})
