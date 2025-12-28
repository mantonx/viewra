import { createFileRoute } from '@tanstack/react-router'
import { AdminRoute } from '@/components/common'
import { UserManagement } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/users')({
  component: () => (
    <AdminRoute>
      <UserManagement />
    </AdminRoute>
  ),
})
