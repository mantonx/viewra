import { createFileRoute } from '@tanstack/react-router'
import { AdminRoute } from '@/components/common'
import { AISettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/ai')({
  component: () => (
    <AdminRoute>
      <AISettings />
    </AdminRoute>
  ),
})
