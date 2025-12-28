import { createFileRoute } from '@tanstack/react-router'
import { AccountSettings } from '@/views/settings'

export const Route = createFileRoute('/_layout/settings/account')({
  component: AccountSettings,
})
