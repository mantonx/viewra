import { PageHeader } from '@/components/common'
import {
  AccountInfo,
  ActiveSessions,
  ChangePassword,
  LocationSettings,
} from '@/components/settings'

export const AccountSettings = () => {
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
