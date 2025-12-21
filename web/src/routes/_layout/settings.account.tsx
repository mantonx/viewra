import { createFileRoute } from '@tanstack/react-router'
import { PageHeader } from '@/components/common'
import {
  AccountInfo,
  LocationSettings,
  ChangePassword,
  ActiveSessions,
} from '@/components/settings'

const AccountSettings = () => {
  return (
    <div className="p-8 page-enter">
      <PageHeader
        title="Account Settings"
        description="Manage your account security and preferences"
      />
      <AccountInfo className="mt-6" />
      <LocationSettings className="mt-6" />
      <ChangePassword className="mt-6" />
      <ActiveSessions className="mt-6" />
    </div>
  )
}

export const Route = createFileRoute('/_layout/settings/account')({
  component: AccountSettings,
})
