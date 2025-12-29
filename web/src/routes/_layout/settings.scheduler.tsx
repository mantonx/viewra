import { createFileRoute } from '@tanstack/react-router'
import { AdminRoute } from '@/components/common'
import { SchedulerSettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/scheduler')({
  component: () => (
    <AdminRoute>
      <SchedulerSettings />
    </AdminRoute>
  ),
})
